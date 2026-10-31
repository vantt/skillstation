package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// RawEventFilter narrows a raw event query. An empty field does not filter.
type RawEventFilter struct {
	// Kinds keeps only events of these types.
	Kinds []string
	// ResolutionIDs keeps only events that carry one of these resolution ids.
	ResolutionIDs []string
}

// rawEventsStore reads valid stored envelopes from the telemetry database
// within the time range [from, to]. Dates in YYYY-MM-DD format are expanded
// to full UTC day boundaries so they behave inclusively; an RFC 3339 time is
// an exact bound. An empty bound is open-ended.
func rawEventsStore(ctx context.Context, config Config, from, to string, filter RawEventFilter) ([]Event, error) {
	if err := maintainStoreIfDue(ctx, config); err != nil {
		return nil, err
	}
	database, err := openDatabase(ctx, config)
	if err != nil {
		return nil, err
	}
	defer database.Close()

	conditions, arguments := []string{}, []any{}
	if bound := rawEventBound(from, "T00:00:00.000000000Z"); bound != "" {
		conditions, arguments = append(conditions, "occurred_at >= ?"), append(arguments, bound)
	}
	if bound := rawEventBound(to, "T23:59:59.999999999Z"); bound != "" {
		conditions, arguments = append(conditions, "occurred_at <= ?"), append(arguments, bound)
	}
	for _, set := range []struct {
		column string
		values []string
	}{{"kind", filter.Kinds}, {"resolution_id", filter.ResolutionIDs}} {
		if len(set.values) == 0 {
			continue
		}
		conditions = append(conditions, set.column+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(set.values)), ",")+")")
		for _, value := range set.values {
			arguments = append(arguments, value)
		}
	}
	query := "SELECT id,occurred_at,kind,payload_json FROM telemetry_events"
	if len(conditions) != 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := database.QueryContext(ctx, query+" ORDER BY occurred_at,id", arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Event{}
	for rows.Next() {
		var id, occurredAt, kind, raw string
		if err := rows.Scan(&id, &occurredAt, &kind, &raw); err != nil {
			return nil, err
		}
		var envelope storedEnvelope
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil || validateStored(envelope, id, occurredAt, kind) != nil {
			continue
		}
		parsedTime, err := time.Parse(time.RFC3339Nano, envelope.OccurredAt)
		if err != nil {
			parsedTime, _ = time.Parse(time.RFC3339, envelope.OccurredAt)
		}
		result = append(result, Event{
			Version:         envelope.Version,
			ID:              envelope.ID,
			Type:            envelope.Type,
			OccurredAt:      parsedTime,
			SessionIDHash:   envelope.SessionIDHash,
			RequestID:       envelope.RequestID,
			ResolutionID:    envelope.ResolutionID,
			CatalogSnapshot: envelope.CatalogSnapshot,
			PolicyRevision:  envelope.PolicyRevision,
			Client:          envelope.Client,
			Payload:         envelope.Payload,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// rawEventBound turns a query bound into the fixed-width stored time format,
// so that it compares correctly with stored timestamps. A day gets daySuffix.
func rawEventBound(value, daySuffix string) string {
	if len(value) == len("2006-01-02") {
		return value + daySuffix
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return formatStoredTime(parsed)
	}
	return value
}

func oldestRawEventStore(ctx context.Context, config Config) (time.Time, bool, error) {
	if err := maintainStoreIfDue(ctx, config); err != nil {
		return time.Time{}, false, err
	}
	database, err := openDatabase(ctx, config)
	if err != nil {
		return time.Time{}, false, err
	}
	defer database.Close()

	var occurredAt sql.NullString
	err = database.QueryRowContext(ctx, `SELECT MIN(occurred_at) FROM telemetry_events WHERE kind != ?`, EventHubCall).Scan(&occurredAt)
	if err != nil || !occurredAt.Valid || occurredAt.String == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse(time.RFC3339Nano, occurredAt.String)
	if err != nil {
		t, err = time.Parse(time.RFC3339, occurredAt.String)
		if err != nil {
			return time.Time{}, false, err
		}
	}
	return t, true, nil
}

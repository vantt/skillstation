package app

import (
	"context"
	"regexp"
	"sort"
	"time"

	"github.com/vantt/skillstation/internal/telemetry"
	"github.com/vantt/skillstation/internal/workspace"
)

const (
	// HubCallRetentionDays mirrors telemetry.HubCallRetention for reports.
	HubCallRetentionDays = int(telemetry.HubCallRetention / (24 * time.Hour))
	hubCallRecentLimit   = 200
	hubCallUnknownTool   = "unknown"
)

var hubCallToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@+\-]{0,255}$`)

// HubCall is one request served by the MCP hub. It holds only bounded tokens;
// arguments and results never reach the log.
type HubCall struct {
	Method    string
	Tool      string
	Failed    bool
	ErrorCode string
	Duration  time.Duration
	Client    telemetry.Client
}

// RecordHubCall asynchronously records one hub.call event.
func RecordHubCall(ctx context.Context, sink TelemetrySink, workspacePath string, call HubCall) {
	if sink == nil {
		return
	}
	status := telemetry.HubCallStatusOK
	if call.Failed {
		status = telemetry.HubCallStatusError
	}
	payload := map[string]any{
		"method":      call.Method,
		"status":      status,
		"duration_ms": call.Duration.Milliseconds(),
	}
	if call.Tool != "" {
		payload["tool"] = hubCallTokenOr(call.Tool, hubCallUnknownTool)
	}
	if call.Failed && call.ErrorCode != "" {
		payload["error_code"] = hubCallTokenOr(call.ErrorCode, hubCallUnknownTool)
	}
	event := curationTelemetryEvent(telemetry.EventHubCall, payload)
	if call.Client.Name != "" {
		event.Client = call.Client
	}
	recordCurationTelemetry(ctx, sink, workspacePath, event)
}

func hubCallTokenOr(value, fallback string) string {
	if hubCallToken.MatchString(value) {
		return value
	}
	return fallback
}

// HubCallEntry is one logged hub request.
type HubCallEntry struct {
	OccurredAt time.Time `json:"occurred_at"`
	Method     string    `json:"method"`
	Tool       string    `json:"tool,omitempty"`
	Status     string    `json:"status"`
	ErrorCode  string    `json:"error_code,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	Client     string    `json:"client"`
}

// HubCallCount is a call and failure count for one tool or one day.
type HubCallCount struct {
	Method string `json:"method,omitempty"`
	Tool   string `json:"tool,omitempty"`
	Day    string `json:"day,omitempty"`
	Client string `json:"client,omitempty"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

// HubCallLog summarizes the retained hub calls.
type HubCallLog struct {
	RetentionDays int            `json:"retention_days"`
	Since         time.Time      `json:"since"`
	Total         int64          `json:"total"`
	Succeeded     int64          `json:"succeeded"`
	Failed        int64          `json:"failed"`
	ByTool        []HubCallCount `json:"by_tool"`
	ByDay         []HubCallCount `json:"by_day"`
	ByClient      []HubCallCount `json:"by_client"`
	Recent        []HubCallEntry `json:"recent"`
}

// HubCallService reads the hub call log.
type HubCallService struct {
	Telemetry TelemetryService
	Now       func() time.Time
}

// Log returns the hub calls of the last HubCallRetentionDays days, newest first.
func (service HubCallService) Log(ctx context.Context, path string) (log HubCallLog, resultErr error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return HubCallLog{}, err
	}
	now := time.Now()
	if service.Now != nil {
		now = service.Now()
	}
	now = now.UTC()
	since := now.Add(-telemetry.HubCallRetention)

	recorder, err := service.Telemetry.Open(root)
	if err != nil {
		return HubCallLog{}, err
	}
	defer closeTelemetryRecorder(recorder, &resultErr)
	events, err := recorder.RawEvents(ctx, since.Format("2006-01-02"), now.Format("2006-01-02"))
	if err != nil {
		return HubCallLog{}, storeFailure(err)
	}

	log = HubCallLog{RetentionDays: HubCallRetentionDays, Since: since, ByTool: []HubCallCount{}, ByDay: []HubCallCount{}, ByClient: []HubCallCount{}, Recent: []HubCallEntry{}}
	tools := map[[2]string]*HubCallCount{}
	days := map[string]*HubCallCount{}
	clients := map[string]*HubCallCount{}
	for _, event := range events {
		if event.Type != telemetry.EventHubCall || event.OccurredAt.Before(since) {
			continue
		}
		entry := hubCallEntry(event)
		failed := entry.Status == telemetry.HubCallStatusError
		log.Total++
		if failed {
			log.Failed++
		} else {
			log.Succeeded++
		}
		toolKey := [2]string{entry.Method, entry.Tool}
		if tools[toolKey] == nil {
			tools[toolKey] = &HubCallCount{Method: entry.Method, Tool: entry.Tool}
		}
		day := entry.OccurredAt.Format("2006-01-02")
		if days[day] == nil {
			days[day] = &HubCallCount{Day: day}
		}
		if clients[entry.Client] == nil {
			clients[entry.Client] = &HubCallCount{Client: entry.Client}
		}
		for _, count := range []*HubCallCount{tools[toolKey], days[day], clients[entry.Client]} {
			count.Calls++
			if failed {
				count.Errors++
			}
		}
		log.Recent = append(log.Recent, entry)
	}
	for _, count := range tools {
		log.ByTool = append(log.ByTool, *count)
	}
	sort.Slice(log.ByTool, func(i, j int) bool {
		a, b := log.ByTool[i], log.ByTool[j]
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Method+a.Tool < b.Method+b.Tool
	})
	for _, count := range days {
		log.ByDay = append(log.ByDay, *count)
	}
	for _, count := range clients {
		log.ByClient = append(log.ByClient, *count)
	}
	sort.Slice(log.ByClient, func(i, j int) bool {
		a, b := log.ByClient[i], log.ByClient[j]
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Client < b.Client
	})
	sort.Slice(log.ByDay, func(i, j int) bool { return log.ByDay[i].Day > log.ByDay[j].Day })
	// RawEvents is oldest first; the screen shows the newest first.
	sort.SliceStable(log.Recent, func(i, j int) bool { return log.Recent[i].OccurredAt.After(log.Recent[j].OccurredAt) })
	if len(log.Recent) > hubCallRecentLimit {
		log.Recent = log.Recent[:hubCallRecentLimit]
	}
	return log, nil
}

func hubCallEntry(event telemetry.Event) HubCallEntry {
	text := func(field string) string { value, _ := event.Payload[field].(string); return value }
	duration, _ := event.Payload["duration_ms"].(float64)
	return HubCallEntry{
		OccurredAt: event.OccurredAt.UTC(),
		Method:     text("method"),
		Tool:       text("tool"),
		Status:     text("status"),
		ErrorCode:  text("error_code"),
		DurationMS: int64(duration),
		Client:     event.Client.Name,
	}
}

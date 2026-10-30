package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/telemetry"
)

func hubCallEvent(client string, at time.Time, payload map[string]any) telemetry.Event {
	return telemetry.Event{
		Type: telemetry.EventHubCall, OccurredAt: at, CatalogSnapshot: "sha256:test", PolicyRevision: "sha256:test",
		Client: telemetry.Client{Name: client}, Payload: payload,
	}
}

// The log counts successes and failures per tool and client, and a call older
// than the retention window is gone.
func TestHubCallsEndpointCountsRecentCallsOnly(t *testing.T) {
	t.Parallel()
	root := newWebWorkspace(t)
	recorder, err := (app.TelemetryService{}).Open(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, event := range []telemetry.Event{
		hubCallEvent("claude-code", now.Add(-time.Hour), map[string]any{"method": "tools_call", "tool": "skill_resolve", "status": "ok", "duration_ms": 12}),
		hubCallEvent("claude-code", now.Add(-2*time.Hour), map[string]any{"method": "tools_call", "tool": "skill_get", "status": "error", "error_code": "not_found", "duration_ms": 3}),
		hubCallEvent("codex", now.Add(-3*time.Hour), map[string]any{"method": "connect", "status": "ok", "duration_ms": 1}),
		hubCallEvent("codex", now.Add(-4*24*time.Hour), map[string]any{"method": "tools_call", "tool": "skill_resolve", "status": "ok", "duration_ms": 9}),
	} {
		recorder.Record(event)
	}
	if err := recorder.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	srv, err := New(Options{Workspace: root, Token: "test-token", ListenAddrs: listenOn("127.0.0.1", 7421), Assets: fstest.MapFS{}, Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	rec := get(t, srv, "/api/v1/hub-calls")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET hub-calls = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var log app.HubCallLog
	if err := json.Unmarshal(rec.Body.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	if log.RetentionDays != 3 || log.Total != 3 || log.Succeeded != 2 || log.Failed != 1 {
		t.Fatalf("totals = %+v, want 3 calls (2 ok, 1 failed) over 3 days", log)
	}
	if len(log.Recent) != 3 || log.Recent[0].Method != "tools_call" || log.Recent[0].Tool != "skill_resolve" || log.Recent[2].Method != "connect" {
		t.Fatalf("recent = %+v, want newest first", log.Recent)
	}
	if log.Recent[1].ErrorCode != "not_found" {
		t.Fatalf("error code = %q, want not_found", log.Recent[1].ErrorCode)
	}
	if len(log.ByClient) != 2 || log.ByClient[0].Client != "claude-code" || log.ByClient[0].Calls != 2 || log.ByClient[0].Errors != 1 {
		t.Fatalf("by client = %+v", log.ByClient)
	}
}

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/telemetry"
)

func linkedEvent(kind, resolutionID string, at time.Time, payload map[string]any) telemetry.Event {
	event := hubCallEvent("claude-code", at, payload)
	event.Type, event.ResolutionID = kind, resolutionID
	return event
}

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
		linkedEvent(telemetry.EventHubCall, "res_1", now.Add(-time.Hour), map[string]any{"method": "tools_call", "tool": "skill_resolve", "status": "ok", "duration_ms": 12, "operation": "review", "task_text": "review changed code"}),
		hubCallEvent("claude-code", now.Add(-2*time.Hour), map[string]any{"method": "tools_call", "tool": "skill_get", "status": "error", "error_code": "not_found", "duration_ms": 3}),
		hubCallEvent("codex", now.Add(-3*time.Hour), map[string]any{"method": "connect", "status": "ok", "duration_ms": 1}),
		linkedEvent(telemetry.EventResolutionCompleted, "res_1", now.Add(-59*time.Minute), map[string]any{"status": "resolved", "top_skill_id": "code-review", "recommended_skill_ids": []string{"code-review"}}),
		linkedEvent(telemetry.EventSkillLoaded, "res_1", now.Add(-58*time.Minute), map[string]any{"skill_id": "code-review", "resource_kind": "entrypoint", "surface": "skill_get", "basis": "server-observed", "first_activation": false}),
		linkedEvent(telemetry.EventSkillUsed, "res_1", now.Add(-57*time.Minute), map[string]any{"skill_id": "code-review"}),
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
	resolved := log.Recent[0].Resolution
	if log.Recent[0].TaskText != "review changed code" || resolved == nil || resolved.Status != "resolved" ||
		len(resolved.Recommended) != 1 || len(resolved.Loaded) != 1 || resolved.Loaded[0] != "code-review" || !resolved.Used {
		t.Fatalf("resolve call = %+v (resolution %+v), want its task text, recommended and loaded skill and use", log.Recent[0], resolved)
	}
	if log.Recent[1].ErrorCode != "not_found" {
		t.Fatalf("error code = %q, want not_found", log.Recent[1].ErrorCode)
	}
	if len(log.ByClient) != 2 || log.ByClient[0].Client != "claude-code" || log.ByClient[0].Calls != 2 || log.ByClient[0].Errors != 1 {
		t.Fatalf("by client = %+v", log.ByClient)
	}
}

// The task capture switch is saved on the machine and read back by the log.
func TestHubCallSettingsToggleTaskCapture(t *testing.T) {
	t.Parallel()
	root := newWebWorkspace(t)
	srv := newTestServer(t, root)
	if log := readHubCallLog(t, srv); log.CaptureTaskText {
		t.Fatal("task capture must start off")
	}
	body, _ := json.Marshal(map[string]bool{"capture_task_text": true})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/hub-calls/settings", bytes.NewReader(body))
	req.Host = "127.0.0.1:7421"
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Origin", "http://127.0.0.1:7421")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !readHubCallLog(t, srv).CaptureTaskText || !app.HubCallSettingsOf(root).CaptureTaskText {
		t.Fatal("task capture was not saved")
	}
}

func readHubCallLog(t *testing.T, srv *Server) app.HubCallLog {
	t.Helper()
	var log app.HubCallLog
	if err := json.Unmarshal(get(t, srv, "/api/v1/hub-calls").Body.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	return log
}

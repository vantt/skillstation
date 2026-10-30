package mcpserver

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/telemetry"
)

// Every request a client sends is logged with its client, tool and outcome,
// and the log carries no request content.
func TestHubCallsAreLogged(t *testing.T) {
	t.Parallel()
	root := newMCPWorkspace(t)
	_, _, recorder, session := connectTelemetrySession(t, root)

	callSkillResolve(t, session, map[string]any{
		"schema_version": "1",
		"request_id":     "REQ-hub-call-log",
		"task":           map[string]any{"description": "secret-task-text", "scope": "multi_step"},
		"operation":      "review",
	})
	failed, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "skill_get", Arguments: map[string]any{"skill_id": "no-such-skill"}})
	if err != nil || !failed.IsError {
		t.Fatalf("skill_get of a missing skill = %#v, %v; want a tool error", failed, err)
	}
	// Calls are logged after the answer is sent, so wait for the log to catch up.
	day := time.Now().UTC().Format("2006-01-02")
	seen := map[string]string{}
	for deadline := time.Now().Add(5 * time.Second); len(seen) < 3 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if err := recorder.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
		events, err := recorder.RawEvents(t.Context(), day, day)
		if err != nil {
			t.Fatal(err)
		}
		seen = map[string]string{}
		for _, event := range events {
			if event.Type != telemetry.EventHubCall {
				continue
			}
			if event.Client.Name == "" {
				t.Fatalf("hub call has no client: %+v", event)
			}
			for _, value := range event.Payload {
				if text, _ := value.(string); text == "secret-task-text" {
					t.Fatalf("hub call leaked request content: %+v", event.Payload)
				}
			}
			method, _ := event.Payload["method"].(string)
			tool, _ := event.Payload["tool"].(string)
			status, _ := event.Payload["status"].(string)
			seen[method+"/"+tool] = status
		}
	}
	for key, want := range map[string]string{
		"connect/":                 telemetry.HubCallStatusOK,
		"tools_call/skill_resolve": telemetry.HubCallStatusOK,
		"tools_call/skill_get":     telemetry.HubCallStatusError,
	} {
		if seen[key] != want {
			t.Fatalf("hub calls = %v; want %s with status %s", seen, key, want)
		}
	}
}

// With task capture on, a skill_resolve call is logged with its task text
// (secrets removed) and the resolution id that links it to its outcome.
func TestHubCallLogsTaskTextOnlyWhenCaptureIsOn(t *testing.T) {
	t.Parallel()
	root := newMCPWorkspace(t)
	if err := app.SaveHubCallSettings(root, app.HubCallSettings{CaptureTaskText: true}); err != nil {
		t.Fatal(err)
	}
	_, _, recorder, session := connectTelemetrySession(t, root)

	resolved := callSkillResolve(t, session, map[string]any{
		"schema_version": "1",
		"request_id":     "REQ-hub-call-capture",
		"task":           map[string]any{"description": "review changed code with token=abc123secret\nplease", "scope": "multi_step"},
		"operation":      "review",
	})

	day := time.Now().UTC().Format("2006-01-02")
	var logged *telemetry.Event
	for deadline := time.Now().Add(5 * time.Second); logged == nil && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if err := recorder.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
		events, err := recorder.RawEvents(t.Context(), day, day)
		if err != nil {
			t.Fatal(err)
		}
		for i, event := range events {
			if event.Type == telemetry.EventHubCall && event.Payload["tool"] == "skill_resolve" {
				logged = &events[i]
			}
		}
	}
	if logged == nil {
		t.Fatal("skill_resolve call was not logged")
	}
	if logged.ResolutionID != resolved.Resolution.ResolutionID || logged.Payload["operation"] != "review" {
		t.Fatalf("hub call = %+v, resolution %q", logged, resolved.Resolution.ResolutionID)
	}
	text, _ := logged.Payload["task_text"].(string)
	if text != "review changed code with token=[REDACTED] please" {
		t.Fatalf("task_text = %q, want one redacted line", text)
	}
}

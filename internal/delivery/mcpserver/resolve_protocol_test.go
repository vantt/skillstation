package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	resolverpkg "github.com/vantt/skillstation/internal/resolver"
)

// resolveToolError calls skill_resolve with arguments the tool must refuse
// and returns the error of its envelope.
func resolveToolError(t *testing.T, session *mcp.ClientSession, arguments map[string]any) toolError {
	t.Helper()
	response, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "skill_resolve", Arguments: arguments})
	if err != nil || !response.IsError {
		t.Fatalf("skill_resolve = %#v, %v; want a tool error", response, err)
	}
	var outcome toolOutcome[resolveResult]
	decodeStructuredContent(t, response, &outcome)
	if outcome.Error == nil {
		t.Fatalf("skill_resolve refused %v without an error envelope: %#v", arguments, response)
	}
	return *outcome.Error
}

// TestResolveSessionRemembersRejectedSkills pins the rejection path of one
// MCP session: a corrected request that rejects an answer does not get the
// rejected skill again, and the third rejection of one task is refused.
func TestResolveSessionRemembersRejectedSkills(t *testing.T) {
	t.Parallel()
	session := resolveSession(t)
	task := func(constraint string, prior *resolverpkg.Response) map[string]any {
		arguments := resolveTask("review changed code in module alpha", "review", "")
		if constraint != "" {
			arguments["task"].(map[string]any)["constraints"] = []any{constraint}
		}
		if prior != nil {
			arguments["prior"] = map[string]any{"resolution_id": prior.ResolutionID, "kind": "rejected", "context_revision": prior.ContextRevision}
		}
		return arguments
	}

	first := callSkillResolve(t, session, task("", nil)).Resolution
	if first.Primary == nil || first.Primary.ID != "review-skill" {
		t.Fatalf("first answer = %#v, want review-skill", first)
	}
	answer := first
	for round, constraint := range []string{"keep the public API", "keep the public API stable"} {
		answer = callSkillResolve(t, session, task(constraint, &answer)).Resolution
		if answer.Primary != nil || answer.ContextRevision != round+2 {
			t.Fatalf("re-resolution %d = %s %#v revision %d, want no rejected skill", round+1, answer.Status, answer.Primary, answer.ContextRevision)
		}
	}
	refused := resolveToolError(t, session, task("", &answer))
	if refused.Code != "resolution_retry_exhausted" || refused.Retryable || refused.SuggestedAction == "" {
		t.Fatalf("third rejection = %+v, want resolution_retry_exhausted, not retryable", refused)
	}
}

// TestResolveRefusalsUseTheErrorEnvelope pins that a request for another
// schema version and a clarification without its question or answer reach the
// agent as coded errors, not as a schema-validation text or internal_error.
func TestResolveRefusalsUseTheErrorEnvelope(t *testing.T) {
	t.Parallel()
	session := resolveSession(t)

	future := resolveTask("review changed code", "review", "")
	future["schema_version"] = "2"
	future["intent"] = "a field version 2 defines"
	if refused := resolveToolError(t, session, future); refused.Code != "unsupported_schema" || refused.Retryable {
		t.Fatalf("schema_version 2 = %+v, want unsupported_schema", refused)
	}

	incomplete := resolveTask("review changed code", "review", "")
	incomplete["prior"] = map[string]any{"resolution_id": "res_any", "kind": "clarification", "context_revision": 1}
	refused := resolveToolError(t, session, incomplete)
	if refused.Code != "invalid_request" || refused.Retryable || !strings.Contains(refused.Message, "question_id and answer") || !strings.Contains(refused.SuggestedAction, "prior.answer") {
		t.Fatalf("clarification without question and answer = %+v, want invalid_request naming both fields", refused)
	}
}

// TestResolvePriorOnAStaleIndexNamesARuntimeAction pins that an agent with the
// runtime profile, which has no rebuild tool, is told what it can do when a
// prior meets an index that is behind the files.
func TestResolvePriorOnAStaleIndexNamesARuntimeAction(t *testing.T) {
	t.Parallel()
	root := newMCPWorkspace(t)
	_, server, err := New(root, nil, ProfileRuntime)
	if err != nil {
		t.Fatal(err)
	}
	session := newClientSession(t, server)
	first := callSkillResolve(t, session, resolveTask("review changed code in module alpha", "review", "")).Resolution
	skillFile := filepath.Join(root, "skills", "core", "review-skill", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillFile, append(content, []byte("\nEdited.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	refused := resolveToolError(t, session, resolveTask("review changed code in module alpha", "review", first.ResolutionID))
	if refused.Code != "index_stale" || !refused.Retryable || strings.Contains(refused.SuggestedAction, "workspace_rebuild") ||
		!strings.Contains(refused.SuggestedAction, "without prior") || !strings.Contains(refused.SuggestedAction, "skillstation doctor --fix") {
		t.Fatalf("prior on a stale index = %+v, want index_stale with an action the runtime profile can take", refused)
	}
}

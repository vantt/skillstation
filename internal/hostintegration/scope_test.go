package hostintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/systemskills"
)

func TestProjectRootIsSeparateFromServedWorkspace(t *testing.T) {
	workspace := t.TempDir()
	project := t.TempDir()
	binary := filepath.Join(workspace, "bin", "skillstation")
	request := Request{Workspace: workspace, Root: project, Binary: binary}

	plan, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, change := range plan.Changes {
		if !strings.HasPrefix(change.Path, project+string(filepath.Separator)) {
			t.Fatalf("change path %s is outside the project root", change.Path)
		}
	}
	if _, err := Apply(context.Background(), plan, ApplyOptions{Confirmed: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertJSONRegistration(t, filepath.Join(project, ".mcp.json"), binary, workspace, false)
	assertNotContains(t, filepath.Join(project, ".codex/config.toml"), "cwd = ")
	if _, err := os.Stat(filepath.Join(project, "CLAUDE.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("connect wrote CLAUDE.md, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".mcp.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace received host files, stat error = %v", err)
	}

	again, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("second Plan: %v", err)
	}
	if len(again.Changes) != 0 {
		t.Fatalf("second plan has %d changes, want idempotent no-op", len(again.Changes))
	}
}

func TestUserScopeWritesHomeFilesAndPreservesUnrelatedClaudeKeys(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	binary := filepath.Join(workspace, "bin", "skillstation")
	claudeJSON := "{\n  \"numStartups\": 42,\n  \"projects\": {\"/some/project\": {\"mcpServers\": {\"local\": {\"command\": \"x\"}}}},\n  \"mcpServers\": {\"other\": {\"command\": \"other\"}}\n}\n"
	writeTestFile(t, filepath.Join(home, ".claude.json"), []byte(claudeJSON), 0o600)
	writeTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), []byte("# My global rules\n"), 0o640)
	request := Request{Workspace: workspace, Root: home, Scope: ScopeUser, Binary: binary}

	plan, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := Apply(context.Background(), plan, ApplyOptions{Confirmed: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertJSONRegistration(t, filepath.Join(home, ".claude.json"), binary, workspace, false)
	assertContains(t, filepath.Join(home, ".claude.json"), `"numStartups": 42`)
	assertContains(t, filepath.Join(home, ".claude.json"), `"local": {"command": "x"}`)
	assertContains(t, filepath.Join(home, ".claude.json"), `"other": {"command": "other"}`)
	assertMode(t, filepath.Join(home, ".claude.json"), 0o600)
	assertContains(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# My global rules")
	if got := string(readTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))); got != "# My global rules\n" {
		t.Fatalf("user CLAUDE.md = %q, want it untouched", got)
	}

	for _, path := range []string{
		".claude/skills/skill-curator/SKILL.md",
		".agents/skills/skill-curator/SKILL.md",
		".codex/config.toml", ".gemini/settings.json",
	} {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected user-scope file %s: %v", path, err)
		}
	}
	assertJSONRegistration(t, filepath.Join(home, ".gemini/settings.json"), binary, workspace, true)
	assertSystemSkillLayout(t, home, systemskills.CuratorSkillID)
	// Project-scope names must not leak into the home directory.
	for _, path := range []string{".mcp.json", "CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(home, path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("project-scope file %s was written into home, stat error = %v", path, err)
		}
	}

	again, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("second Plan: %v", err)
	}
	if len(again.Changes) != 0 {
		t.Fatalf("second plan has %d changes, want idempotent no-op", len(again.Changes))
	}
}

func TestUserScopeRefusesWhenClaudeConfigChangesAfterPlanning(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude.json"), []byte("{\"mcpServers\":{}}\n"), 0o600)
	request := Request{Workspace: workspace, Root: home, Scope: ScopeUser, Binary: filepath.Join(workspace, "skillstation"), Hosts: []Host{HostClaude}}
	plan, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// Claude Code rewrites this file while it runs; a concurrent edit must win.
	writeTestFile(t, filepath.Join(home, ".claude.json"), []byte("{\"mcpServers\":{},\"changed\":true}\n"), 0o600)
	if _, err := Apply(context.Background(), plan, ApplyOptions{Confirmed: true}); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("Apply error = %v, want ErrStalePlan", err)
	}
	assertContains(t, filepath.Join(home, ".claude.json"), `"changed":true`)
}

// relayoutJSON rewrites a JSON file the way a host does when it saves its own
// settings: other indentation, one array element per line, keys reordered.
func relayoutJSON(t *testing.T, path string, edit func(map[string]any)) {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(readTestFile(t, path), &document); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if edit != nil {
		edit(document)
	}
	relaid, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, relaid, 0o600)
}

func TestJSONHostFilesRelaidOutByTheHostStayCurrent(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude.json"), []byte(`{"numStartups": 42, "mcpServers": {"other": {"command": "other"}}}`), 0o600)
	request := Request{Workspace: workspace, Root: home, Scope: ScopeUser, Binary: filepath.Join(workspace, "bin", "skillstation")}
	applyAll(t, request)

	jsonFiles := []string{".claude.json", ".claude/settings.json", ".gemini/settings.json"}
	before := map[string][]byte{}
	for _, rel := range jsonFiles {
		path := filepath.Join(home, filepath.FromSlash(rel))
		relayoutJSON(t, path, nil)
		before[path] = readTestFile(t, path)
	}
	if !strings.Contains(string(before[filepath.Join(home, ".claude.json")]), "\"args\": [\n") {
		t.Fatalf("args are not laid out one per line:\n%s", before[filepath.Join(home, ".claude.json")])
	}

	inspection, err := Inspect(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range inspection.Hosts {
		for _, file := range host.Files {
			if !file.Current {
				t.Fatalf("%s %s is reported out of date after a relayout", host.Host, file.Path)
			}
		}
	}
	applyAll(t, request)
	for path, content := range before {
		if got := readTestFile(t, path); !bytes.Equal(got, content) {
			t.Fatalf("%s was rewritten although its managed entries were right", path)
		}
	}
}

func TestClaudeConfigDriftPreviewsManagedFieldsAndFixKeepsTheRest(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	claudePath := filepath.Join(home, ".claude.json")
	writeTestFile(t, claudePath, []byte(`{"numStartups": 42, "mcpServers": {"other": {"command": "other"}}}`), 0o600)
	request := Request{Workspace: workspace, Root: home, Scope: ScopeUser, Binary: filepath.Join(workspace, "bin", "skillstation"), Hosts: []Host{HostClaude}}
	applyAll(t, request)
	relayoutJSON(t, claudePath, func(document map[string]any) {
		server := document["mcpServers"].(map[string]any)["skillstation"].(map[string]any)
		server["command"] = "/moved/skillstation"
	})

	plan, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Path != claudePath {
		t.Fatalf("plan = %+v, want one change to %s", plan.Changes, claudePath)
	}
	// The allowed directories are already in ~/.claude/settings.json, so the
	// preview names only the managed MCP fields, each old value as a digest.
	lines := strings.Split(plan.Changes[0].Preview, "\n")
	if lines[0] != "managed MCP fields only:" || len(lines) != 4 {
		t.Fatalf("preview = %q, want the three managed fields only", plan.Changes[0].Preview)
	}
	for index, field := range []string{"type: sha256:", "command: sha256:", "args: sha256:"} {
		if !strings.HasPrefix(lines[index+1], field) || !strings.Contains(lines[index+1], " bytes) -> ") {
			t.Fatalf("preview line %q, want %q with digest and size", lines[index+1], field)
		}
	}

	applyAll(t, request)
	assertJSONRegistration(t, claudePath, request.Binary, workspace, false)
	assertContains(t, claudePath, `"numStartups": 42`)
	assertContains(t, claudePath, `"command": "other"`)
}

func TestClaudeConfigPreviewNamesDirectoriesMissingFromPermissionSettings(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	request := Request{Workspace: workspace, Root: home, Scope: ScopeUser, Binary: filepath.Join(workspace, "bin", "skillstation"), Hosts: []Host{HostClaude}}
	applyAll(t, request)
	dirs := RuntimeAccessDirs(workspace)
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	relayoutJSON(t, settingsPath, func(document map[string]any) {
		document["permissions"].(map[string]any)["additionalDirectories"] = []string{dirs[0], dirs[1]}
	})
	relayoutJSON(t, filepath.Join(home, ".claude.json"), func(document map[string]any) {
		document["mcpServers"].(map[string]any)["skillstation"].(map[string]any)["command"] = "/moved/skillstation"
	})

	plan, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 2 || plan.Changes[0].Kind != ChangeMCP || plan.Changes[1].Path != settingsPath {
		t.Fatalf("plan = %+v, want the config and the permission settings", plan.Changes)
	}
	if preview := plan.Changes[0].Preview; !strings.HasSuffix(preview, "\nallowed directories to add: "+dirs[2]) {
		t.Fatalf("config preview = %q, want only %s to add", preview, dirs[2])
	}
	applyAll(t, request)
	assertHasAll(t, "claude additionalDirectories", jsonStrings(t, settingsPath, "permissions", "additionalDirectories"), dirs)
}

func TestApplyRejectsPlanPathOutsideScopeContract(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	request := Request{Workspace: workspace, Root: home, Scope: ScopeUser, Binary: filepath.Join(workspace, "skillstation"), Hosts: []Host{HostClaude}}
	plan, err := Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// A user-scope plan replayed as project scope must not match the contract.
	plan.Scope = ScopeProject
	if _, err := Apply(context.Background(), plan, ApplyOptions{Confirmed: true}); err == nil || !strings.Contains(err.Error(), "host contract") {
		t.Fatalf("Apply error = %v, want host contract violation", err)
	}
}

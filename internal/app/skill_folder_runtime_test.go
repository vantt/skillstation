package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTestFolder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tool-runner")
	files["SKILL.md"] = "---\nname: tool-runner\ndescription: Runs a tool.\n---\n\nRun the tool.\n"
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// A hub that cannot be asked still leaves the author the findings the folder
// gives on its own.
func TestRemoteLintKeepsRuntimeFindingsWhenTheHubCannotBeAsked(t *testing.T) {
	t.Parallel()
	dir := writeTestFolder(t, map[string]string{"tools/package.json": "{}\n"})
	result, err := (RemoteLintService{}).Lint(context.Background(), RemoteHub{URL: "http://hub.invalid", MaySendToken: false}, dir, t.TempDir())
	if err != nil || ErrorOf(result, nil) == nil {
		t.Fatalf("want an error result, got %v, %v", result.Status, err)
	}
	found := false
	for _, finding := range result.Findings {
		found = found || finding.Code == "missing_runtime_block"
	}
	if !found {
		t.Fatalf("findings = %v", result.Findings)
	}
}

// The trial runs the block the way the hub serves a skill: a read-only copy of
// the folder, an empty state folder, check then setup then check.
func TestTryRunsCheckSetupCheckFromAReadOnlyCopy(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("the block's commands are POSIX shell")
	}
	runtimeBlock := func(setup, check string) string {
		return "runtime:\n  setup:\n    command: " + setup + "\n    check: " + check + "\n"
	}
	try := func(settings string, confirm bool) SkillTryResult {
		t.Helper()
		dir := writeTestFolder(t, map[string]string{"skill.yaml": settings, "tools/run.sh": "echo hi\n"})
		result, err := (SkillTryService{}).Try(context.Background(), dir, t.TempDir(), confirm)
		if err != nil || ErrorOf(result, nil) != nil {
			t.Fatalf("try: %v, %+v", err, result.Error)
		}
		return result
	}

	good := runtimeBlock(`'cp -R "$SKILLSTATION_SKILL_DIR/tools" ./tools'`, `'test -f tools/run.sh'`)
	if result := try(good, false); result.Outcome != TryPreview || len(result.Steps) != 0 {
		t.Fatalf("without confirmation nothing runs: %+v", result)
	}
	if result := try(good, true); result.Outcome != TryWorks || len(result.Steps) != 3 {
		t.Fatalf("a block that installs into the state folder: %+v", result)
	}
	// Installing into the skill folder is what a read-only copy forbids.
	bad := runtimeBlock(`'touch "$SKILLSTATION_SKILL_DIR/installed"'`, `'test -f "$SKILLSTATION_SKILL_DIR/installed"'`)
	if result := try(bad, true); result.Outcome != TryFails {
		t.Fatalf("a setup that writes into the skill folder worked: %+v", result)
	}
	if result := try(runtimeBlock(`'true'`, `'true'`), true); result.Outcome != TryWorks || len(result.Warnings) == 0 || result.Warnings[0].Code != "check_passes_before_setup" {
		t.Fatalf("a check that passes on an empty state folder: %+v", result)
	}
	if result := try("schema_version: 1\n", true); result.Outcome != TryNoRuntime {
		t.Fatalf("no runtime block: %+v", result)
	}
}

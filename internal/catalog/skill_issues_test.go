package catalog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/workspace"
)

// An assessment from an observed view reuses the view's scan and the
// validation of an unchanged tree, so an edited file, a conflicted Git index
// entry and a staged and committed edit must each reach the next one. The test
// stays serial because the validation memo is process-wide.
func TestAssessmentInViewSeesEditsIndexConflictsAndCommits(t *testing.T) {
	root := newWorkspace(t)
	writeReplaySkill(t, root, "Replay the run.")
	build(t, root, BuildOptions{})
	const entrypoint = "skills/core/replay/SKILL.md"
	git := func(stdin string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Skillstation Tests", "-c", "user.email=tests@example.invalid"}, args...)...)
		command.Stdin = strings.NewReader(stdin)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	issues := func(step string) []string {
		t.Helper()
		view, err := ObserveWithFallbackLocked(t.Context(), root)
		if err != nil {
			t.Fatalf("%s: observe: %v", step, err)
		}
		defer view.Close()
		assessment, err := AssessSkillStateInView(t.Context(), view, "replay")
		if err != nil {
			t.Fatalf("%s: assess: %v", step, err)
		}
		if !assessment.Canonical.Known {
			t.Fatalf("%s: the skill is not known: %#v", step, assessment)
		}
		return assessment.Canonical.Issues
	}
	expectValid := func(step string) {
		t.Helper()
		if found := issues(step); len(found) != 0 {
			t.Fatalf("%s: issues = %v, want none", step, found)
		}
	}
	expectIssue := func(step, want string) {
		t.Helper()
		found := issues(step)
		if !strings.Contains(strings.Join(found, "\n"), want) {
			t.Fatalf("%s: issues = %v, want one mentioning %q", step, found, want)
		}
	}
	broken := "schema_version: 1\nid: replay\nname: Replay\nstatus: shiny\ndescription: Replay the run.\nrouting:\n  triggers: [replay]\n  not_for: [unrelated]\n  min_scope: single_step\n"

	git("", "add", "-A", "skills")
	git("", "commit", "-q", "-m", "add replay")
	expectValid("first read")
	expectValid("unchanged tree")

	writeSkillMetadata(t, root, "skills/core/replay", broken)
	expectIssue("edited file", "skill status must be")
	writeReplaySkill(t, root, "Replay the run.")
	expectValid("restored file")

	// Only the index changes: the entrypoint becomes a conflict.
	blob := git("", "hash-object", "-w", entrypoint)
	git("0 0000000000000000000000000000000000000000\t"+entrypoint+"\n"+
		"100644 "+blob+" 1\t"+entrypoint+"\n100644 "+blob+" 2\t"+entrypoint+"\n100644 "+blob+" 3\t"+entrypoint+"\n",
		"update-index", "--index-info")
	expectIssue("conflicted index entry", entrypoint)
	git("", "add", entrypoint)
	expectValid("resolved index entry")

	writeSkillMetadata(t, root, "skills/core/replay", broken)
	git("", "add", "-A", "skills")
	expectIssue("staged edit", "skill status must be")
	git("", "commit", "-q", "-m", "break replay")
	expectIssue("committed edit", "skill status must be")
}

// An assessment from a view uses the view's scan of the canonical tree instead
// of listing it again, which stats every canonical file and on Windows opens
// each one: a tree that can no longer be listed once the view was observed is
// still assessed. The test stays serial because the validation memo is
// process-wide.
func TestAssessmentInViewDoesNotListTheTreeAgain(t *testing.T) {
	root := newWorkspace(t)
	writeReplaySkill(t, root, "Replay the run.")
	build(t, root, BuildOptions{})
	warm, err := ObserveWithFallbackLocked(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AssessSkillStateInView(t.Context(), warm, "replay"); err != nil {
		t.Fatal(err)
	}
	if err := warm.Close(); err != nil {
		t.Fatal(err)
	}

	view, err := ObserveWithFallbackLocked(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	// A canonical file over the per-file limit makes any listing fail.
	oversized := filepath.Join(root, "skills", "core", "replay", "references", "oversized.md")
	if err := os.MkdirAll(filepath.Dir(oversized), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oversized, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(oversized, workspace.MaxCanonicalFileBytesV1+1); err != nil {
		t.Fatal(err)
	}
	if _, err := AssessSkillStateWhileLocked(t.Context(), root, "replay"); err == nil {
		t.Fatal("an assessment that lists the tree accepted an oversized canonical file")
	}
	assessment, err := AssessSkillStateInView(t.Context(), view, "replay")
	if err != nil {
		t.Fatalf("the assessment listed the canonical tree again: %v", err)
	}
	if !assessment.Canonical.Known || !assessment.Canonical.Valid || assessment.ServingMode != ServingCurrent {
		t.Fatalf("assessment = %#v, want the valid skill served from the current generation", assessment)
	}
}

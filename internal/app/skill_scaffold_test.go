package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldProjectSkillWritesOnlyInTheProjectAndFeedsTheCommit(t *testing.T) {
	t.Parallel()
	hub := newGitHub(t)
	hubBefore := hubGit(t, hub, "status", "--porcelain", "--untracked-files=all")
	project := newProjectDir(t)

	result, err := ScaffoldProjectSkill(context.Background(), SkillScaffoldInput{
		SkillID: "deploy-notes", ProjectDir: project, Description: "Summarize a deploy for the team channel",
		Triggers: []string{"after a production deploy", "write a commit message"}, NotFor: []string{"staging deploys"}, Examples: []string{"write the deploy notes for today's release"},
	})
	if err != nil || result.Error != nil {
		t.Fatalf("scaffold: %v %#v", err, result.Error)
	}
	if result.Path != ".agents/skills/deploy-notes/SKILL.md" || result.Summary != "Created .agents/skills/deploy-notes/ with SKILL.md and skill.yaml (nothing changed on the hub)." {
		t.Fatalf("result = %+v", result)
	}
	folder := filepath.Join(project, ".agents", "skills", "deploy-notes")
	data, err := os.ReadFile(filepath.Join(folder, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: deploy-notes", "Summarize a deploy for the team channel. Use after a production deploy. Use it to write a commit message.", "## Instructions"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("SKILL.md lacks %q:\n%s", want, data)
		}
	}
	// The routing the form collected goes to skill.yaml, the one place the
	// hub reads it from; SKILL.md does not repeat it.
	if strings.Contains(string(data), "## When to use") || strings.Contains(string(data), "## Examples") {
		t.Fatalf("SKILL.md repeats the routing:\n%s", data)
	}
	settings, err := os.ReadFile(filepath.Join(folder, "skill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- after a production deploy", "- write a commit message", "- staging deploys", "- write the deploy notes for today's release"} {
		if !strings.Contains(string(settings), want) {
			t.Fatalf("skill.yaml lacks %q:\n%s", want, settings)
		}
	}
	if ignore, err := os.ReadFile(filepath.Join(project, ".gitignore")); err != nil || !strings.Contains(string(ignore), "/.agents/skills/deploy-notes/\n/.claude/skills/deploy-notes/\n") || strings.Join(result.GitignoreAdded, " ") != "/.agents/skills/deploy-notes/ /.claude/skills/deploy-notes/" || result.ClaudePointer != ".claude/skills/deploy-notes/SKILL.md" {
		t.Fatalf("the created folder must stay out of the project's Git like a checkout: %q %v %+v", ignore, err, result)
	}
	if _, found, _ := ReadCheckoutMarker(folder); found {
		t.Fatalf("a created folder has no marker: its first commit is a new skill")
	}
	if hubGit(t, hub, "status", "--porcelain", "--untracked-files=all") != hubBefore {
		t.Fatalf("create changed the hub")
	}
	preview := previewCommit(t, hub, project, "deploy-notes", SkillCommitInput{})
	if preview.Case != commitCaseNew || preview.Routing == nil || len(preview.Routing.Triggers) != 2 || len(preview.Routing.Examples) != 1 || preview.RoutingSuggestions != nil {
		t.Fatalf("the commit must keep the routing the form collected: %+v %+v", preview.Routing, preview.RoutingSuggestions)
	}
	if preview.ActivationReady {
		t.Fatalf("an untouched scaffold must not be offered for activation")
	}

	again, err := ScaffoldProjectSkill(context.Background(), SkillScaffoldInput{SkillID: "deploy-notes", ProjectDir: project, Description: "x"})
	if err != nil || again.Error == nil || again.Error.Code != ErrorSkillConflict {
		t.Fatalf("an existing folder must be refused, got %+v %v", again.Error, err)
	}
	if bad, err := ScaffoldProjectSkill(context.Background(), SkillScaffoldInput{SkillID: "Bad Id", ProjectDir: project, Description: "x"}); err != nil || bad.Error == nil {
		t.Fatalf("a bad id must be refused, got %+v %v", bad.Error, err)
	}
}

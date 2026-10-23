package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moveToClaudeFolder turns a checkout into one an older release made: the
// files with their marker in .claude/skills/<id>/ and nothing in .agents.
func moveToClaudeFolder(t *testing.T, project, id string) string {
	t.Helper()
	legacy := filepath.Join(project, ".claude", "skills", id)
	if err := os.RemoveAll(legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(project, ".agents", "skills", id), legacy); err != nil {
		t.Fatal(err)
	}
	return legacy
}

func commitRefusal(t *testing.T, hub, project, target string) *Error {
	t.Helper()
	preview, err := (SkillCommitService{}).Preview(context.Background(), hub, SkillCommitInput{Target: target, ProjectDir: project})
	if err != nil {
		t.Fatal(err)
	}
	return preview.Error
}

func TestAnOlderCheckoutInTheClaudeFolderIsStillFoundAndBlocksANewOne(t *testing.T) {
	t.Parallel()
	hub, project, _ := checkedOut(t, fixtureSkill{id: "alpha"})
	legacy := moveToClaudeFolder(t, project, "alpha")
	replaceInProject(t, legacy, "line one", "line 1")

	if preview := previewCommit(t, hub, project, "alpha", SkillCommitInput{}); preview.Checkout == nil || preview.Checkout.Path != legacy || len(preview.Files) != 1 {
		t.Fatalf("commit by id must read the older checkout: %+v", preview)
	}
	pulled, err := (SkillPullService{}).Pull(context.Background(), hub, SkillPullInput{Target: "alpha", ProjectDir: project})
	if err != nil || pulled.Error != nil || pulled.Folder != legacy {
		t.Fatalf("pull by id must read the older checkout: %+v %v", pulled, err)
	}
	again, err := (SkillCheckoutService{}).Checkout(context.Background(), hub, SkillCheckoutInput{SkillID: "alpha", ProjectDir: project})
	if err != nil || again.Error == nil || !strings.Contains(again.Error.Render.Why, ".claude/skills/alpha/ is a checkout from") {
		t.Fatalf("a second checkout beside the older one must be refused: %+v %v", again.Error, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("the refused checkout wrote the shared folder: %v", err)
	}
	home, err := (CurationService{ProjectDir: project}).GetCurationHome(context.Background(), hub)
	if err != nil {
		t.Fatal(err)
	}
	if action, found := openCheckoutOf(t, home, "alpha"); !found || !strings.Contains(action.Summary, "(in .claude/skills/alpha/)") {
		t.Fatalf("status must list the older checkout by its folder: %+v %v", action, found)
	}
}

func TestTwoCopiesOfOneSkillAreNamedAndNeverGuessedBetween(t *testing.T) {
	t.Parallel()
	hub, project, shared := checkedOut(t, fixtureSkill{id: "alpha"})
	// The pointer is overwritten with a full copy, as a person or an agent
	// editing .claude/skills by habit would.
	claude := filepath.Join(project, ".claude", "skills", "alpha")
	data, err := os.ReadFile(filepath.Join(shared, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeProjectFile(t, claude, "SKILL.md", strings.Replace(string(data), "line one", "line 1", 1), 0o644)

	refusal := commitRefusal(t, hub, project, "alpha")
	if refusal == nil || refusal.Code != ErrorSkillConflict ||
		!strings.Contains(refusal.Render.Why, ".agents/skills/alpha/ is a checkout from") ||
		!strings.Contains(refusal.Render.Why, ".claude/skills/alpha/ is a copy that was not checked out") ||
		!strings.Contains(refusal.Render.Fix, "./.agents/skills/alpha") {
		t.Fatalf("commit by id must name both copies: %+v", refusal)
	}
	if pulled, err := (SkillPullService{}).Pull(context.Background(), hub, SkillPullInput{Target: "alpha", ProjectDir: project}); err != nil || pulled.Error == nil || pulled.Error.Code != ErrorSkillConflict {
		t.Fatalf("pull by id must refuse two copies too: %+v %v", pulled.Error, err)
	}
	if preview := previewCommit(t, hub, project, "./.claude/skills/alpha", SkillCommitInput{}); preview.Checkout != nil || preview.SkillID != "alpha" {
		t.Fatalf("a path names one copy: %+v", preview)
	}
	if preview := previewCommit(t, hub, project, "./.agents/skills/alpha", SkillCommitInput{}); preview.Checkout == nil || preview.Checkout.Path != shared {
		t.Fatalf("a path names one copy: %+v", preview)
	}
}

func TestTwoCheckoutsOfOneSkillEachNameTheirFolderInStatus(t *testing.T) {
	t.Parallel()
	hub, project, _ := checkedOut(t, fixtureSkill{id: "alpha"})
	moveToClaudeFolder(t, project, "alpha")
	// A checkout made before the move found nothing in .agents, so a second
	// copy can only arrive by hand; copy the marker along to stand in for it.
	if result, err := (SkillCheckoutService{}).Checkout(context.Background(), hub, SkillCheckoutInput{SkillID: "alpha", ProjectDir: project}); err != nil || result.Error == nil {
		t.Fatalf("checkout beside an older checkout must be refused: %+v %v", result, err)
	}
	legacy := filepath.Join(project, ".claude", "skills", "alpha")
	shared := filepath.Join(project, ".agents", "skills", "alpha")
	for _, name := range []string{"SKILL.md", CheckoutMarkerName} {
		data, err := os.ReadFile(filepath.Join(legacy, name))
		if err != nil {
			t.Fatal(err)
		}
		writeProjectFile(t, shared, name, string(data), 0o644)
	}
	home, err := (CurationService{ProjectDir: project}).GetCurationHome(context.Background(), hub)
	if err != nil {
		t.Fatal(err)
	}
	commands := map[string]bool{}
	for _, action := range home.Actions {
		if action.Kind == "open_checkout" && action.ID == "alpha" {
			if !strings.Contains(action.Summary, "alpha is checked out twice in this project") {
				t.Fatalf("summary must say there are two: %+v", action)
			}
			commands[action.Command] = true
		}
	}
	if !commands["skillstation skill commit ./.agents/skills/alpha"] || !commands["skillstation skill commit ./.claude/skills/alpha"] {
		t.Fatalf("each checkout must be committed by its path: %v", commands)
	}
}

func TestAPointerWithoutItsSkillIsNeverCommittedAndIsReplacedByANewCheckout(t *testing.T) {
	t.Parallel()
	hub, project, shared := checkedOut(t, fixtureSkill{id: "alpha"})
	if err := os.RemoveAll(shared); err != nil {
		t.Fatal(err)
	}
	refusal := commitRefusal(t, hub, project, "alpha")
	if refusal == nil || !strings.Contains(refusal.Render.Why, "pointer Skillstation wrote for Claude Code") || !strings.Contains(refusal.Render.Why, "are gone") {
		t.Fatalf("commit by id with only a pointer left: %+v", refusal)
	}
	if refusal := commitRefusal(t, hub, project, "./.claude/skills/alpha"); refusal == nil || !strings.Contains(refusal.Render.Why, "holds only the pointer") {
		t.Fatalf("commit of the pointer folder by path: %+v", refusal)
	}
	result, err := (SkillCheckoutService{}).Checkout(context.Background(), hub, SkillCheckoutInput{SkillID: "alpha", ProjectDir: project})
	if err != nil || result.Error != nil || result.Folder != shared {
		t.Fatalf("a checkout over a lone pointer: %+v %v", result, err)
	}
}

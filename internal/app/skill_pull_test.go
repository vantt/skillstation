package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func pullSkill(t *testing.T, hub, project, target string, resolutions ...SkillCommitResolution) SkillPullResult {
	t.Helper()
	result, err := (SkillPullService{}).Pull(context.Background(), hub, SkillPullInput{Target: target, ProjectDir: project, Resolutions: resolutions})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSkillPullBringsTheHubVersionIntoAnUntouchedCheckout(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf", files: map[string]string{"references/old.md": "old\n"}})
	replaceInHub(t, hub, "pdf", "line three", "hub three")
	if err := os.Remove(filepath.Join(hub, "skills", "default", "pdf", "references", "old.md")); err != nil {
		t.Fatal(err)
	}
	commitHubAll(t, hub, "hub removes old.md")

	if preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{}); !preview.NothingToCommit || !strings.Contains(preview.Summary, "skillstation skill pull pdf") {
		t.Fatalf("commit must point at pull when only the hub changed: %q", preview.Summary)
	}
	result := pullSkill(t, hub, project, "pdf")
	if result.Error != nil || len(result.Conflicts) != 0 || len(result.Files) != 2 || len(result.Kept) != 0 {
		t.Fatalf("pull = %+v", result)
	}
	if got, _ := os.ReadFile(filepath.Join(folder, "SKILL.md")); !strings.Contains(string(got), "hub three") {
		t.Fatalf("SKILL.md was not updated:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(folder, "references", "old.md")); !os.IsNotExist(err) {
		t.Fatalf("a file the hub removed stayed in the folder")
	}
	if preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{}); !preview.NothingToCommit || !strings.Contains(preview.Summary, "matches the hub") {
		t.Fatalf("after a pull the folder must match the hub: %q", preview.Summary)
	}
	if again := pullSkill(t, hub, project, "pdf"); !again.UpToDate {
		t.Fatalf("second pull = %+v", again)
	}
}

func TestSkillPullKeepsTheProjectsOwnChanges(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	replaceInHub(t, hub, "pdf", "line three", "hub three")
	replaceInProject(t, folder, "line one", "project one")
	writeProjectFile(t, folder, "notes.md", "mine\n", 0o644)
	writeProjectFile(t, folder, ".env", "TOKEN=1\n", 0o644)

	result := pullSkill(t, hub, project, "pdf")
	if result.Error != nil || len(result.Conflicts) != 0 || !slices.Equal(result.Kept, []string{"SKILL.md", "notes.md"}) {
		t.Fatalf("pull = %+v", result)
	}
	got, _ := os.ReadFile(filepath.Join(folder, "SKILL.md"))
	if !strings.Contains(string(got), "project one") || !strings.Contains(string(got), "hub three") {
		t.Fatalf("merge lost a side:\n%s", got)
	}
	for _, rel := range []string{"notes.md", ".env"} {
		if _, err := os.Stat(filepath.Join(folder, rel)); err != nil {
			t.Fatalf("%s did not survive the pull: %v", rel, err)
		}
	}
	preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	if preview.Case != commitCaseUpdate || len(preview.Files) != 2 {
		t.Fatalf("after a pull the commit sends only the project's changes: case=%s files=%+v", preview.Case, preview.Files)
	}
}

func TestSkillPullWritesNothingUntilEveryConflictIsDecided(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	replaceInHub(t, hub, "pdf", "line two", "hub two")
	replaceInProject(t, folder, "line two", "project two")

	result := pullSkill(t, hub, project, "pdf")
	if result.Status != StatusActionRequired || !slices.Equal(result.Conflicts, []string{"SKILL.md"}) {
		t.Fatalf("pull = %+v", result)
	}
	marker, _, _ := ReadCheckoutMarker(folder)
	if got, _ := os.ReadFile(filepath.Join(folder, "SKILL.md")); !strings.Contains(string(got), "project two") || marker.BaseVersion == result.BaseVersion {
		t.Fatalf("an undecided pull wrote the folder or its marker")
	}
	if refused := pullSkill(t, hub, project, "pdf", SkillCommitResolution{Path: "notes.md", Choice: "hub"}); refused.Error == nil {
		t.Fatalf("deciding a file without a difference must be refused")
	}
	decided := pullSkill(t, hub, project, "pdf", SkillCommitResolution{Path: "SKILL.md", Choice: "hub"})
	if decided.Error != nil || len(decided.Conflicts) != 0 {
		t.Fatalf("decided pull = %+v", decided)
	}
	if got, _ := os.ReadFile(filepath.Join(folder, "SKILL.md")); !strings.Contains(string(got), "hub two") {
		t.Fatalf("the hub's version was not taken:\n%s", got)
	}
}

func TestSkillPullRefusesAFolderThatWasNotCheckedOut(t *testing.T) {
	t.Parallel()
	hub := newGitHub(t)
	project := newProjectDir(t)
	newProjectSkill(t, project)
	result := pullSkill(t, hub, project, "team-release")
	if result.Error == nil || !strings.Contains(result.Error.Render.Why, "not checked out") {
		t.Fatalf("pull = %+v", result.Result)
	}
}

func TestMergeByDigestTakesTheSideThatChanged(t *testing.T) {
	t.Parallel()
	file := func(text string) treeFile { return treeFile{Data: []byte(text)} }
	digests := func(tree map[string]treeFile) map[string]string {
		out := map[string]string{}
		for rel, f := range tree {
			out[rel] = digestOf(f.Data)
		}
		return out
	}
	base := map[string]treeFile{"hub-edits.md": file("a"), "mine.md": file("b"), "both.md": file("c"), "gone.md": file("d")}
	hubTree := map[string]treeFile{"hub-edits.md": file("A"), "mine.md": file("b"), "both.md": file("hub"), "gone.md": file("d"), "hub-new.md": file("n")}
	projectTree := map[string]treeFile{"hub-edits.md": file("a"), "mine.md": file("B"), "both.md": file("project"), "own-new.md": file("o")}

	final, unresolved, refusal := mergeByDigest(hubTree, projectTree, digests(base), nil)
	if refusal != nil || !slices.Equal(unresolved, []string{"both.md"}) {
		t.Fatalf("unresolved = %v refusal = %+v", unresolved, refusal)
	}
	want := map[string]string{"hub-edits.md": "A", "mine.md": "B", "both.md": "project", "hub-new.md": "n", "own-new.md": "o"}
	if len(final) != len(want) {
		t.Fatalf("final = %v", final)
	}
	for rel, text := range want {
		if string(final[rel].Data) != text {
			t.Fatalf("%s = %q, want %q", rel, final[rel].Data, text)
		}
	}
	final, unresolved, _ = mergeByDigest(hubTree, projectTree, digests(base), []SkillCommitResolution{{Path: "both.md", Choice: "hub"}})
	if len(unresolved) != 0 || string(final["both.md"].Data) != "hub" {
		t.Fatalf("decided: unresolved=%v both=%q", unresolved, final["both.md"].Data)
	}
}

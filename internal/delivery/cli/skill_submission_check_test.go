package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The send preview of a project connected to a hub on another machine says,
// before anything is sent, whether the id is new or replaces a skill the hub
// serves, and whether the hub moved on since the checkout. It refuses a folder
// the hub could not take as a skill, and status keeps naming a checkout that
// was left open.
func TestRemoteSendPreviewComparesWithTheHubAndChecksSkillMD(t *testing.T) {
	project := newRemoteProject(t)
	for _, id := range []string{"shared-notes", "review-checklist"} {
		project.hub.createReady(id)
		project.hub.mustRun("skill", "activate", id, "--yes")
	}
	create := func(id string) {
		t.Helper()
		if code, stdout, stderr := runCLI(t, "skill", "create", id, "--description", "Notes for "+id, "--trigger", "write "+id, "--not-for", "deploying", "--example", "write the "+id); code != 0 {
			t.Fatalf("create %s: exit = %d\n%s%s", id, code, stdout, stderr)
		}
	}

	create("team-release")
	_, stdout, _ := runCLI(t, "skill", "commit", "team-release")
	if !strings.Contains(oneLine(stdout), "no skill with this id is served by the hub") || strings.Contains(stdout, "replacement") {
		t.Fatalf("a new id:\n%s", stdout)
	}

	create("shared-notes")
	_, stdout, _ = runCLI(t, "skill", "commit", "shared-notes")
	if !strings.Contains(oneLine(stdout), "already has a skill with the id shared-notes: sending this folder proposes a replacement") {
		t.Fatalf("an id the hub has:\n%s", stdout)
	}

	broken := filepath.Join(project.dir, "broken-notes")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(project.dir, "plain-notes")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, "SKILL.md"), []byte("---\nname: plain-notes\ndescription: Notes kept plain.\n---\n\nKeep notes plain.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := runCLI(t, "skill", "commit", "plain-notes"); code != 0 || !strings.Contains(oneLine(stdout), "plain-notes: no skill with this id is served by the hub") {
		t.Fatalf("a folder named by its bare name: exit = %d\n%s%s", code, stdout, stderr)
	}

	if err := os.WriteFile(filepath.Join(broken, "SKILL.md"), []byte("---\nname: broken-notes\ndescription: never closed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "skill", "commit", "./broken-notes", "--yes")
	if code == 0 || !strings.Contains(oneLine(stdout+stderr), "unclosed frontmatter delimiter") {
		t.Fatalf("a SKILL.md whose frontmatter is not closed: exit = %d\n%s%s", code, stdout, stderr)
	}

	if code, stdout, stderr = runCLI(t, "skill", "checkout", "review-checklist"); code != 0 {
		t.Fatalf("checkout: exit = %d\n%s%s", code, stdout, stderr)
	}
	folder := filepath.Join(project.dir, ".agents", "skills", "review-checklist")
	appendToFile(t, filepath.Join(folder, "SKILL.md"), "\nOne more line from this project.\n")
	_, stdout, _ = runCLI(t, "skill", "commit", "review-checklist")
	if strings.Contains(oneLine(stdout), "changed since you checked it out") {
		t.Fatalf("the hub did not move yet:\n%s", stdout)
	}

	code, stdout, stderr = runCLI(t, "status", "--json")
	if code != 0 {
		t.Fatalf("status: exit = %d\n%s%s", code, stdout, stderr)
	}
	var status struct {
		Actions []struct {
			Kind, ID, Command, Summary string
			Count                      int
		}
	}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatal(err)
	}
	reminded := false
	for _, action := range status.Actions {
		if action.Kind == "open_checkout" && action.ID == "review-checklist" {
			reminded = action.Count == 1 && action.Command == "skillstation skill commit review-checklist" && strings.Contains(action.Summary, "1 file changed since checkout")
		}
	}
	if !reminded {
		t.Fatalf("status does not name the open checkout:\n%s", stdout)
	}

	hubFile := filepath.Join(project.hub.root, "skills", "software", "review-checklist", "SKILL.md")
	appendToFile(t, hubFile, "\nA line added on the hub.\n")
	project.hub.mustRun("maintain", "rebuild")
	_, stdout, _ = runCLI(t, "skill", "commit", "review-checklist")
	if !strings.Contains(oneLine(stdout), "The hub's copy of review-checklist changed since you checked it out. Run `skillstation skill pull review-checklist` first") {
		t.Fatalf("the hub moved:\n%s", stdout)
	}
}

// A project connected to a hub on another machine can ask the hub to check the
// routing of a folder before sending it: the hub compares it with its active
// skills and keeps nothing.
func TestRemoteLintChecksAFolderAgainstTheHubsSkills(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("shared-notes")
	project.hub.mustRun("skill", "activate", "shared-notes", "--yes")
	create := func(id, trigger string) {
		t.Helper()
		if code, stdout, stderr := runCLI(t, "skill", "create", id, "--description", "Notes for "+id, "--trigger", trigger, "--not-for", "deploying", "--example", trigger); code != 0 {
			t.Fatalf("create %s: exit = %d\n%s%s", id, code, stdout, stderr)
		}
	}
	lint := func(target string) (found []string, stdout string) {
		t.Helper()
		code, out, stderr := runCLI(t, "skill", "lint", target, "--json")
		var report struct {
			SkillID  string `json:"skill_id"`
			Findings []struct {
				Code         string `json:"code"`
				OtherSkillID string `json:"other_skill_id"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil || code != 0 || report.SkillID != target {
			t.Fatalf("lint %s: exit = %d, %v\n%s%s", target, code, err, out, stderr)
		}
		for _, finding := range report.Findings {
			found = append(found, finding.Code+":"+finding.OtherSkillID)
		}
		return found, out
	}

	create("release-helper", "cut a release")
	found, _ := lint("release-helper")
	collides := false
	for _, finding := range found {
		collides = collides || strings.HasSuffix(finding, ":shared-notes")
	}
	if !collides {
		t.Fatalf("a trigger the hub's skill already has: findings = %v", found)
	}

	create("tidy-notes", "write tidy notes")
	if found, _ = lint("tidy-notes"); slices.ContainsFunc(found, func(finding string) bool { return strings.HasSuffix(finding, ":shared-notes") }) {
		t.Fatalf("a trigger of its own collides with the hub's skill: findings = %v", found)
	}

	if entries, _ := os.ReadDir(filepath.Join(project.hub.root, "runtime", "submissions")); len(entries) != 0 {
		t.Fatalf("a routing check queued %d submissions on the hub", len(entries))
	}
	if code, stdout, stderr := runCLI(t, "skill", "lint"); code == 0 || !strings.Contains(oneLine(stdout+stderr), "lint needs a skill folder") {
		t.Fatalf("lint with no folder: exit = %d\n%s%s", code, stdout, stderr)
	}
}

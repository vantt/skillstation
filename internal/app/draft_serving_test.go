package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/catalog"
	resolverpkg "github.com/vantt/skillstation/internal/resolver"
)

func draftResolveRequest(description string) resolverpkg.Request {
	return resolverpkg.Request{
		SchemaVersion: resolverpkg.SchemaVersion,
		RequestID:     "req-draft",
		Task:          resolverpkg.Task{Description: description, Scope: "multi_step"},
	}
}

func resolveFrom(t *testing.T, hub, dir, description string) resolverpkg.Response {
	t.Helper()
	ctx := WithProjectDir(context.Background(), dir)
	response, err := (ResolverService{}).Resolve(ctx, hub, draftResolveRequest(description))
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestTheProjectWithADraftRoutesAndReadsTheDraft(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	settings := filepath.Join(folder, "skill.yaml")
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	routed := strings.Replace(string(data), "- run pdf\n", "- run pdf\n        - convert scanned invoices into searchable text\n", 1)
	if err := os.WriteFile(settings, []byte(routed), 0o644); err != nil {
		t.Fatal(err)
	}
	replaceInProject(t, folder, "line one", "line one from the draft")
	task := "convert scanned invoices into searchable text"

	subfolder := filepath.Join(project, "src", "billing")
	if err := os.MkdirAll(subfolder, 0o755); err != nil {
		t.Fatal(err)
	}
	response := resolveFrom(t, hub, subfolder, task)
	if response.Primary == nil || response.Primary.ID != "pdf" || response.Primary.Draft != folder {
		t.Fatalf("the project with the draft must route the draft: %+v", response.Primary)
	}
	core, served, local, found, err := (SkillService{}).ReadDraftSkill(WithProjectDir(context.Background(), subfolder), hub, "pdf")
	if err != nil || !found || served.Folder != folder || local.Path != folder || !strings.Contains(core.Content, "line one from the draft") {
		t.Fatalf("skill_get of the draft: found=%v err=%v served=%+v local=%+v", found, err, served, local)
	}

	elsewhere := newProjectDir(t)
	if official := resolveFrom(t, hub, elsewhere, task); official.Primary != nil && official.Primary.Draft != "" {
		t.Fatalf("another project must get the official version: %+v", official.Primary)
	}
	if _, _, _, found, err := (SkillService{}).ReadDraftSkill(WithProjectDir(context.Background(), elsewhere), hub, "pdf"); err != nil || found {
		t.Fatalf("another project read the draft: found=%v err=%v", found, err)
	}
	if inHub := resolveFrom(t, hub, hub, task); inHub.Primary != nil && inHub.Primary.Draft != "" {
		t.Fatalf("an agent in the hub folder must get the official version: %+v", inHub.Primary)
	}
}

func TestADraftOfANewSkillRoutesInItsProject(t *testing.T) {
	t.Parallel()
	hub := newGitHub(t)
	if _, err := catalog.BuildCatalogGeneration(context.Background(), hub, catalog.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	project := newProjectDir(t)
	result, err := ScaffoldProjectSkill(context.Background(), SkillScaffoldInput{
		SkillID: "deploy-notes", ProjectDir: project, Description: "Summarize a production deploy for the team channel", Hub: hub,
		Triggers: []string{"summarize a production deploy for the team channel"},
	})
	if err != nil || result.Error != nil {
		t.Fatalf("scaffold: %v %#v", err, result.Error)
	}
	response := resolveFrom(t, hub, project, "summarize a production deploy for the team channel")
	if response.Primary == nil || response.Primary.ID != "deploy-notes" || response.Primary.Draft != result.Folder {
		t.Fatalf("the new skill's draft must route in its project: %+v", response.Primary)
	}
}

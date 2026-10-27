package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/skill"
)

func draftsIn(t *testing.T, hub string) []DraftRecord {
	t.Helper()
	drafts, err := loadDrafts(hub)
	if err != nil {
		t.Fatal(err)
	}
	return drafts
}

func TestCheckoutRecordsTheDraftAndACommitClosesIt(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	drafts := draftsIn(t, hub)
	if len(drafts) != 1 || drafts[0].SkillID != "pdf" || drafts[0].Folder != ".agents/skills/pdf" || drafts[0].Pointer != ".claude/skills/pdf/SKILL.md" || drafts[0].BaseVersion == "" || drafts[0].StartedAt.IsZero() {
		t.Fatalf("drafts after checkout = %+v", drafts)
	}
	if resolved, _ := filepath.EvalSymlinks(drafts[0].ProjectRoot); resolved != mustEvalSymlinks(t, project) {
		t.Fatalf("draft project = %s, want %s", drafts[0].ProjectRoot, project)
	}

	replaceInProject(t, folder, "line one", "line 1 from the project")
	preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	result, err := (SkillCommitService{}).Apply(commitCtx(), hub, preview, SkillCommitApplyOptions{})
	if err != nil || result.Error != nil {
		t.Fatalf("apply: %v %#v", err, result.Error)
	}
	if !result.DraftClosed || len(result.LeftOut) != 0 || result.Checkout != nil {
		t.Fatalf("result = closed %v, left out %v, checkout %+v", result.DraftClosed, result.LeftOut, result.Checkout)
	}
	for _, gone := range []string{folder, filepath.Join(project, ".claude", "skills", "pdf")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived the closed draft: %v", gone, err)
		}
	}
	if drafts := draftsIn(t, hub); len(drafts) != 0 {
		t.Fatalf("the hub still records the closed draft: %+v", drafts)
	}
	if ignore, _ := os.ReadFile(filepath.Join(project, ".gitignore")); !strings.Contains(string(ignore), "/.agents/skills/pdf/") {
		t.Fatalf("the .gitignore lines must stay: %q", ignore)
	}
	if !strings.Contains(hubSkillMD(t, hub, "pdf"), "line 1 from the project") {
		t.Fatalf("the hub lacks the committed edit")
	}
}

func TestACommitKeepsADraftThatHoldsLeftOutFiles(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	writeProjectFile(t, folder, ".env", "TOKEN=abc\n", 0o600)
	replaceInProject(t, folder, "line one", "line 1 from the project")
	preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	result, err := (SkillCommitService{}).Apply(commitCtx(), hub, preview, SkillCommitApplyOptions{})
	if err != nil || result.Error != nil {
		t.Fatalf("apply: %v %#v", err, result.Error)
	}
	if result.DraftClosed || !slices.Equal(result.LeftOut, []string{".env"}) || result.Checkout == nil {
		t.Fatalf("result = closed %v, left out %v, checkout %+v", result.DraftClosed, result.LeftOut, result.Checkout)
	}
	if data, err := os.ReadFile(filepath.Join(folder, ".env")); err != nil || string(data) != "TOKEN=abc\n" {
		t.Fatalf("the left-out file was touched: %q %v", data, err)
	}
	drafts := draftsIn(t, hub)
	if len(drafts) != 1 || drafts[0].BaseVersion != result.Checkout.BaseVersion {
		t.Fatalf("the kept draft must be recorded at its new base: %+v", drafts)
	}
}

func TestCreateRecordsTheDraftOnTheHubItUses(t *testing.T) {
	t.Parallel()
	hub := newGitHub(t)
	project := newProjectDir(t)
	result, err := ScaffoldProjectSkill(context.Background(), SkillScaffoldInput{SkillID: "deploy-notes", ProjectDir: project, Description: "Summarize a deploy", Hub: hub})
	if err != nil || result.Error != nil {
		t.Fatalf("scaffold: %v %#v", err, result.Error)
	}
	drafts := draftsIn(t, hub)
	if len(drafts) != 1 || drafts[0].SkillID != "deploy-notes" || drafts[0].BaseVersion != "" {
		t.Fatalf("drafts after create = %+v", drafts)
	}
}

func TestDraftsNeverTouchTheHubsOwnMetaFiles(t *testing.T) {
	t.Parallel()
	hub := newGitHub(t)
	seedHubSkill(t, hub, fixtureSkill{id: "pdf"})
	distill := filepath.Join(hub, "skills", "default", "pdf", ".meta", "distill.yaml")
	writeProjectFile(t, filepath.Dir(distill), "distill.yaml", "schema_version: 1\nlessons: []\n", 0o644)
	commitHubAll(t, hub, "add pdf")
	before, err := os.ReadFile(distill)
	if err != nil {
		t.Fatal(err)
	}
	project := newProjectDir(t)
	if result, err := (SkillCheckoutService{}).Checkout(context.Background(), hub, SkillCheckoutInput{SkillID: "pdf", ProjectDir: project}); err != nil || result.Error != nil {
		t.Fatalf("checkout: %v %#v", err, result.Error)
	}
	folder := filepath.Join(project, ".agents", "skills", "pdf")
	if _, err := os.Stat(filepath.Join(folder, ".meta")); !os.IsNotExist(err) {
		t.Fatalf("a draft must not carry .meta/: %v", err)
	}
	replaceInProject(t, folder, "line one", "line 1 from the project")
	preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	if result, err := (SkillCommitService{}).Apply(commitCtx(), hub, preview, SkillCommitApplyOptions{}); err != nil || result.Error != nil {
		t.Fatalf("apply: %v %#v", err, result.Error)
	}
	if after, err := os.ReadFile(distill); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("distill.yaml changed: %q %v", after, err)
	}
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestAThirdPartySkillNeedsNoNewApprovalWhenOnlyItsRoutingChanges(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf", origin: "github"})
	replaceInProject(t, folder, "line one", "line one, edited")
	approve := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	if result, err := (SkillCommitService{}).Apply(commitCtx(), hub, approve, SkillCommitApplyOptions{Approve: true}); err != nil || result.Error != nil || !result.Approved {
		t.Fatalf("approve the content: %+v %v", result.Error, err)
	}

	checkout := func() string {
		t.Helper()
		if result, err := (SkillCheckoutService{}).Checkout(context.Background(), hub, SkillCheckoutInput{SkillID: "pdf", ProjectDir: project}); err != nil || result.Error != nil {
			t.Fatalf("checkout: %v %#v", err, result.Error)
		}
		return filepath.Join(project, ".agents", "skills", "pdf")
	}
	editSettings := func(folder, from, to string) {
		t.Helper()
		path := filepath.Join(folder, "skill.yaml")
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), from) {
			t.Fatalf("skill.yaml lacks %q: %v\n%s", from, err, data)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(data), from, to, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	folder = checkout()
	editSettings(folder, "- run pdf\n", "- run pdf\n        - fill a pdf form\n")
	routing := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	if routing.Trust == nil || !routing.Trust.ThirdParty || !routing.Trust.ContentKept || routing.Trust.NeedsApproval {
		t.Fatalf("a routing change must keep the approval: %+v", routing.Trust)
	}
	if result, err := (SkillCommitService{}).Apply(commitCtx(), hub, routing, SkillCommitApplyOptions{}); err != nil || result.Error != nil || result.Approved {
		t.Fatalf("apply without approval: %+v %v", result.Error, err)
	}
	review, err := (SkillService{}).ReviewSkill(context.Background(), hub, "pdf")
	if err != nil || !review.ContentTrust.Approved || !strings.Contains(string(readSkillMetadata(t, hub, "skills/default/pdf")), "fill a pdf form") {
		t.Fatalf("after a routing change: approved = %v, err = %v", review.ContentTrust.Approved, err)
	}

	folder = checkout()
	editSettings(folder, "routing:", "runtime:\n    requires:\n        bins:\n            - python3\nrouting:")
	runtime := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	if runtime.Trust == nil || runtime.Trust.ContentKept || !runtime.Trust.NeedsApproval {
		t.Fatalf("a runtime change must need a new approval: %+v", runtime.Trust)
	}
	if refused, err := (SkillCommitService{}).Apply(commitCtx(), hub, runtime, SkillCommitApplyOptions{}); err != nil || refused.Error == nil || refused.Error.Code != ErrorContentReviewRequired {
		t.Fatalf("a runtime change without approval = %+v %v", refused.Error, err)
	}
}

func TestAChangeElsewhereWarnsAboutTheOpenDraftAndItsCommitNamesIt(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})

	edit, err := (SkillService{}).PreviewSkillUpdate(context.Background(), hub, "pdf", skill.UpdateInput{Routing: &skill.RoutingInput{Triggers: []string{"run pdf", "merge two pdfs"}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	warned := false
	for _, warning := range edit.Warnings {
		warned = warned || (warning.Code == "open_draft" && strings.Contains(warning.Summary, "pdf is checked out in"))
	}
	if !warned {
		t.Fatalf("an edit on the hub must warn about the open draft: %+v", edit.Warnings)
	}

	replaceInHub(t, hub, "pdf", "line three", "line 3 from the hub")
	replaceInProject(t, folder, "line one", "line 1 from the draft")
	preview := previewCommit(t, hub, project, "pdf", SkillCommitInput{})
	if preview.Case != commitCaseMerge || len(preview.HubChanges) == 0 || !slices.Contains(preview.HubChanges[0].Files, "SKILL.md") {
		t.Fatalf("the draft's commit must name what changed on the hub meanwhile: case %s, %+v", preview.Case, preview.HubChanges)
	}
	for _, warning := range preview.Warnings {
		if warning.Code == "open_draft" {
			t.Fatalf("a draft's own commit must not warn about itself: %+v", warning)
		}
	}
}

func TestDiscardingADraftShowsWhatIsLostThenDeletesOnlyTheDraft(t *testing.T) {
	t.Parallel()
	hub, project, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	replaceInProject(t, folder, "line one", "line 1 never committed")
	writeProjectFile(t, folder, ".env", "TOKEN=abc\n", 0o600)
	writeProjectFile(t, project, "notes.md", "the project's own file\n", 0o644)
	service := DraftService{}

	list, err := service.List(context.Background(), hub)
	if err != nil || len(list.Drafts) != 1 || list.Drafts[0].State != "open" || list.Drafts[0].ChangedFiles != 1 {
		t.Fatalf("list = %+v %v", list.Drafts, err)
	}
	projectRoot := list.Drafts[0].ProjectRoot
	preview, err := service.PreviewDiscard(context.Background(), hub, "pdf", projectRoot)
	if err != nil || preview.Error != nil || !slices.Equal(preview.Unsaved, []string{"SKILL.md"}) || !slices.Equal(preview.LeftOut, []string{".env"}) {
		t.Fatalf("preview = %+v %v", preview, err)
	}

	writeProjectFile(t, folder, "late.md", "written after the preview\n", 0o644)
	if stale, err := service.ConfirmDiscard(context.Background(), hub, "pdf", projectRoot, preview.Digest); err != nil || stale.Error == nil {
		t.Fatalf("a folder that changed after the preview must not be deleted: %+v %v", stale, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "late.md")); err != nil {
		t.Fatalf("the refused discard deleted files: %v", err)
	}

	hubBefore := hubSkillMD(t, hub, "pdf")
	preview, err = service.PreviewDiscard(context.Background(), hub, "pdf", projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.ConfirmDiscard(context.Background(), hub, "pdf", projectRoot, preview.Digest); err != nil || result.Error != nil {
		t.Fatalf("discard = %+v %v", result, err)
	}
	for _, gone := range []string{folder, filepath.Join(project, ".claude", "skills", "pdf")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived the discard: %v", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(project, "notes.md")); err != nil {
		t.Fatalf("the discard touched the project's own files: %v", err)
	}
	if hubSkillMD(t, hub, "pdf") != hubBefore || len(draftsIn(t, hub)) != 0 {
		t.Fatalf("the discard changed the hub or kept the record")
	}
}

func TestARecordWhoseFolderIsGoneIsListedAndRemoved(t *testing.T) {
	t.Parallel()
	hub, _, folder := checkedOut(t, fixtureSkill{id: "pdf"})
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	service := DraftService{}
	list, err := service.List(context.Background(), hub)
	if err != nil || len(list.Drafts) != 1 || list.Drafts[0].State != "folder_gone" {
		t.Fatalf("list = %+v %v", list.Drafts, err)
	}
	preview, err := service.PreviewDiscard(context.Background(), hub, "pdf", list.Drafts[0].ProjectRoot)
	if err != nil || preview.Error != nil {
		t.Fatalf("preview = %+v %v", preview, err)
	}
	if result, err := service.ConfirmDiscard(context.Background(), hub, "pdf", list.Drafts[0].ProjectRoot, preview.Digest); err != nil || result.Error != nil || len(draftsIn(t, hub)) != 0 {
		t.Fatalf("removing the stale record = %+v %v", result, err)
	}
}

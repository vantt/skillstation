package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vantt/skillstation/internal/canonical"
	"github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/distill"
	"github.com/vantt/skillstation/internal/skill"
	sourcepkg "github.com/vantt/skillstation/internal/source"
	"gopkg.in/yaml.v3"
)

func TestSourceLinks(t *testing.T) {
	root := newSourceWorkspace(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	// Create test skill
	skillService := SkillService{}
	createPrev, err := skillService.PreviewCreate(ctx, root, skill.CreateInput{
		ID:          "test-skill",
		Collection:  "default",
		Name:        "Test Skill",
		Description: "A skill for testing links.",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = skillService.ConfirmSkillMutation(ctx, root, createPrev, createPrev.Confirmation.Confirmation.Pins)
	if err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(root, "skills", "default", "test-skill", ".meta", "hub.yaml")
	metaBytesBefore, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}

	adapter := &fakeImportAdapter{
		fakeSourceAdapter: fakeSourceAdapter{
			revisions: map[string]sourcepkg.Revision{
				"skills":       revision("one"),
				"custom-watch": revision("one"),
			},
		},
	}
	service := SourceService{
		Clock:    sourceClock{now: now},
		Adapters: map[string]sourcepkg.Adapter{"git": adapter},
	}

	// 1. Attach by locator creates source + link and leaves skill's meta byte-identical
	attachPrev, err := service.PreviewAttach(ctx, root, SourceAttachInput{
		SkillID: "test-skill",
		Locator: testGitHubURL,
	})
	if err != nil || attachPrev.Error != nil {
		t.Fatalf("attach preview failed: %v, %#v", err, attachPrev.Error)
	}
	if attachPrev.WriteCommand() != "source_attach" {
		t.Fatalf("expected WriteCommand 'source_attach', got %q", attachPrev.WriteCommand())
	}
	sourceID := attachPrev.Source.ID

	confirmRes, err := service.ConfirmSourceProposal(ctx, root, attachPrev, attachPrev.Confirmation.Confirmation.Pins)
	if err != nil || confirmRes.Error != nil {
		t.Fatalf("confirm attach failed: %v, %#v", err, confirmRes.Error)
	}

	// Source record created; the skill, which had no distill.yaml, gets one
	// holding only its sources.
	sourceRecordPath := filepath.Join(root, "sources", "catalog", sourceID+".yaml")
	if _, err := os.Stat(sourceRecordPath); err != nil {
		t.Fatalf("source record missing: %v", err)
	}
	linkPath := filepath.Join(root, "skills", "default", "test-skill", ".meta", "distill.yaml")
	if got, err := os.ReadFile(linkPath); err != nil || string(got) != "sources:\n  - id: "+sourceID+"\n    repository: "+testGitHubURL+"\n    ref: main\n" {
		t.Fatalf("distill.yaml after attach = %q, %v", got, err)
	}

	// Skill meta byte-identical (no source_id)
	metaBytesAfter, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(metaBytesBefore, metaBytesAfter) {
		t.Fatalf("skill meta was modified by attach")
	}

	// canonical.Validate returns no issues
	issues, err := canonical.Validate(root)
	if err != nil || len(issues) != 0 {
		t.Fatalf("canonical validate issues after attach: %v, %v", err, issues)
	}

	// 2. Attach again -> "already linked"
	attachAgain, err := service.PreviewAttach(ctx, root, SourceAttachInput{
		SkillID: "test-skill",
		Locator: testGitHubURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if attachAgain.Status != StatusOK || attachAgain.Confirmation.Confirmation.Pins.ProposalID != "" {
		t.Fatalf("expected already linked StatusOK without pins, got %#v", attachAgain)
	}

	// 3. Detach when an observation references the source keeps it with monitoring off and warning source_kept_referenced
	// Update source record to have DistilledRevision so it can be prepared
	fromRev := revision("one")
	toRev := revision("two")
	adapter.revisions[sourceID] = toRev
	recData, _ := os.ReadFile(sourceRecordPath)
	var recObj sourcepkg.Record
	_ = yaml.Unmarshal(recData, &recObj)
	recObj.DistilledRevision = &fromRev
	recObj.CurrentRevision = &toRev
	recObj.Status = "changed"
	recBytes, _ := sourcepkg.MarshalCanonical(recObj)
	_ = os.WriteFile(sourceRecordPath, recBytes, 0o644)
	commitWorkspace(t, root)

	skillContent := []byte("# Target\nTest skill content.\n")
	adapter.files = map[string][]byte{
		"SKILL.md": skillContent,
	}
	adapter.resources = []sourcepkg.Resource{
		{Path: "SKILL.md", Size: int64(len(skillContent))},
	}

	runDir := filepath.Join(root, "distill", "sources", sourceID, "runs")
	_ = os.MkdirAll(runDir, 0o755)
	validRun := `schema_version: 1
id: RUN-01
source_id: ` + sourceID + `
state: prepared
package_digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
prepared_at: "2026-10-09T08:00:00Z"
attempt: 0
to_revision:
  kind: git-commit
  value: abc
  content_digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
  observed_at: "2026-10-09T08:00:00Z"
finding_ids:
  - OBS-` + sourceID + `--feature
`
	_ = os.WriteFile(filepath.Join(runDir, "RUN-01.yaml"), []byte(validRun), 0o644)
	obsDir := filepath.Join(root, "distill", "sources", sourceID, "observations")
	_ = os.MkdirAll(obsDir, 0o755)
	validObs := `schema_version: 1
id: OBS-` + sourceID + `--feature
source_id: ` + sourceID + `
run_id: RUN-01
stable_key: feature
status: active
what: Feature
vocabulary: [feature]
first_seen:
  kind: git-commit
  value: abc
  content_digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
last_seen:
  kind: git-commit
  value: abc
  content_digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
evidence:
  - revision:
      kind: git-commit
      value: abc
      content_digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    run_id: RUN-01
    package_digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
    path: SKILL.md
    locator: SKILL.md
    digest: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
`
	_ = os.WriteFile(filepath.Join(obsDir, "OBS-"+sourceID+"--feature.yaml"), []byte(validObs), 0o644)
	commitWorkspace(t, root)
	_, _ = catalog.BuildCatalogGeneration(ctx, root, catalog.BuildOptions{})

	detachPrevRef, err := service.PreviewDetach(ctx, root, "test-skill", sourceID)
	if err != nil || detachPrevRef.Error != nil {
		t.Fatalf("preview detach referenced failed: %v, %#v", err, detachPrevRef.Error)
	}
	hasWarning := false
	for _, w := range detachPrevRef.Warnings {
		if w.Code == "source_kept_referenced" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Fatalf("expected warning source_kept_referenced, got: %#v", detachPrevRef.Warnings)
	}

	confirmDetachRef, err := service.ConfirmSourceProposal(ctx, root, detachPrevRef, detachPrevRef.Confirmation.Confirmation.Pins)
	if err != nil || confirmDetachRef.Error != nil {
		t.Fatalf("confirm detach referenced failed: %v, %#v", err, confirmDetachRef.Error)
	}
	// Source record is kept on disk with monitoring disabled
	srcData, err := os.ReadFile(sourceRecordPath)
	if err != nil {
		t.Fatalf("source record should have been kept: %v", err)
	}
	rec, err := sourcepkg.ParseRecord(srcData)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Monitoring.Enabled || rec.Monitoring.Cadence != "manual" {
		t.Fatalf("expected monitoring disabled, got %#v", rec.Monitoring)
	}
	// The distill.yaml held nothing else, so it is removed
	if _, err := os.Stat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("the emptied distill.yaml should have been removed")
	}

	issues, err = canonical.Validate(root)
	if err != nil || len(issues) != 0 {
		t.Fatalf("canonical validate issues after detach referenced: %v, %v", err, issues)
	}

	// 4. Detach of the only link deletes the source (for a source without external references)
	// Create another source and link
	secAttach, err := service.PreviewAttach(ctx, root, SourceAttachInput{
		SkillID:  "test-skill",
		SourceID: sourceID,
	})
	if err != nil || secAttach.Error != nil {
		t.Fatalf("re-attach failed: %v, %#v", err, secAttach.Error)
	}
	_, err = service.ConfirmSourceProposal(ctx, root, secAttach, secAttach.Confirmation.Confirmation.Pins)
	if err != nil {
		t.Fatal(err)
	}
	// Remove observation reference so source is completely unreferenced outside the link
	_ = os.RemoveAll(filepath.Join(root, "distill"))
	commitWorkspace(t, root)
	_, _ = catalog.BuildCatalogGeneration(ctx, root, catalog.BuildOptions{})

	detachDeletePrev, err := service.PreviewDetach(ctx, root, "test-skill", sourceID)
	if err != nil || detachDeletePrev.Error != nil {
		t.Fatalf("preview detach delete failed: %v, %#v", err, detachDeletePrev.Error)
	}
	_, err = service.ConfirmSourceProposal(ctx, root, detachDeletePrev, detachDeletePrev.Confirmation.Confirmation.Pins)
	if err != nil {
		t.Fatal(err)
	}
	// Both the source and the skill's sources are removed
	if _, err := os.Stat(sourceRecordPath); !os.IsNotExist(err) {
		t.Fatalf("expected source record deleted on disk")
	}
	if _, err := os.Stat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("expected the emptied distill.yaml deleted on disk")
	}

	issues, err = canonical.Validate(root)
	if err != nil || len(issues) != 0 {
		t.Fatalf("canonical validate issues after delete detach: %v, %v", err, issues)
	}

	// 5. Unwatch of a source with upstream skills turns monitoring off and keeps the record
	// Add an upstream tracked skill
	addService := SkillAddService{
		Clock:    sourceClock{now: now},
		Adapters: map[string]sourcepkg.Adapter{"git": adapter},
	}
	// Create an upstream source manually for unwatch test
	upSourceRec := sourcepkg.Record{
		SchemaVersion: 1,
		ID:            "src-with-upstream-skill",
		Purpose:       "upstream",
		Adapter:       "git",
		Locator:       sourcepkg.Locator{Repository: "https://github.com/example/upstream.git", Ref: "main"},
		Status:        "watching",
		Identity:      sourcepkg.Identity{Name: "upstream", Canonical: "https://github.com/example/upstream.git"},
		Limits:        sourcepkg.Limits{TimeoutSeconds: 20, MaxBytes: sourcepkg.DefaultMaxBytes, MaxFiles: sourcepkg.DefaultMaxFiles, MaxFileBytes: sourcepkg.DefaultMaxFileSize},
		Monitoring:    sourcepkg.Monitoring{Enabled: true, Cadence: "weekly"},
	}
	upBytes, _ := sourcepkg.MarshalCanonical(upSourceRec)
	_ = os.WriteFile(filepath.Join(root, "sources", "catalog", upSourceRec.ID+".yaml"), upBytes, 0o644)

	// Link test-skill to this source via provenance.source_id
	metaData, _ := os.ReadFile(metaPath)
	var doc map[string]any
	if err := yaml.Unmarshal(metaData, &doc); err != nil {
		t.Fatal(err)
	}
	prov, ok := doc["provenance"].(map[string]any)
	if !ok || prov == nil {
		prov = make(map[string]any)
	}
	prov["source_id"] = "src-with-upstream-skill"
	doc["provenance"] = prov
	newMeta, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(metaPath, newMeta, 0o644)
	commitWorkspace(t, root)
	_, _ = catalog.BuildCatalogGeneration(ctx, root, catalog.BuildOptions{})

	unwatchPrev, err := service.PreviewUnwatch(ctx, root, upSourceRec.ID)
	if err != nil || unwatchPrev.Error != nil {
		t.Fatalf("preview unwatch failed: %v, %#v", err, unwatchPrev.Error)
	}
	unwatchRes, err := service.ConfirmSourceProposal(ctx, root, unwatchPrev, unwatchPrev.Confirmation.Confirmation.Pins)
	if err != nil || unwatchRes.Error != nil {
		t.Fatalf("confirm unwatch failed: %v, %#v", err, unwatchRes.Error)
	}
	if unwatchRes.Summary != "Stopped watching src-with-upstream-skill." {
		t.Fatalf("expected summary 'Stopped watching src-with-upstream-skill.', got %q", unwatchRes.Summary)
	}

	// Record kept on disk with monitoring disabled
	upDataAfter, err := os.ReadFile(filepath.Join(root, "sources", "catalog", upSourceRec.ID+".yaml"))
	if err != nil {
		t.Fatalf("upstream source record should be kept: %v", err)
	}
	recAfter, _ := sourcepkg.ParseRecord(upDataAfter)
	if recAfter.Monitoring.Enabled || recAfter.Monitoring.Cadence != "manual" {
		t.Fatalf("expected monitoring disabled, got %#v", recAfter.Monitoring)
	}

	issues, err = canonical.Validate(root)
	if err != nil || len(issues) != 0 {
		t.Fatalf("canonical validate issues after unwatch: %v, %v", err, issues)
	}
	_ = addService
}

// The hub edits only the sources block of a distill.yaml: the lessons the
// skill wrote stay byte for byte, and a block the hub cannot read is refused
// instead of overwritten.
func TestSourceAttachAndDetachKeepTheDistilledKnowledge(t *testing.T) {
	t.Parallel()
	root := newSourceWorkspace(t)
	ctx := context.Background()
	created, err := (SkillService{}).PreviewCreate(ctx, root, skill.CreateInput{ID: "learner", Collection: "default", Name: "Learner", Description: "Learns."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (SkillService{}).ConfirmSkillMutation(ctx, root, created, created.Confirmation.Confirmation.Pins); err != nil {
		t.Fatal(err)
	}
	record := sourcepkg.Record{SchemaVersion: 1, ID: "docs", Adapter: "git", Status: "watching",
		Locator:    sourcepkg.Locator{Repository: "https://github.com/example/docs", Ref: "main", Path: "guides"},
		Identity:   sourcepkg.Identity{Name: "docs", Canonical: "https://github.com/example/docs", DefaultBranch: "main"},
		Monitoring: sourcepkg.Monitoring{Enabled: true, Cadence: "weekly"},
		Limits:     sourcepkg.Limits{TimeoutSeconds: 30, MaxBytes: 1 << 20, MaxFiles: 100, MaxFileBytes: 1 << 20}}
	recordBytes, _ := sourcepkg.MarshalCanonical(record)
	if err := os.WriteFile(filepath.Join(root, "sources", "catalog", "docs.yaml"), recordBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	distilled, err := os.ReadFile(filepath.Join("..", "migration", "testdata", "phase0-distill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	distillPath := filepath.Join(root, "skills", "default", "learner", ".meta", "distill.yaml")
	if err := os.WriteFile(distillPath, distilled, 0o644); err != nil {
		t.Fatal(err)
	}
	commitWorkspace(t, root)
	service := SourceService{Clock: sourceClock{now: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}}

	attach, err := service.PreviewAttach(ctx, root, SourceAttachInput{SkillID: "learner", SourceID: "docs"})
	if err != nil || attach.Error != nil {
		t.Fatalf("attach preview: %v, %#v", err, attach.Error)
	}
	if want := "skills/default/learner/.meta/distill.yaml"; len(attach.Diff.Modified) != 1 || attach.Diff.Modified[0] != want || len(attach.Diff.Added) != 0 {
		t.Fatalf("attach diff = %#v", attach.Diff)
	}
	if _, err := service.ConfirmSourceProposal(ctx, root, attach, attach.Confirmation.Confirmation.Pins); err != nil {
		t.Fatal(err)
	}
	withSources, _ := os.ReadFile(distillPath)
	doc, err := distill.UnmarshalDocument(withSources)
	if err != nil {
		t.Fatalf("distill.yaml after attach does not validate: %v", err)
	}
	if want := []distill.Source{{ID: "docs", Repository: "https://github.com/example/docs", Ref: "main", Path: "guides"}}; !reflect.DeepEqual(doc.Sources, want) || len(doc.Lessons) != 42 {
		t.Fatalf("after attach: sources %#v, %d lessons", doc.Sources, len(doc.Lessons))
	}
	commitWorkspace(t, root)

	detach, err := service.PreviewDetach(ctx, root, "learner", "docs")
	if err != nil || detach.Error != nil {
		t.Fatalf("detach preview: %v, %#v", err, detach.Error)
	}
	if _, err := service.ConfirmSourceProposal(ctx, root, detach, detach.Confirmation.Confirmation.Pins); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(distillPath); !bytes.Equal(after, distilled) {
		t.Fatal("detach did not give back the distill.yaml the skill wrote")
	}
	commitWorkspace(t, root)

	unreadable := append([]byte("sources: docs\n"), distilled...)
	if err := os.WriteFile(distillPath, unreadable, 0o644); err != nil {
		t.Fatal(err)
	}
	refused, err := service.PreviewAttach(ctx, root, SourceAttachInput{SkillID: "learner", SourceID: "docs"})
	if err != nil || refused.Error == nil || refused.Error.Code != ErrorInvalidRequest {
		t.Fatalf("attach to an unreadable sources block = %#v, %v", refused, err)
	}
	if after, _ := os.ReadFile(distillPath); !bytes.Equal(after, unreadable) {
		t.Fatal("a refused attach changed distill.yaml")
	}
}

// Doctor reports a sources block the hub cannot read, an entry whose address
// differs from the hub's record (the record wins) and a source the hub does
// not watch.
func TestDoctorReportsSkillSourcesThatDisagreeWithTheHub(t *testing.T) {
	t.Parallel()
	root := newSourceWorkspace(t)
	record := sourcepkg.Record{SchemaVersion: 1, ID: "docs", Adapter: "git", Status: "watching",
		Locator:    sourcepkg.Locator{Repository: "https://github.com/example/docs", Ref: "main"},
		Identity:   sourcepkg.Identity{Name: "docs", Canonical: "https://github.com/example/docs", DefaultBranch: "main"},
		Monitoring: sourcepkg.Monitoring{Enabled: true, Cadence: "weekly"},
		Limits:     sourcepkg.Limits{TimeoutSeconds: 30, MaxBytes: 1 << 20, MaxFiles: 100, MaxFileBytes: 1 << 20}}
	recordBytes, _ := sourcepkg.MarshalCanonical(record)
	if err := os.WriteFile(filepath.Join(root, "sources", "catalog", "docs.yaml"), recordBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := sourceLinkFindings(root, RoleReport{}); len(got) != 1 || got[0].Severity != SeverityOK {
		t.Fatalf("a hub without sources blocks: %#v", got)
	}
	write := func(skillID, contents string) {
		file := filepath.Join(root, "skills", "default", skillID, ".meta", "distill.yaml")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("matching", "sources:\n  - id: docs\n    repository: https://github.com/example/docs\n    ref: main\n")
	write("moved", "sources:\n  - id: docs\n    repository: https://github.com/example/old-docs\n")
	write("standalone", "sources:\n  - id: notes\n    repository: https://github.com/example/notes\n    ref: v2\n")
	write("broken", "sources: docs\n")

	got := map[string]Finding{}
	for _, finding := range sourceLinkFindings(root, RoleReport{}) {
		got[finding.Code] = finding
	}
	if len(got) != 3 {
		t.Fatalf("findings = %#v", got)
	}
	if f := got["hub_skill_sources_unreadable"]; f.Severity != SeverityProblem || !strings.HasPrefix(f.Message, "skills/default/broken/.meta/distill.yaml: ") {
		t.Errorf("unreadable block: %#v", f)
	}
	if f := got["hub_skill_source_address"]; f.Severity != SeverityNotice || !strings.Contains(f.Message, "moved names source docs as https://github.com/example/old-docs, but the hub's record sources/catalog/docs.yaml says https://github.com/example/docs ref main; the record is used.") {
		t.Errorf("address differs: %#v", f)
	}
	if f := got["hub_skill_source_unwatched"]; f.Severity != SeverityNotice || f.Manual != "skillstation source watch https://github.com/example/notes --skill-id standalone --id notes --ref v2" {
		t.Errorf("unwatched source: %#v", f)
	}
}

// Watching a source that a skill's block already names, without a hub record
// (the skill added it outside a hub), creates the record and takes its
// address into the block.
func TestSourceWatchAdoptsAnEntryTheSkillWrote(t *testing.T) {
	t.Parallel()
	root := newSourceWorkspace(t)
	ctx := context.Background()
	created, err := (SkillService{}).PreviewCreate(ctx, root, skill.CreateInput{ID: "adopter", Collection: "default", Name: "Adopter", Description: "Learns."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (SkillService{}).ConfirmSkillMutation(ctx, root, created, created.Confirmation.Confirmation.Pins); err != nil {
		t.Fatal(err)
	}
	distillPath := filepath.Join(root, "skills", "default", "adopter", ".meta", "distill.yaml")
	if err := os.WriteFile(distillPath, []byte("sources:\n  - id: custom-watch\n    repository: https://github.com/example/old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitWorkspace(t, root)
	adapter := &fakeImportAdapter{fakeSourceAdapter: fakeSourceAdapter{revisions: map[string]sourcepkg.Revision{"custom-watch": revision("one")}}}
	service := SourceService{Clock: sourceClock{now: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}, Adapters: map[string]sourcepkg.Adapter{"git": adapter}}

	preview, err := service.PreviewSourceWatch(ctx, root, SourceWatchInput{Locator: testGitHubURL, SkillID: "adopter", SourceID: "custom-watch"})
	if err != nil || preview.Error != nil || preview.Status != StatusActionRequired {
		t.Fatalf("watch preview = %#v, %v", preview, err)
	}
	if _, err := service.ConfirmSourceProposal(ctx, root, preview, preview.Confirmation.Confirmation.Pins); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(distillPath)
	if got, err := distill.ReadSources(data); err != nil || len(got) != 1 || got[0].Repository != testGitHubURL {
		t.Fatalf("sources after watch = %#v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "sources", "catalog", "custom-watch.yaml")); err != nil {
		t.Fatalf("no record was created: %v", err)
	}
}

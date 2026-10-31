package mutation

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

const datedSkillDir = "skills/core/dated"

func setMetadataClock(t *testing.T, at string) {
	t.Helper()
	now, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatal(err)
	}
	previous := metadataClock
	metadataClock = func() time.Time { return now }
	t.Cleanup(func() { metadataClock = previous })
}

func planAndConfirm(t *testing.T, root string, changes ...Change) Proposal {
	t.Helper()
	proposal, err := PlanMutation(root, WriteSet{Command: "skill_edit", Changes: changes})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ConfirmMutationWithOptions(root, proposal, Confirmation{ProposalID: proposal.ID, ProposalDigest: proposal.Digest, BaseCatalogSnapshot: proposal.BaseCatalogSnapshot}, Options{}); err != nil {
		t.Fatal(err)
	}
	return proposal
}

func hubDates(t *testing.T, root string) (string, string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(datedSkillDir+"/"+skillmeta.HubFile)))
	if err != nil {
		t.Fatal(err)
	}
	var dates struct {
		CreatedAt string `yaml:"created_at"`
		UpdatedAt string `yaml:"updated_at"`
	}
	if err := yaml.Unmarshal(contents, &dates); err != nil {
		t.Fatal(err)
	}
	return dates.CreatedAt, dates.UpdatedAt
}

// newDatedSkill adds a skill on a current-layout hub at the given time.
func newDatedSkill(t *testing.T, at string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	setMetadataClock(t, at)
	planAndConfirm(t, root,
		Change{Path: datedSkillDir + "/SKILL.md", Contents: []byte("---\nname: dated\ndescription: Dated skill.\n---\n# Dated\n")},
		Change{Path: datedSkillDir + "/" + skillmeta.HubFile, Contents: []byte("schema_version: 1\nstatus: draft\n")},
		Change{Path: datedSkillDir + "/" + skillmeta.SettingsFile, Contents: []byte("schema_version: 1\nlabels: [dated]\n")},
	)
	return root
}

func TestNewSkillGetsCreatedAndUpdatedDates(t *testing.T) {
	root := newDatedSkill(t, "2026-10-01T10:00:00Z")
	created, updated := hubDates(t, root)
	if created != "2026-10-01T10:00:00Z" || updated != "2026-10-01T10:00:00Z" {
		t.Fatalf("dates = %q, %q", created, updated)
	}
}

func TestSkillEditKeepsCreatedAtAndSetsUpdatedAt(t *testing.T) {
	root := newDatedSkill(t, "2026-10-01T10:00:00Z")
	setMetadataClock(t, "2026-10-05T12:30:00Z")
	planAndConfirm(t, root, Change{Path: datedSkillDir + "/SKILL.md", Contents: []byte("---\nname: dated\ndescription: Dated skill.\n---\n# Dated, edited\n")})
	created, updated := hubDates(t, root)
	if created != "2026-10-01T10:00:00Z" || updated != "2026-10-05T12:30:00Z" {
		t.Fatalf("dates after a content edit = %q, %q", created, updated)
	}

	// A hub.yaml written with other dates keeps the stored created_at.
	setMetadataClock(t, "2026-10-06T08:00:00Z")
	planAndConfirm(t, root, Change{Path: datedSkillDir + "/" + skillmeta.HubFile, Contents: []byte("schema_version: 1\nstatus: draft\ncreated_at: \"2020-01-01T00:00:00Z\"\nupdated_at: \"2020-01-01T00:00:00Z\"\n")})
	created, updated = hubDates(t, root)
	if created != "2026-10-01T10:00:00Z" || updated != "2026-10-06T08:00:00Z" {
		t.Fatalf("dates after a hub.yaml write = %q, %q", created, updated)
	}
}

func TestDistillWriteChangesNoSkillDate(t *testing.T) {
	root := newDatedSkill(t, "2026-10-01T10:00:00Z")
	hubPath := filepath.Join(root, filepath.FromSlash(datedSkillDir+"/"+skillmeta.HubFile))
	before, err := os.ReadFile(hubPath)
	if err != nil {
		t.Fatal(err)
	}
	setMetadataClock(t, "2026-10-09T09:00:00Z")
	proposal := planAndConfirm(t, root, Change{Path: datedSkillDir + "/.meta/distill.yaml", Contents: []byte("goal: learn something\nlessons: []\n")})
	for _, change := range proposal.WriteSet.Changes {
		if change.Path != datedSkillDir+"/.meta/distill.yaml" {
			t.Fatalf("a distill-only write also changed %s", change.Path)
		}
	}
	after, err := os.ReadFile(hubPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("hub.yaml changed by a distill write:\n%s\n->\n%s", before, after)
	}
}

func TestOldPathMetadataChangeBecomesTheTwoFiles(t *testing.T) {
	root := newDatedSkill(t, "2026-10-01T10:00:00Z")
	setMetadataClock(t, "2026-10-07T07:00:00Z")
	merged := "schema_version: 1\nid: dated\nname: dated\ndescription: Dated skill.\nstatus: draft\nlabels: [dated, edited]\nrouting:\n  triggers: [date a skill]\n"
	proposal := planAndConfirm(t, root, Change{Path: datedSkillDir + "/" + skillmeta.LegacyFile, Contents: []byte(merged)})

	paths := map[string]bool{}
	for _, change := range proposal.WriteSet.Changes {
		paths[change.Path] = true
	}
	if paths[datedSkillDir+"/"+skillmeta.LegacyFile] || !paths[datedSkillDir+"/"+skillmeta.HubFile] || !paths[datedSkillDir+"/"+skillmeta.SettingsFile] {
		t.Fatalf("changes = %v, want skill.yaml and .meta/hub.yaml only", paths)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(datedSkillDir+"/"+skillmeta.LegacyFile))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".meta/skill.yaml was written: %v", err)
	}
	settings, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(datedSkillDir+"/"+skillmeta.SettingsFile)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), "edited") || !strings.Contains(string(settings), "date a skill") || strings.Contains(string(settings), "description") {
		t.Fatalf("skill.yaml = %s", settings)
	}
	created, updated := hubDates(t, root)
	if created != "2026-10-01T10:00:00Z" || updated != "2026-10-07T07:00:00Z" {
		t.Fatalf("dates = %q, %q", created, updated)
	}
}

func TestOldPathMetadataChangeChecksItsPinnedDigest(t *testing.T) {
	root := newDatedSkill(t, "2026-10-01T10:00:00Z")
	current, _, found, err := skillmeta.Read(skillmeta.DirReader(root), datedSkillDir)
	if err != nil || !found {
		t.Fatalf("read metadata: found=%v err=%v", found, err)
	}
	merged := []byte("schema_version: 1\nid: dated\nstatus: draft\nlabels: [pinned]\n")

	stale := "sha256:" + strings.Repeat("0", 64)
	_, err = PlanMutation(root, WriteSet{Command: "skill_edit", Changes: []Change{{Path: datedSkillDir + "/" + skillmeta.LegacyFile, Contents: merged, BeforeDigest: stale}}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale pinned digest error = %v, want ErrConflict", err)
	}

	if _, err := PlanMutation(root, WriteSet{Command: "skill_edit", Changes: []Change{{Path: datedSkillDir + "/" + skillmeta.LegacyFile, Contents: merged, BeforeDigest: digest(current)}}}); err != nil {
		t.Fatalf("the digest of the current merged metadata was refused: %v", err)
	}
}

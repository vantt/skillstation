package migration

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

const v5FullSkillMetadata = `schema_version: 1
id: full-skill
name: full-skill
description: Review full changes.
collection: core
collection_id: core
status: active
aliases: [full-review]
domain: [software]
topics: [review]
technologies: [go]
labels: [favorite]
content:
  kind: procedure
runtime:
  requires:
    bins: [git]
# How the skill is found.
routing:
  triggers: [review a full change]
  not_for: [write prose]
  min_scope: multi_step
quality:
  reviewed: true
  routing_review_rationale: Reviewed by a curator.
sources:
  - id: upstream
    roles: [upstream]
    repository: https://github.com/example/full
    commit: 1111111111111111111111111111111111111111
provenance:
  origin:
    kind: local
    path: skills/core/full-skill
history:
  - state: active
    occurred_at: "2026-10-01T00:00:00Z"
created_at: "2026-10-01T00:00:00Z"
updated_at: "2026-10-02T00:00:00Z"
`

const v6DistillYAML = "goal: learn something\nlessons: []\n# kept as written\n"

func v5Workspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".skillstation/schema-version", "5\n")
	return root
}

func writeFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(contents, &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}

func TestMigrationV5ToV6MovesEveryFieldIntoItsFile(t *testing.T) {
	root := v5Workspace(t)
	skillDir := "skills/core/full-skill"
	writeFile(t, root, skillDir+"/SKILL.md", "---\nname: full-skill\ndescription: Review full changes.\n---\n# Full\n")
	writeFile(t, root, skillDir+"/.meta/skill.yaml", v5FullSkillMetadata)
	writeFile(t, root, skillDir+"/.meta/distill.yaml", v6DistillYAML)

	proposal, err := DefaultRegistry().Preview(root, CurrentVersion)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.SourceVersion != 5 || proposal.TargetVersion != CurrentVersion {
		t.Fatalf("proposal versions = %d -> %d", proposal.SourceVersion, proposal.TargetVersion)
	}
	if _, err := mutation.ConfirmMutationWithOptions(root, proposal.Mutation, mutation.Confirmation{ProposalID: proposal.ID, ProposalDigest: proposal.Digest, BaseCatalogSnapshot: proposal.BaseSnapshot}, mutation.Options{}); err != nil {
		t.Fatal(err)
	}
	if version, err := DetectVersion(root); err != nil || version != CurrentVersion {
		t.Fatalf("version = %d, %v", version, err)
	}
	dir := filepath.Join(root, filepath.FromSlash(skillDir))
	if _, err := os.Stat(filepath.Join(dir, ".meta", "skill.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".meta/skill.yaml was not removed: %v", err)
	}

	settings := readYAMLMap(t, filepath.Join(dir, "skill.yaml"))
	hub := readYAMLMap(t, filepath.Join(dir, ".meta", "hub.yaml"))
	for _, key := range []string{"routing", "topics", "technologies", "domain", "aliases", "runtime", "labels"} {
		if _, ok := settings[key]; !ok {
			t.Errorf("skill.yaml lacks %s", key)
		}
		if _, ok := hub[key]; ok {
			t.Errorf("hub.yaml holds %s", key)
		}
	}
	for _, key := range []string{"status", "quality", "sources", "provenance", "history", "created_at", "updated_at", "content"} {
		if _, ok := hub[key]; !ok {
			t.Errorf("hub.yaml lacks %s", key)
		}
		if _, ok := settings[key]; ok {
			t.Errorf("skill.yaml holds %s", key)
		}
	}
	for _, key := range []string{"id", "collection", "collection_id", "name", "description"} {
		if _, ok := settings[key]; ok {
			t.Errorf("skill.yaml keeps derived key %s", key)
		}
		if _, ok := hub[key]; ok {
			t.Errorf("hub.yaml keeps derived key %s", key)
		}
	}
	for name, values := range map[string]map[string]any{"skill.yaml": settings, "hub.yaml": hub} {
		if values["schema_version"] != 1 {
			t.Errorf("%s schema_version = %v", name, values["schema_version"])
		}
	}
	if hub["created_at"] != "2026-10-01T00:00:00Z" {
		t.Errorf("created_at = %v, want the stored date", hub["created_at"])
	}
	settingsText, err := os.ReadFile(filepath.Join(dir, "skill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settingsText), "# How the skill is found.") {
		t.Errorf("comment on routing was lost:\n%s", settingsText)
	}
	distill, err := os.ReadFile(filepath.Join(dir, ".meta", "distill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(distill, []byte(v6DistillYAML)) {
		t.Fatalf("distill.yaml changed:\n%s", distill)
	}
	if _, err := DefaultRegistry().Preview(root, CurrentVersion); !errors.Is(err, ErrAlreadyCurrent) {
		t.Fatalf("second preview error = %v", err)
	}
}

func TestMigrationV5ToV6StopsWhenAValueWouldBeLost(t *testing.T) {
	const skillMD = "---\nname: lossy\ndescription: Kept description.\n---\n# Lossy\n"
	const base = "schema_version: 1\nid: lossy\nname: lossy\ndescription: Kept description.\ncollection: core\nstatus: draft\n"
	cases := map[string]struct {
		metadata string
		extra    map[string]string
		field    string
	}{
		"name":        {metadata: strings.Replace(base, "name: lossy\n", "name: Lossy Display\n", 1), field: "name"},
		"description": {metadata: strings.Replace(base, "description: Kept description.\n", "description: Another description.\n", 1), field: "description"},
		"id":          {metadata: strings.Replace(base, "id: lossy\n", "id: other-id\n", 1), field: "id"},
		"collection":  {metadata: strings.Replace(base, "collection: core\n", "collection: other\n", 1), field: "collection"},
		"skill.yaml":  {metadata: base, extra: map[string]string{"skills/core/lossy/skill.yaml": "shipped: by upstream\n"}, field: "skill.yaml"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := v5Workspace(t)
			writeFile(t, root, "skills/core/lossy/SKILL.md", skillMD)
			writeFile(t, root, "skills/core/lossy/.meta/skill.yaml", tc.metadata)
			for rel, contents := range tc.extra {
				writeFile(t, root, rel, contents)
			}

			_, err := DefaultRegistry().Preview(root, CurrentVersion)
			var disagreement *DisagreementError
			if !errors.As(err, &disagreement) || !errors.Is(err, ErrMetadataDisagrees) {
				t.Fatalf("preview error = %v, want a DisagreementError", err)
			}
			found := false
			for _, item := range disagreement.Items {
				if item.Skill == "lossy" && item.Field == tc.field {
					found = true
				}
			}
			if !found {
				t.Fatalf("disagreement does not name skill lossy and field %s: %#v", tc.field, disagreement.Items)
			}
			if !strings.Contains(err.Error(), "lossy") {
				t.Fatalf("error does not name the skill: %v", err)
			}

			if version, err := DetectVersion(root); err != nil || version != 5 {
				t.Fatalf("version = %d, %v", version, err)
			}
			stored, err := os.ReadFile(filepath.Join(root, "skills", "core", "lossy", ".meta", "skill.yaml"))
			if err != nil || string(stored) != tc.metadata {
				t.Fatalf(".meta/skill.yaml changed: %q, %v", stored, err)
			}
			if _, err := os.Stat(filepath.Join(root, "skills", "core", "lossy", ".meta", "hub.yaml")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("hub.yaml was written: %v", err)
			}
			for rel, contents := range tc.extra {
				if got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err != nil || string(got) != contents {
					t.Fatalf("%s changed: %q, %v", rel, got, err)
				}
			}
			if entries, err := os.ReadDir(filepath.Join(root, "history", "operations")); err != nil || len(entries) != 0 {
				t.Fatalf("a receipt was written: %#v, %v", entries, err)
			}
		})
	}
}

func TestMigrationFromLayout2SplitsSkillMetadataAtTheCurrentLayout(t *testing.T) {
	root := v2Workspace(t)
	skillDir := "skills/core/old-skill"
	writeFile(t, root, skillDir+"/SKILL.md", "---\nname: old-skill\ndescription: Old skill.\n---\n# Old\n")
	writeFile(t, root, skillDir+"/skill.meta.yaml", "schema_version: 1\nid: old-skill\nname: old-skill\ndescription: Old skill.\nstatus: active\nrouting:\n  triggers: [run old skill]\n  not_for: [none]\n  min_scope: single_step\nquality:\n  reviewed: true\n  routing_review_rationale: Reviewed.\n")

	proposal, err := DefaultRegistry().Preview(root, CurrentVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mutation.ConfirmMutationWithOptions(root, proposal.Mutation, mutation.Confirmation{ProposalID: proposal.ID, ProposalDigest: proposal.Digest, BaseCatalogSnapshot: proposal.BaseSnapshot}, mutation.Options{}); err != nil {
		t.Fatal(err)
	}
	merged, source, found, err := skillmeta.Read(skillmeta.DirReader(root), skillDir)
	if err != nil || !found || !source.Split() {
		t.Fatalf("metadata after migration: found=%v source=%#v err=%v", found, source, err)
	}
	if !strings.Contains(string(merged), "run old skill") {
		t.Fatalf("routing was lost:\n%s", merged)
	}
	for _, rel := range []string{skillDir + "/skill.meta.yaml", skillDir + "/.meta/skill.yaml"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s is left after migration: %v", rel, err)
		}
	}
}

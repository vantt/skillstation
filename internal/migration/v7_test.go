package migration

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/distill"
	"github.com/vantt/skillstation/internal/mutation"
	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
)

// v6Hub is a hub at layout 6 with skills alpha (no distill.yaml), beta (a
// distilled skill), gamma (its block already names src-a) and delta (no
// links), source records src-a and src-b, and four link files.
func v6Hub(t *testing.T) (root string, distilled []byte) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".skillstation/schema-version", "6\n")
	for _, id := range []string{"alpha", "beta", "gamma", "delta"} {
		dir := "skills/core/" + id
		writeFile(t, root, dir+"/SKILL.md", "---\nname: "+id+"\ndescription: Skill "+id+".\n---\n# "+id+"\n")
		writeFile(t, root, dir+"/.meta/hub.yaml", "schema_version: 1\nstatus: draft\n")
	}
	for id, repository := range map[string]string{"src-a": "https://github.com/example/a", "src-b": "https://github.com/example/b"} {
		record := sourcepkg.Record{SchemaVersion: 1, ID: id, Adapter: "git", Status: "watching",
			Locator:    sourcepkg.Locator{Repository: repository, Ref: "main", Path: "docs"},
			Identity:   sourcepkg.Identity{Name: id, Canonical: repository, DefaultBranch: "main"},
			Monitoring: sourcepkg.Monitoring{Enabled: true, Cadence: "weekly"},
			Limits:     sourcepkg.Limits{TimeoutSeconds: 30, MaxBytes: 1 << 20, MaxFiles: 100, MaxFileBytes: 1 << 20}}
		contents, err := sourcepkg.MarshalCanonical(record)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, "sources/catalog/"+id+".yaml", string(contents))
	}
	distilled, err := os.ReadFile(filepath.Join("testdata", "phase0-distill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "skills/core/beta/.meta/distill.yaml", string(distilled))
	writeFile(t, root, "skills/core/gamma/.meta/distill.yaml", "sources:\n  - id: src-a\n    repository: https://github.com/example/a-as-the-skill-wrote-it\n")
	writeFile(t, root, "skills/core/delta/.meta/distill.yaml", "# not distilled yet\n")
	for _, link := range [][2]string{{"alpha", "src-a"}, {"alpha", "src-b"}, {"beta", "src-a"}, {"gamma", "src-a"}} {
		writeLink(t, root, link[0], link[1], "learning-source")
	}
	return root, distilled
}

func writeLink(t *testing.T, root, skillID, sourceID, role string) {
	t.Helper()
	id := "LINK-" + skillID + "--" + sourceID
	writeFile(t, root, "sources/skills/"+id+".yaml", "schema_version: 1\nid: "+id+"\nskill_id: "+skillID+"\nsource_id: "+sourceID+"\nrole: "+role+"\n")
}

func confirmMigration(t *testing.T, root string) Proposal {
	t.Helper()
	proposal, err := DefaultRegistry().Preview(root, CurrentVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mutation.ConfirmMutationWithOptions(root, proposal.Mutation, mutation.Confirmation{ProposalID: proposal.ID, ProposalDigest: proposal.Digest, BaseCatalogSnapshot: proposal.BaseSnapshot}, mutation.Options{}); err != nil {
		t.Fatal(err)
	}
	return proposal
}

func readSources(t *testing.T, root, skillID string) []distill.Source {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, "skills", "core", skillID, ".meta", "distill.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sources, err := distill.ReadSources(contents)
	if err != nil {
		t.Fatal(err)
	}
	return sources
}

func TestMigrationV6ToV7MovesEveryLinkIntoItsSkillsDistillFile(t *testing.T) {
	t.Parallel()
	root, distilled := v6Hub(t)
	proposal := confirmMigration(t, root)
	if proposal.SourceVersion != 6 || proposal.TargetVersion != 7 {
		t.Fatalf("proposal versions = %d -> %d", proposal.SourceVersion, proposal.TargetVersion)
	}
	if links, _ := filepath.Glob(filepath.Join(root, "sources", "skills", "*")); len(links) != 0 {
		t.Fatalf("link files left: %v", links)
	}
	a := distill.Source{ID: "src-a", Repository: "https://github.com/example/a", Ref: "main", Path: "docs"}
	b := distill.Source{ID: "src-b", Repository: "https://github.com/example/b", Ref: "main", Path: "docs"}
	if got := readSources(t, root, "alpha"); !reflect.DeepEqual(got, []distill.Source{a, b}) {
		t.Errorf("alpha, which had no distill.yaml: %#v", got)
	}
	if got := readSources(t, root, "beta"); !reflect.DeepEqual(got, []distill.Source{a}) {
		t.Errorf("beta: %#v", got)
	}
	beta, _ := os.ReadFile(filepath.Join(root, "skills", "core", "beta", ".meta", "distill.yaml"))
	if without, _ := distill.WithSources(beta, nil); string(without) != string(distilled) {
		t.Error("the move changed beta's lessons")
	}
	// An entry the block already holds is kept as the skill wrote it.
	if got := readSources(t, root, "gamma"); len(got) != 1 || got[0].Repository != "https://github.com/example/a-as-the-skill-wrote-it" {
		t.Errorf("gamma: %#v", got)
	}
	if delta, _ := os.ReadFile(filepath.Join(root, "skills", "core", "delta", ".meta", "distill.yaml")); string(delta) != "# not distilled yet\n" {
		t.Errorf("a skill without links was changed: %q", delta)
	}
	for _, id := range []string{"src-a", "src-b"} {
		if _, err := os.Stat(filepath.Join(root, "sources", "catalog", id+".yaml")); err != nil {
			t.Errorf("source record %s was not kept: %v", id, err)
		}
	}
	if _, err := DefaultRegistry().Preview(root, CurrentVersion); !errors.Is(err, ErrAlreadyCurrent) {
		t.Fatalf("second preview error = %v", err)
	}
}

func TestMigrationV6ToV7LeavesAHubWithoutLinksAlone(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".skillstation/schema-version", "6\n")
	proposal := confirmMigration(t, root)
	if len(proposal.Changes) != 1 || proposal.Changes[0].Path != ".skillstation/schema-version" {
		t.Fatalf("changes = %#v", proposal.Changes)
	}
}

func TestMigrationV6ToV7StopsWhenALinkHasNoHomeInTheBlock(t *testing.T) {
	t.Parallel()
	root, _ := v6Hub(t)
	writeLink(t, root, "removed-skill", "src-a", "learning-source")
	writeLink(t, root, "alpha", "src-gone", "learning-source")
	writeLink(t, root, "beta", "src-b", "inspiration")
	writeFile(t, root, "sources/skills/LINK-gamma--src-b.yaml", "schema_version: 1\nid: LINK-gamma--src-b\nskill_id: gamma\nsource_id: src-b\nnote: keep me\n")

	_, err := DefaultRegistry().Preview(root, CurrentVersion)
	var linkErr *SourceLinkError
	if !errors.As(err, &linkErr) || !errors.Is(err, ErrSourceLinksUnmovable) {
		t.Fatalf("preview error = %v", err)
	}
	want := map[string]string{
		"sources/skills/LINK-removed-skill--src-a.yaml": `names skill "removed-skill", which the hub no longer has.`,
		"sources/skills/LINK-alpha--src-gone.yaml":      `names source "src-gone", which has no record in sources/catalog/src-gone.yaml`,
		"sources/skills/LINK-beta--src-b.yaml":          `has role "inspiration"`,
		"sources/skills/LINK-gamma--src-b.yaml":         `field note not found`,
	}
	if len(linkErr.Items) != len(want) {
		t.Fatalf("problems = %#v", linkErr.Items)
	}
	for _, item := range linkErr.Items {
		if !strings.Contains(item.Problem, want[item.Path]) || item.Fix == "" {
			t.Errorf("%s: %q (fix %q)", item.Path, item.Problem, item.Fix)
		}
	}
	if version, err := DetectVersion(root); err != nil || version != 6 {
		t.Fatalf("version = %d, %v", version, err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "core", "alpha", ".meta", "distill.yaml")); !os.IsNotExist(err) {
		t.Fatal("a refused preview wrote a distill.yaml")
	}
}

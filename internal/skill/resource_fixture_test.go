package skill

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/workspace"
)

const resourceSkillDirPath = "skills/core/res-skill"

type resourceFixture struct {
	root string
	dir  string
}

func (f resourceFixture) path(relative string) string { return f.dir + "/" + relative }

func (f resourceFixture) abs(relative string) string {
	return filepath.Join(f.root, filepath.FromSlash(f.path(relative)))
}

func digestOf(contents []byte) string {
	sum := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// newResourceFixture creates an active skill with a mix of companion files and
// a second skill, then rebuilds the catalog so every file is a manifest row.
func newResourceFixture(t *testing.T) resourceFixture {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	manager := Manager{}
	for _, id := range []string{"res-skill", "other"} {
		created, err := manager.PreviewCreate(t.Context(), root, CreateInput{
			ID: id, Collection: "core", Name: "Resource Skill " + id, Description: "Resource editing fixture.",
			Content: []byte("# Resource Skill\n\nReal instructions.\n"),
			Routing: RoutingInput{Triggers: []string{"resource " + id}, NotFor: []string{"none"}, MinScope: "single_step"},
		}, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Confirm(t.Context(), root, created); err != nil {
			t.Fatal(err)
		}
	}
	activate, err := manager.PreviewTransition(t.Context(), root, "res-skill", "active", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Confirm(t.Context(), root, activate); err != nil {
		t.Fatal(err)
	}
	fixture := resourceFixture{root: root, dir: resourceSkillDirPath}
	big := bytes.Repeat([]byte("0123456789abcde\n"), (3<<20)/2/16)
	files := map[string][]byte{
		"scripts/run.sh":       []byte("#!/bin/sh\necho one\necho two\n"),
		"references/guide.md":  []byte("# Guide\n\nFirst line.\n"),
		"references/win.txt":   []byte("alpha\r\nbeta\r\n"),
		"references/bom.txt":   []byte("\xef\xbb\xbfwith bom\n"),
		"references/link.md":   []byte("# Link target\n"),
		"docs/nested/note.md":  []byte("# Note\n"),
		"assets/logo.png":      []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"),
		"references/large.txt": big,
	}
	for relative, contents := range files {
		target := fixture.abs(relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "skills/core/other/notes.md"), []byte("# Other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fixture.abs("scripts/run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.BuildCatalogGeneration(t.Context(), root, catalog.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	return fixture
}

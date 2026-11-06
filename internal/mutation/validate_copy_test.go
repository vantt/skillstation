package mutation

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/workspace"
)

func newValidationHub(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	return root
}

// Validating a change must not depend on, or copy, what the validator never reads: the Git
// repository and the host wrappers. A wrapper holding a symbolic link used to make every
// change to the hub fail.
func TestValidationCopyLeavesOutGitAndHostWrappers(t *testing.T) {
	t.Parallel()
	root := newValidationHub(t)
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, ".claude", "skills", "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sources", "catalog", "SRC-1.yaml"), []byte("id: SRC-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	copyRoot := t.TempDir()
	if err := copyCanonicalFiles(root, copyRoot); err != nil {
		t.Fatal(err)
	}
	for _, left := range []string{".git", ".claude"} {
		if _, err := os.Lstat(filepath.Join(copyRoot, left)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s was copied into the validation copy: %v", left, err)
		}
	}
	if _, err := os.Stat(filepath.Join(copyRoot, "sources", "catalog", "SRC-1.yaml")); err != nil {
		t.Fatalf("canonical file missing from the validation copy: %v", err)
	}

	change := []Change{{Path: "sources/catalog/SRC-2.yaml", Contents: []byte("id: SRC-2\n")}}
	if err := validateVirtual(root, change); err != nil {
		t.Fatalf("a hub with a symbolic link in a host wrapper must still validate: %v", err)
	}
}

// The copy has no index, so an unresolved merge in the real hub must still stop a change.
func TestValidationStillRefusesUnresolvedGitIndexConflict(t *testing.T) {
	t.Parallel()
	root := newValidationHub(t)
	run := func(stdin string, args ...string) string {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Stdin = strings.NewReader(stdin)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	blob := run("id: SRC-3\n", "hash-object", "-w", "--stdin")
	path := "sources/catalog/SRC-3.yaml"
	run("100644 "+blob+" 1\t"+path+"\n100644 "+blob+" 2\t"+path+"\n100644 "+blob+" 3\t"+path+"\n", "update-index", "--index-info")

	err := validateVirtual(root, []Change{{Path: "sources/catalog/SRC-4.yaml", Contents: []byte("id: SRC-4\n")}})
	var invalid *ValidationError
	if !errors.As(err, &invalid) || !strings.Contains(invalid.Issue.Message, "unresolved Git index conflict") {
		t.Fatalf("an unresolved index conflict must refuse the change, got: %v", err)
	}
}

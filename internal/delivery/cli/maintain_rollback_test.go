package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/mutation"
)

// A change blocked by an outside edit: doctor names the rollback command, the
// command previews without writing, and --yes undoes the change so the hub
// takes normal writes again.
func TestMaintainRollbackIsTheExitOfABlockedChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := initTestWorkspace(t)
	created, edited := "sources/catalog/SRC-CREATED.yaml", "sources/catalog/SRC-EDITED.yaml"
	failure := errors.New("interrupted")
	_, err := mutation.CommitWithOptions(root, mutation.WriteSet{OperationID: "OP-BLOCKED", Command: "create_sources", Changes: []mutation.Change{
		{Path: created, Contents: []byte("id: SRC-CREATED\n")},
		{Path: edited, Contents: []byte("id: SRC-EDITED\n")},
	}}, mutation.Options{Fault: func(point mutation.FaultPoint) error {
		if point == mutation.FaultLastCanonicalReplace {
			return failure
		}
		return nil
	}})
	if !errors.Is(err, failure) {
		t.Fatalf("commit error = %v", err)
	}
	editedFile := filepath.Join(root, filepath.FromSlash(edited))
	if err := os.WriteFile(editedFile, []byte("id: SRC-EDITED\nname: by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, stdout, _ := runCLI(t, "doctor", "--workspace", root); !strings.Contains(stdout, "skillstation maintain rollback OP-BLOCKED") {
		t.Fatalf("doctor does not name the rollback command:\n%s", stdout)
	}

	code, stdout, stderr := runCLI(t, "maintain", "rollback", "OP-BLOCKED", "--workspace", root, "--json")
	var preview app.RollbackResult
	if code != 0 || json.Unmarshal([]byte(stdout), &preview) != nil || preview.Applied {
		t.Fatalf("preview exit %d: %s %s", code, stdout, stderr)
	}
	actions := map[string]string{}
	for _, step := range preview.Steps {
		actions[step.Path] = step.Action
	}
	if actions[created] != "remove" || actions[edited] != "leave" {
		t.Fatalf("preview steps = %v", actions)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(created))); err != nil {
		t.Fatalf("the preview changed a file: %v", err)
	}

	if code, stdout, stderr := runCLI(t, "maintain", "rollback", "OP-BLOCKED", "--workspace", root, "--yes"); code != 0 || !strings.Contains(stdout, "rolled back") {
		t.Fatalf("rollback exit %d: %s %s", code, stdout, stderr)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(created))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s survived the rollback: %v", created, err)
	}
	if got, _ := os.ReadFile(editedFile); string(got) != "id: SRC-EDITED\nname: by hand\n" {
		t.Fatalf("the outside edit was overwritten: %q", got)
	}
	if pending, err := mutation.Pending(root); err != nil || len(pending) != 0 {
		t.Fatalf("pending after the rollback = %v, %v", pending, err)
	}
	if code, _, stderr := runCLI(t, "maintain", "rollback", "OP-BLOCKED", "--workspace", root); code != 2 || !strings.Contains(stderr, "No interrupted change") {
		t.Fatalf("a second rollback: exit %d: %s", code, stderr)
	}
}

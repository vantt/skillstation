package mutation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeCanonical(t *testing.T, root, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

func interruptAt(t *testing.T, root string, set WriteSet, at FaultPoint) {
	t.Helper()
	failure := errors.New("interrupted")
	_, err := CommitWithOptions(root, set, Options{Fault: func(point FaultPoint) error {
		if point == at {
			return failure
		}
		return nil
	}})
	if !errors.Is(err, failure) {
		t.Fatalf("commit error = %v; want the injected interruption", err)
	}
}

func readCanonical(root, path string) (string, error) {
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	return string(contents), err
}

// A change blocked by an outside edit is undone where that is safe: what the
// change wrote goes back, the outside edit stays, and normal writes resume.
func TestConflictRollbackUndoesTheChangeAndKeepsOutsideEdits(t *testing.T) {
	t.Parallel()
	root := newWorkspace(t)
	replaced, created, edited := "sources/catalog/SRC-R.yaml", "sources/catalog/SRC-N.yaml", "sources/catalog/SRC-X.yaml"
	old := []byte("id: SRC-R\nname: old\n")
	writeCanonical(t, root, replaced, old)
	interruptAt(t, root, WriteSet{OperationID: "OP-CONFLICT", Command: "replace_sources", Changes: []Change{
		{Path: replaced, BeforeDigest: digest(old), Contents: []byte("id: SRC-R\nname: new\n")},
		{Path: created, Contents: []byte("id: SRC-N\n")},
		{Path: edited, Contents: []byte("id: SRC-X\n")},
	}}, FaultLastCanonicalReplace)
	writeCanonical(t, root, edited, []byte("id: SRC-X\nname: edited by hand\n"))

	if _, err := PlanConflictRollback(root, "OP-MISSING"); !errors.Is(err, ErrNoInterruptedChange) {
		t.Fatalf("plan of an unknown operation = %v", err)
	}
	preview, err := PlanConflictRollback(root, "OP-CONFLICT")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]RollbackAction{replaced: RollbackRestore, created: RollbackRemove, edited: RollbackLeave}
	for _, step := range preview.Steps {
		if action, named := want[step.Path]; named && action != step.Action {
			t.Errorf("%s: action %s; want %s", step.Path, step.Action, action)
		}
	}
	if _, err := RollBackConflict(root, "OP-CONFLICT", []RollbackStep{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("rollback with a different approved plan = %v; want a conflict", err)
	}
	if _, err := RollBackConflict(root, "OP-CONFLICT", preview.Steps); err != nil {
		t.Fatal(err)
	}

	if got, err := readCanonical(root, replaced); err != nil || got != string(old) {
		t.Fatalf("%s = %q, %v; want the earlier file", replaced, got, err)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(created))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s still exists: %v", created, err)
	}
	if got, _ := readCanonical(root, edited); got != "id: SRC-X\nname: edited by hand\n" {
		t.Fatalf("the outside edit was overwritten: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "runtime", "discarded-transactions", "OP-CONFLICT", "manifest.json")); err != nil {
		t.Fatalf("the journal was not kept: %v", err)
	}
	if _, err := CommitWithOptions(root, WriteSet{OperationID: "OP-AFTER", Command: "create_source", Changes: []Change{{Path: "sources/catalog/SRC-AFTER.yaml", Contents: []byte("id: SRC-AFTER\n")}}}, Options{}); err != nil {
		t.Fatalf("normal write after the rollback: %v", err)
	}
}

// Rollback is the exit for a blocked change only; one recovery can finish is
// left to doctor --fix.
func TestConflictRollbackRefusesARecoverableChange(t *testing.T) {
	t.Parallel()
	root := newWorkspace(t)
	interruptAt(t, root, threeFileWriteSet("OP-FORWARD"), FaultFirstCanonicalReplace)
	if _, err := RollBackConflict(root, "OP-FORWARD", nil); !errors.Is(err, ErrNotConflicted) {
		t.Fatalf("rollback of a recoverable change = %v", err)
	}
	if pending, err := Pending(root); err != nil || len(pending) != 1 {
		t.Fatalf("pending = %v, %v; the journal must stay", pending, err)
	}
}

// A change stopped after moving the old file aside and before publishing the
// new one leaves the canonical name empty; rollback puts the old file back.
func TestConflictRollbackRestoresAFileMovedAsideButNotReplaced(t *testing.T) {
	t.Parallel()
	root := newWorkspace(t)
	path := "sources/catalog/SRC-R.yaml"
	old := []byte("id: SRC-R\nname: old\n")
	writeCanonical(t, root, path, old)
	interruptAt(t, root, WriteSet{OperationID: "OP-DISPLACED", Command: "replace_source", Changes: []Change{
		{Path: path, BeforeDigest: digest(old), Contents: []byte("id: SRC-R\nname: new\n")},
	}}, FaultAfterTargetDisplace)
	// A damaged after-image makes the change unrecoverable forward.
	writeCanonical(t, root, ".skillstation/transactions/OP-DISPLACED/after/"+path, []byte("damaged"))
	recoveries, err := InspectRecovery(root)
	if err != nil || len(recoveries) != 1 || recoveries[0].Action != RecoveryConflict {
		t.Fatalf("recovery = %#v, %v; want a conflict", recoveries, err)
	}
	if _, err := RollBackConflict(root, "OP-DISPLACED", nil); err != nil {
		t.Fatal(err)
	}
	if got, err := readCanonical(root, path); err != nil || got != string(old) {
		t.Fatalf("%s = %q, %v; want the earlier file", path, got, err)
	}
}

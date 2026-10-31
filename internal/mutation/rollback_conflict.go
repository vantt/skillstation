package mutation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// RollbackAction says what rolling back a conflicted change does to one path.
type RollbackAction string

const (
	// RollbackRestore puts back the file as it was before the change.
	RollbackRestore RollbackAction = "restore"
	// RollbackRemove removes a file the change created.
	RollbackRemove RollbackAction = "remove"
	// RollbackUnchanged marks a path the change had not reached yet.
	RollbackUnchanged RollbackAction = "unchanged"
	// RollbackLeave keeps a path as it is: it was edited outside the change, or
	// no valid copy of its earlier content exists. Nothing overwrites it.
	RollbackLeave RollbackAction = "leave"
)

// RollbackStep is what a conflict rollback does to one canonical path.
type RollbackStep struct {
	Path   string
	Action RollbackAction
	Reason string
}

// ConflictRollback describes the rollback of one interrupted change that
// recovery could not finish. The journal is not deleted but kept below
// runtime/, so the bytes it held stay available for a manual check.
type ConflictRollback struct {
	OperationID string
	Phase       string
	Detail      string
	Steps       []RollbackStep
	// Journal is the hub-relative folder the journal is moved to.
	Journal string
}

// ErrNotConflicted refuses a conflict rollback for a change that recovery can
// still finish or discard by itself.
var ErrNotConflicted = errors.New("the interrupted change is not blocked; recover it with `skillstation doctor --fix`")

// ErrNoInterruptedChange reports a rollback of an operation that has no journal.
var ErrNoInterruptedChange = errors.New("no interrupted change has this operation id")

// discardedTransactions is where rolled-back journals are kept. runtime/ is
// neither canonical nor tracked by Git.
const discardedTransactions = "runtime/discarded-transactions"

// PlanConflictRollback reports, without writing, what RollBackConflict would
// do for the conflicted change operationID.
func PlanConflictRollback(root, operationID string) (ConflictRollback, error) {
	lock, err := AcquireSharedLock(context.Background(), root, DefaultLockTimeout)
	if err != nil {
		return ConflictRollback{}, err
	}
	defer func() { _ = lock.Unlock() }()
	rollback, _, err := planConflictRollback(root, operationID)
	return rollback, err
}

// RollBackConflict undoes the parts of a conflicted change that are safe to
// undo: a path that still holds what the change wrote gets its earlier file
// back (or is removed when the change created it). A path edited outside the
// change is left as it is. The journal then moves to runtime/, which unblocks
// normal writes. When expected is not nil the plan must equal it, so a preview
// a person approved is what runs.
func RollBackConflict(root, operationID string, expected []RollbackStep) (ConflictRollback, error) {
	lock, err := AcquireExclusiveLock(context.Background(), root, DefaultLockTimeout)
	if err != nil {
		return ConflictRollback{}, err
	}
	defer func() { _ = lock.Unlock() }()
	rollback, plan, err := planConflictRollback(root, operationID)
	if err != nil {
		return ConflictRollback{}, err
	}
	if expected != nil && !slices.Equal(expected, rollback.Steps) {
		return ConflictRollback{}, fmt.Errorf("%w: the interrupted change %s changed since the preview", ErrConflict, operationID)
	}
	for index := len(plan.manifest.Files) - 1; index >= 0; index-- {
		step := rollback.Steps[index]
		if step.Action != RollbackRestore && step.Action != RollbackRemove {
			continue
		}
		item := plan.manifest.Files[index]
		if step.Action == RollbackRestore {
			if err := ensureDisplacedBeforeImage(root, plan.txn, item); err != nil {
				return ConflictRollback{}, err
			}
		}
		if err := rollbackCanonicalChecked(root, plan.txn, item); err != nil {
			return ConflictRollback{}, fmt.Errorf("roll back %s: %w", item.Path, err)
		}
	}
	if err := ensureSafeParents(root, discardedTransactions, 0o700); err != nil {
		return ConflictRollback{}, err
	}
	target := filepath.Join(root, filepath.FromSlash(rollback.Journal))
	if _, err := os.Lstat(target); err == nil {
		return ConflictRollback{}, fmt.Errorf("%s already exists; move it away and retry", rollback.Journal)
	} else if !errors.Is(err, os.ErrNotExist) {
		return ConflictRollback{}, err
	}
	if err := os.Rename(plan.txn, target); err != nil {
		return ConflictRollback{}, fmt.Errorf("move the journal of %s: %w", operationID, err)
	}
	if err := syncDir(filepath.Dir(plan.txn)); err != nil {
		return ConflictRollback{}, err
	}
	if err := syncDir(filepath.Dir(target)); err != nil {
		return ConflictRollback{}, err
	}
	return rollback, nil
}

func planConflictRollback(root, operationID string) (ConflictRollback, recoveryPlan, error) {
	if !validOpaqueID(operationID) {
		return ConflictRollback{}, recoveryPlan{}, fmt.Errorf("%w: %q", ErrNoInterruptedChange, operationID)
	}
	plans, err := preflightRecovery(root, false)
	if err != nil {
		return ConflictRollback{}, recoveryPlan{}, err
	}
	index := slices.IndexFunc(plans, func(plan recoveryPlan) bool { return plan.OperationID == operationID })
	if index < 0 {
		return ConflictRollback{}, recoveryPlan{}, fmt.Errorf("%w: %s", ErrNoInterruptedChange, operationID)
	}
	plan := plans[index]
	if plan.Action != RecoveryConflict {
		return ConflictRollback{}, recoveryPlan{}, fmt.Errorf("%s: %w", operationID, ErrNotConflicted)
	}
	rollback := ConflictRollback{
		OperationID: operationID, Phase: plan.Phase, Detail: plan.Detail, Steps: []RollbackStep{},
		Journal: discardedTransactions + "/" + operationID,
	}
	// An unreadable journal names no path, and a journal still staging has
	// changed none: every difference there was made outside the change.
	staging := plan.manifest.Phase == "staging"
	for _, item := range plan.manifest.Files {
		step, err := rollbackStep(root, plan.txn, item, staging)
		if err != nil {
			return ConflictRollback{}, recoveryPlan{}, err
		}
		rollback.Steps = append(rollback.Steps, step)
	}
	return rollback, plan, nil
}

func rollbackStep(root, txn string, item change, staging bool) (RollbackStep, error) {
	step := RollbackStep{Path: item.Path}
	actual, err := digestAt(root, item.Path)
	if err != nil {
		return step, err
	}
	written := !staging && actual == item.AfterDigest
	if !written && !staging && actual == "" && item.BeforeDigest != "" {
		// Interrupted after the old file was moved aside and before the new
		// one was published.
		written = beforeImageValid(root, txn, item)
	}
	switch {
	case actual == item.BeforeDigest:
		step.Action, step.Reason = RollbackUnchanged, "still as before the change"
	case !written:
		step.Action, step.Reason = RollbackLeave, "edited outside the change; left as it is"
	case item.BeforeDigest == "":
		step.Action, step.Reason = RollbackRemove, "created by the change"
	case beforeImageValid(root, txn, item):
		step.Action, step.Reason = RollbackRestore, "written by the change; the earlier file is put back"
	default:
		step.Action, step.Reason = RollbackLeave, "written by the change, but the journal holds no valid copy of the earlier file"
	}
	return step, nil
}

// beforeImageValid reports whether the journal holds the file as it was
// before the change, moved aside or copied in before/.
func beforeImageValid(root, txn string, item change) bool {
	if item.Displaced != "" {
		if actual, err := digestAt(root, item.Displaced); err == nil && actual == item.BeforeDigest {
			return true
		}
	}
	if item.BeforeStaged == "" {
		return false
	}
	contents, err := readStagedFile(txn, item.BeforeStaged)
	return err == nil && digest(contents) == item.BeforeDigest
}

// ensureDisplacedBeforeImage puts the before/ copy where rollback restores
// from when the moved-aside file is missing or damaged.
func ensureDisplacedBeforeImage(root, txn string, item change) error {
	if actual, err := digestAt(root, item.Displaced); err == nil && actual == item.BeforeDigest {
		return nil
	}
	contents, err := readStagedFile(txn, item.BeforeStaged)
	if err != nil || digest(contents) != item.BeforeDigest {
		return fmt.Errorf("%w: no valid earlier copy of %s", ErrRecoveryRequired, item.Path)
	}
	if err := writeFileSync(root, item.Displaced, contents); err != nil {
		return err
	}
	// The copy becomes the canonical file, which is created with the
	// canonical default mode.
	return os.Chmod(filepath.Join(root, filepath.FromSlash(item.Displaced)), 0o644)
}

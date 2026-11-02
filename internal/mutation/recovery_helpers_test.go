package mutation

import (
	"context"
	"fmt"
)

// The helpers below are for the tests of this package: no command rolls a
// transaction back without a conflict preview (RollBackConflict) and none lists
// transaction names.

// Pending reports transaction directory names without modifying the workspace.
func Pending(root string) ([]string, error) {
	if exists, err := transactionDirectoryExists(root); err != nil {
		return nil, err
	} else if !exists {
		return []string{}, nil
	}
	lock, err := AcquireSharedLock(context.Background(), root, DefaultLockTimeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Unlock() }()
	return pendingUnlocked(root)
}

// RollBack explicitly restores all pending transactions. Mixed before/after
// progress is allowed only when all paths are recognized and every required
// before-image is present and digest-valid. Preflight covers all transactions
// before the first restoration write.
func RollBack(root string) error {
	lock, err := AcquireExclusiveLock(context.Background(), root, DefaultLockTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()
	plans, err := preflightRollback(root)
	if err != nil {
		return err
	}
	for planIndex := len(plans) - 1; planIndex >= 0; planIndex-- {
		plan := plans[planIndex]
		if plan.Action == RecoveryAbort {
			if err := removeTransaction(plan.txn); err != nil {
				return err
			}
			continue
		}
		for index := len(plan.manifest.Files) - 1; index >= 0; index-- {
			item := plan.manifest.Files[index]
			actual, err := digestAt(root, item.Path)
			if err != nil {
				return err
			}
			if actual == item.BeforeDigest {
				continue
			}
			if actual != item.AfterDigest {
				if actual != "" || item.Displaced == "" || item.BeforeDigest == "" {
					return fmt.Errorf("%w: path %s changed during rollback", ErrRecoveryRequired, item.Path)
				}
				displacedDigest, displacedErr := digestAt(root, item.Displaced)
				if displacedErr != nil || displacedDigest != item.BeforeDigest {
					return fmt.Errorf("%w: displaced target for %s is invalid", ErrRecoveryRequired, item.Path)
				}
			}
			if err := rollbackCanonicalChecked(root, plan.txn, item); err != nil {
				return err
			}
		}
		if err := removeTransaction(plan.txn); err != nil {
			return err
		}
	}
	return nil
}

func preflightRollback(root string) ([]recoveryPlan, error) {
	plans, err := preflightRecovery(root, false)
	if err != nil {
		return nil, err
	}
	for index := range plans {
		plan := &plans[index]
		if plan.Action == RecoveryConflict {
			return nil, fmt.Errorf("%w: transaction %s: %s", ErrRecoveryRequired, plan.OperationID, plan.Detail)
		}
		if plan.Action == RecoveryAbort {
			continue
		}
		if err := validateStagedImages(plan.txn, plan.manifest, true); err != nil {
			return nil, err
		}
	}
	return plans, nil
}

package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/workspace"
)

// RollbackCommand is the CLI command that rolls back a blocked change.
const RollbackCommand = "skillstation maintain rollback"

// RollbackStep is what rolling back a blocked change does to one hub file.
type RollbackStep struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// RollbackResult is the preview or the outcome of rolling back one
// interrupted change that recovery cannot finish.
type RollbackResult struct {
	Result
	OperationID string         `json:"operation_id"`
	Phase       string         `json:"phase"`
	Detail      string         `json:"detail"`
	Steps       []RollbackStep `json:"steps"`
	// Journal is where the change's journal is kept after the rollback.
	Journal string `json:"journal"`
	Applied bool   `json:"applied"`
}

// RollbackService rolls back an interrupted change classified as a conflict,
// the one recovery state doctor --fix cannot leave on its own.
type RollbackService struct {
	// Expected, when not nil, are the steps of the preview a person approved;
	// an apply that would do anything else is refused.
	Expected []RollbackStep
}

// Rollback previews the rollback of operationID, or applies it with apply.
func (service RollbackService) Rollback(ctx context.Context, path, operationID string, apply bool) (RollbackResult, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return RollbackResult{}, err
	}
	if _, statErr := os.Lstat(root); errors.Is(statErr, os.ErrNotExist) {
		return RollbackResult{}, fmt.Errorf("no Skillstation workspace at %s", root)
	} else if statErr != nil {
		return RollbackResult{}, statErr
	}
	var rollback mutation.ConflictRollback
	if apply {
		var expected []mutation.RollbackStep
		if service.Expected != nil {
			expected = make([]mutation.RollbackStep, 0, len(service.Expected))
			for _, step := range service.Expected {
				expected = append(expected, mutation.RollbackStep{Path: step.Path, Action: mutation.RollbackAction(step.Action), Reason: step.Reason})
			}
		}
		rollback, err = mutation.RollBackConflict(root, operationID, expected)
	} else {
		rollback, err = mutation.PlanConflictRollback(root, operationID)
	}
	if err != nil {
		return RollbackResult{}, rollbackError(err, root, operationID)
	}
	result := RollbackResult{OperationID: rollback.OperationID, Phase: rollback.Phase, Detail: rollback.Detail, Steps: []RollbackStep{}, Journal: rollback.Journal, Applied: apply}
	for _, step := range rollback.Steps {
		result.Steps = append(result.Steps, RollbackStep{Path: step.Path, Action: string(step.Action), Reason: step.Reason})
	}
	if !apply {
		result.Result = NewResult(StatusActionRequired, "Rolling back the interrupted change "+operationID+" would change these files; nothing was changed yet.")
		result.Items = rollbackItems(result.Steps)
		result.SuggestedActions = []Action{{Label: "Roll back the interrupted change", Command: RollbackCommand + " " + operationID + " --yes --workspace " + root, RequiresConfirmation: true}}
		return result, nil
	}
	result.Result = NewResult(StatusApplied, "The interrupted change "+operationID+" was rolled back; its journal is kept in "+rollback.Journal+".")
	result.Items = rollbackItems(result.Steps)
	// The rolled-back files are not committed: Git shows the files left as
	// they were, and the search index rebuilds from the hub files.
	built, catalogErr := (CatalogService{}).BuildCatalogGeneration(ctx, root)
	if catalogErr == nil && built.Error != nil {
		catalogErr = built.Error
	}
	if catalogErr != nil {
		result.Warnings = append(result.Warnings, Warning{Code: "catalog_not_rebuilt", Summary: "The search index was not rebuilt: " + catalogErr.Error()})
	}
	result.SuggestedActions = []Action{
		{Label: "Check the hub", Command: "skillstation doctor --workspace " + root},
		{Label: "Review the files left as they were", Command: "git -C " + root + " status"},
	}
	return result, nil
}

func rollbackItems(steps []RollbackStep) []Item {
	items := make([]Item, 0, len(steps))
	for _, step := range steps {
		items = append(items, Item{ID: step.Action, Summary: step.Path + ": " + step.Action, Impact: step.Reason})
	}
	return items
}

func rollbackError(err error, root, operationID string) error {
	switch {
	case errors.Is(err, mutation.ErrNotConflicted):
		return NewInvalidRequestError("The interrupted change "+operationID+" is not blocked, so it is not rolled back.", "Run `skillstation doctor --fix --workspace "+root+"`; it finishes the change.")
	case errors.Is(err, mutation.ErrNoInterruptedChange):
		return NewInvalidRequestError("No interrupted change has the id "+operationID+".", "Run `skillstation doctor --workspace "+root+"` to list the interrupted changes.")
	case errors.Is(err, mutation.ErrConflict):
		return NewInvalidRequestError("The interrupted change "+operationID+" changed since the preview.", "Run `"+RollbackCommand+" "+operationID+"` again and review the new preview.")
	}
	return err
}

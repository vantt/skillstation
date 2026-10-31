package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
)

// runRollback is `maintain rollback <operation-id>`: the exit for an
// interrupted change that doctor --fix cannot finish. It previews what it
// undoes; --yes, or y at the prompt, applies exactly that preview.
func runRollback(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	workspacePath, operationID, yes, jsonOutput, err := rollbackFlags(args)
	if err != nil {
		var resErr *WorkspaceResolutionError
		if errors.As(err, &resErr) {
			return writeWorkspaceResolutionError(stdout, stderr, hasJSONFlag(args), resErr)
		}
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), err.Error(), "Run `"+app.RollbackCommand+" <operation-id>`; `skillstation doctor` names the operation id.")
	}
	result, err := (app.RollbackService{}).Rollback(ctx, workspacePath, operationID, yes)
	if err != nil {
		return writeRollbackError(stdout, stderr, jsonOutput, err)
	}
	if !yes {
		apply, code := confirmInPlace(stdout, jsonOutput, true, func(interactive bool) int {
			return writeRollbackResult(stdout, stderr, jsonOutput, result, rerunCommand(app.RollbackCommand, args), interactive)
		})
		if !apply {
			return code
		}
		if result, err = (app.RollbackService{Expected: result.Steps}).Rollback(ctx, workspacePath, operationID, true); err != nil {
			return writeRollbackError(stdout, stderr, jsonOutput, err)
		}
	}
	return writeRollbackResult(stdout, stderr, jsonOutput, result, "", false)
}

func writeRollbackError(stdout, stderr io.Writer, jsonOutput bool, err error) int {
	var structured *app.Error
	if errors.As(err, &structured) {
		writeStructuredSkillError(stdout, stderr, jsonOutput, structured)
		return 2
	}
	return writeInvalidWorkspace(stdout, stderr, jsonOutput, err)
}

// writeRollbackResult prints a rollback preview or its outcome: one line per
// file of the change with what happens to it.
func writeRollbackResult(stdout, stderr io.Writer, jsonOutput bool, result app.RollbackResult, rerun string, interactive bool) int {
	return writeResult(stdout, stderr, jsonOutput, result, func(p *termui.Printer) {
		p.Line(result.Summary)
		if result.Detail != "" {
			p.Line("Why recovery is blocked: " + result.Detail)
		}
		if len(result.Steps) == 0 {
			p.Line("The journal names no file, so no hub file changes; only the journal moves to " + result.Journal + ".")
		}
		for _, step := range result.Steps {
			p.Line("  " + step.Path + ": " + step.Action + " (" + step.Reason + ")")
		}
		if !result.Applied {
			p.Line("The journal moves to " + result.Journal + "; files left as they are stay for you to check with `git status`.")
			if rerun != "" {
				if !interactive {
					p.Line("Nothing written yet.")
					p.Raw("Apply: " + rerun + " --yes\n")
				}
			}
			return
		}
		for _, warning := range result.Warnings {
			p.Warning(warning.Summary)
		}
		p.Line("Next: run `skillstation doctor`, and `git status` in the hub for the files left as they were.")
	})
}

func rollbackFlags(args []string) (workspacePath, operationID string, yes, jsonOutput bool, err error) {
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; arg {
		case "--workspace":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return "", "", false, false, errors.New("--workspace requires a path")
			}
			index++
			workspacePath = args[index]
		case "--yes":
			yes = true
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", "", false, false, fmt.Errorf("unknown argument %q", arg)
			}
			if operationID != "" {
				return "", "", false, false, errors.New("rollback takes one operation id")
			}
			operationID = arg
		}
	}
	if operationID == "" {
		return "", "", false, false, errors.New("rollback requires the operation id of the interrupted change")
	}
	resolved, resErr := resolveWorkspace(workspacePath)
	if resErr != nil {
		return "", "", false, false, resErr
	}
	return resolved, operationID, yes, jsonOutput, nil
}

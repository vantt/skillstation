package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
)

const tryUsage = "Run `skillstation skill try <folder|id> [--yes] [--json]`."

// runSkillTry is `skill try`: it tries a skill folder's runtime block from a
// read-only copy with a temporary state folder, like the hub serves it. It
// needs no hub. The exit code is 0 when the block works or there is nothing to
// try, and 1 when it does not work.
func runSkillTry(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var target string
	var confirm, jsonOutput bool
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "--yes":
			confirm = true
		case strings.HasPrefix(arg, "--"):
			return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), fmt.Sprintf("try does not accept %s", arg), tryUsage)
		case target != "":
			return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), "try accepts one skill folder or id", tryUsage)
		default:
			target = arg
		}
	}
	if target == "" {
		return writeInvalidRequest(stdout, stderr, jsonOutput, "try needs a skill folder or the id of a skill checked out in this project", tryUsage)
	}
	result, err := (app.SkillTryService{}).Try(ctx, target, currentDirectory(), confirm)
	if err != nil {
		return writeSkillErrorFor(target, stdout, stderr, jsonOutput, err)
	}
	code := writeResult(stdout, stderr, jsonOutput, result, func(p *termui.Printer) {
		p.Line(result.Summary)
		for _, step := range result.Steps {
			p.Raw(fmt.Sprintf("  - %s: %s", step.Step, step.Status))
			if step.Detail != "" {
				p.Raw(" (" + step.Detail + ")")
			}
			p.Raw("\n")
			if step.Status != "pass" && step.Output != "" {
				p.Raw("    " + strings.ReplaceAll(strings.TrimSpace(step.Output), "\n", "\n    ") + "\n")
			}
		}
		for _, check := range result.Checks {
			if check.Status != "pass" {
				p.Raw(fmt.Sprintf("  - %s %s: %s %s\n", check.Kind, check.Name, check.Status, check.Detail))
			}
		}
		for _, warning := range result.Warnings {
			p.Raw("  ! " + warning.Code + ": " + warning.Summary + "\n")
		}
		if result.Outcome == app.TryPreview {
			p.Raw("  Setup: " + result.SetupCommand + "\n  Check: " + result.CheckCommand + "\n")
		}
	})
	if code == 0 && result.Outcome == app.TryFails {
		return 1
	}
	return code
}

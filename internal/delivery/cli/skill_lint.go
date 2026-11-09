package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
)

// runSkillLint lists the skills whose routing needs work, active skills first.
// With --json it is the worklist an agent drafts new routing from.
func runSkillLint(ctx context.Context, flags skillFlags, stdout, stderr io.Writer) int {
	result, err := (app.SkillService{}).LintRouting(ctx, flags.workspace, app.SkillLintInput{SkillID: flags.id, All: flags.all})
	if err != nil {
		return writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	return writeResult(stdout, stderr, flags.jsonOutput, result, func(p *termui.Printer) {
		p.Line(result.Summary)
		for _, entry := range result.Skills {
			p.Blank()
			p.Line(fmt.Sprintf("%s (%s): %s", entry.ID, entry.State, termui.Plural(len(entry.Findings), "finding", "findings")))
			for _, finding := range entry.Findings {
				p.Raw("  - " + finding.Code + ": " + finding.Message + ". " + strings.TrimSuffix(finding.Fix, ".") + ".\n")
			}
			p.Raw("  Triggers: " + quoteAll(entry.Routing.Triggers) + "\n")
			p.Raw(fmt.Sprintf("  Examples: %d · Not for: %s\n", len(entry.Routing.Examples), quoteAll(entry.Routing.NotFor)))
		}
		if len(result.Skills) > 0 {
			p.Blank()
			p.Line("Fix one with `skillstation skill edit <id> --trigger ... --example ... --not-for ...`: each routing flag replaces its whole field, so repeat the entries you keep. Or ask your agent to improve the routing; it reads `skillstation skill lint --json`.")
		}
	})
}

// runRemoteLint checks the routing of a folder of this project against the hub
// on another machine. It sends SKILL.md and skill.yaml only, and the hub keeps
// nothing.
func runRemoteLint(ctx context.Context, flags skillFlags, stdout, stderr io.Writer) int {
	if flags.all {
		return writeInvalidRequest(stdout, stderr, flags.jsonOutput, "--all lints the skills of a hub on this machine", "Name a skill folder: `skillstation skill lint <folder>`.")
	}
	target := ""
	if len(flags.positionals) > 0 {
		target = flags.positionals[0]
	}
	result, err := (app.RemoteLintService{}).Lint(ctx, *flags.remote, target, currentDirectory())
	if err != nil {
		return writeSkillErrorFor(target, stdout, stderr, flags.jsonOutput, err)
	}
	if !flags.jsonOutput && app.ErrorOf(result, nil) != nil && len(result.Findings) > 0 {
		// The hub could not be asked, but the folder's own findings stand.
		p := termui.New(stdout)
		p.Line(result.SkillID + ": the hub could not be asked, so these are the findings from this folder alone:")
		for _, finding := range result.Findings {
			p.Raw("  - " + finding.Code + ": " + finding.Message + ". " + strings.TrimSuffix(finding.Fix, ".") + ".\n")
		}
	}
	return writeResult(stdout, stderr, flags.jsonOutput, result, func(p *termui.Printer) {
		p.Line(result.Summary)
		for _, finding := range result.Findings {
			p.Raw("  - " + finding.Code + ": " + finding.Message + ". " + strings.TrimSuffix(finding.Fix, ".") + ".\n")
		}
		p.Raw("  Triggers: " + quoteAll(result.Routing.Triggers) + "\n")
		p.Raw(fmt.Sprintf("  Examples: %d · Not for: %s\n", len(result.Routing.Examples), quoteAll(result.Routing.NotFor)))
	})
}

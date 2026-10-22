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

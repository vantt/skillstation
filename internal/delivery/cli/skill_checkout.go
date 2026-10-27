package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
	"github.com/vantt/skillstation/internal/shellquote"
)

// currentDirectory is the folder a project command works from.
func currentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// runSkillCheckout copies a hub skill into the project, to edit and try there.
func runSkillCheckout(ctx context.Context, flags skillFlags, stdout, stderr io.Writer) int {
	result, err := (app.SkillCheckoutService{}).Checkout(ctx, flags.workspace, app.SkillCheckoutInput{SkillID: flags.id, ProjectDir: currentDirectory()})
	if err != nil {
		return writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	return writeResult(stdout, stderr, flags.jsonOutput, result, func(p *termui.Printer) { printCheckout(p, result) })
}

// printCheckout says where the checked-out files are, which file only points
// at them, and what to do next.
func printCheckout(p *termui.Printer, result app.SkillCheckoutResult) {
	p.Line(result.Summary)
	printSkillLocation(p, result.Folder, result.Path, result.ClaudePointer, result.GitignoreAdded)
	if result.DraftRecorded {
		p.Line("Agents in this project now use this checkout; other projects keep the current version.")
	} else {
		p.Line("The hub keeps serving the current version; agents in other projects are not affected.")
	}
	p.Line(fmt.Sprintf("Try it: ask your agent to use /%s.", result.SkillID))
	p.Raw("When ready: skillstation skill commit " + shellquote.Arg(result.SkillID) + "\n")
}

// printSkillLocation names the one folder that holds a project skill's files,
// the agents that read it there, and the pointer Claude Code reads instead.
func printSkillLocation(p *termui.Printer, folder, rel, pointer string, gitignoreAdded []string) {
	p.Line("Edit the skill here; it is the only copy:")
	p.Raw(fmt.Sprintf("  %s%c\n", folder, filepath.Separator))
	p.Line(fmt.Sprintf("Codex, Gemini CLI, Antigravity, pi and omp read %s/ directly.", rel))
	p.Line(fmt.Sprintf("Claude Code reads %s, which only points to that folder; do not edit it.", pointer))
	if len(gitignoreAdded) > 0 {
		added := make([]string, len(gitignoreAdded))
		for i, pattern := range gitignoreAdded {
			added[i] = strings.TrimPrefix(pattern, "/")
		}
		p.Line("Added " + strings.Join(added, " and ") + " to .gitignore.")
	}
}

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
	"github.com/vantt/skillstation/internal/shellquote"
)

// runSkillPull brings the hub's newer version of a checked-out skill into its
// folder. A person at a terminal settles every file both sides changed with
// one answer; without a terminal each needs --accept.
func runSkillPull(ctx context.Context, flags skillFlags, stdout, stderr io.Writer) int {
	resolutions, err := resolutionsFromFlags(flags)
	if err != nil {
		return writeInvalidRequest(stdout, stderr, flags.jsonOutput, err.Error(), "Review `skillstation skill pull` arguments and retry.")
	}
	input := app.SkillPullInput{Target: flags.positionals[0], ProjectDir: currentDirectory(), Resolutions: resolutions}
	if flags.remote != nil && !flags.remote.MaySendToken {
		// Fetching sends the hub token, as a remote checkout does.
		if flags.jsonOutput || !confirmInteractive(stdout) {
			return writeInvalidRequest(stdout, stderr, flags.jsonOutput,
				fmt.Sprintf("only this project's files name the hub at %s, so pull does not send your hub token there", flags.remote.URL),
				"Run the command in a terminal and answer the question, or trust that hub with `skillstation connect --hub "+flags.remote.URL+" --user`.")
		}
		if !askYesNo(stdout, fmt.Sprintf("Fetch %s from the hub at %s? Your hub token is sent there.", input.Target, flags.remote.URL)) {
			fmt.Fprintln(stdout, "Nothing fetched.")
			return 0
		}
	}
	pull := func(input app.SkillPullInput) (app.SkillPullResult, error) {
		if flags.remote != nil {
			return (app.RemotePullService{}).Pull(ctx, *flags.remote, input)
		}
		return (app.SkillPullService{}).Pull(ctx, flags.workspace, input)
	}
	result, err := pull(input)
	if err != nil {
		return writeSkillErrorFor(input.Target, stdout, stderr, flags.jsonOutput, err)
	}
	rerun := "skillstation skill pull " + shellquote.Arg(input.Target)
	if len(result.Conflicts) > 0 && !flags.jsonOutput && confirmInteractive(stdout) {
		p := termui.New(stdout)
		writeSkillPullResult(p, result, "")
		files, them := "this file", "it"
		if len(result.Conflicts) > 1 {
			files, them = fmt.Sprintf("these %d files", len(result.Conflicts)), "them"
		}
		if !askYesNo(stdout, fmt.Sprintf("Take the hub version of %s? Your own changes to %s are dropped.", files, them)) {
			p.Line("Nothing written.")
			writePullChoices(p, result, rerun)
			return 0
		}
		for _, path := range result.Conflicts {
			input.Resolutions = append(input.Resolutions, app.SkillCommitResolution{Path: path, Choice: "hub"})
		}
		if result, err = pull(input); err != nil {
			return writeSkillErrorFor(input.Target, stdout, stderr, false, err)
		}
	}
	return writeResult(stdout, stderr, flags.jsonOutput, result, func(p *termui.Printer) {
		writeSkillPullResult(p, result, rerun)
	})
}

// writeSkillPullResult says what the pull changed, which of the project's own
// changes it kept and, with rerun, how to settle what still needs a decision.
func writeSkillPullResult(p *termui.Printer, result app.SkillPullResult, rerun string) {
	p.Line(result.Summary)
	for _, warning := range result.Warnings {
		p.Line(warning.Summary)
	}
	if len(result.Files) > 0 {
		rows := make([]string, 0, len(result.Files))
		for _, file := range result.Files {
			rows = append(rows, file.Change+" "+file.Path+"\t"+lineCounts(file))
		}
		p.Raw(alignedRows(rows, "  ", false))
	}
	if len(result.Kept) > 0 && len(result.Conflicts) == 0 {
		p.Line("Your own changes are kept in:")
		p.Bullets(result.Kept...)
	}
	if len(result.Conflicts) > 0 {
		p.Line(fmt.Sprintf("%s by both the hub and you:", termui.Plural(len(result.Conflicts), "file was changed", "files were changed")))
		p.Bullets(result.Conflicts...)
		if rerun != "" {
			writePullChoices(p, result, rerun)
		}
		return
	}
	if !result.UpToDate {
		p.Raw("When ready: skillstation skill commit " + shellquote.Arg(result.SkillID) + "\n")
	}
}

// writePullChoices says how to decide each file without a terminal.
func writePullChoices(p *termui.Printer, result app.SkillPullResult, rerun string) {
	p.Raw("Decide each: " + rerun + " --accept " + shellquote.Arg(result.Conflicts[0]) + "=hub   (or =project to keep yours)\n")
}

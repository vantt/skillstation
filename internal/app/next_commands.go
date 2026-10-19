package app

import (
	"strings"

	"github.com/vantt/skillstation/internal/shellquote"
)

// ShellCommandFor turns a suggested-action command into the plain shell
// command a person can run. Commands that are MCP tool names map to their
// CLI counterpart; a command that is already a shell command is returned as
// it is. The result carries no --json or --workspace flags.
func ShellCommandFor(command, id string) string {
	if strings.HasPrefix(command, "skillstation ") {
		return command
	}
	target := ""
	if id != "" {
		target = " " + shellquote.Arg(id)
	}
	var cli string
	switch command {
	case "workspace_validate", "ValidateWorkspace":
		cli = "skillstation maintain validate"
	case "workspace_rebuild":
		cli = "skillstation maintain rebuild"
	case "workspace_diff", "GetCurationDiff":
		cli = "skillstation maintain diff"
	case "source_check":
		cli = "skillstation source check"
	case "skill_upstream_status":
		cli = "skillstation skill outdated"
	case "skill_review":
		cli = "skillstation skill review" + target
	case "skill_list":
		cli = "skillstation skill list"
	case "skill_active":
		cli = "skillstation skill activate" + target
	case "skill_deprecated":
		cli = "skillstation skill deprecate" + target
	case "skill_archived":
		cli = "skillstation skill archive" + target
	case "skill_edit", "PreviewSkillUpdate":
		// Original edit inputs are unavailable after a stale confirmation.
		// Inspect the affected skill rather than advertise an invalid bare edit.
		cli = "skillstation skill review" + target
	case "skill_create":
		// The command line no longer creates a hub draft: a draft made through
		// the managed tools is inspected like any other skill.
		cli = "skillstation skill review" + target
	case "doctor":
		cli = "skillstation doctor"
	case "curation_run_start", "curation_run_retry":
		cli = "skillstation source list"
	default:
		if command != "" {
			return command
		} // Declared runtime setup is already a shell command.
		cli = "skillstation status"
	}
	if target == "" && (strings.HasPrefix(cli, "skillstation skill review") || strings.HasPrefix(cli, "skillstation skill edit")) {
		cli = "skillstation skill list"
	}
	return cli
}

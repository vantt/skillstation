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

const auditStatusUsage = "Run `skillstation skill audit-status <id>... | --all [--rules-digest <d>] [--json]`."

type auditStatusArgs struct {
	workspace   string
	ids         []string
	rulesDigest string
	jsonOutput  bool
}

// runSkillAuditStatus is `skill audit-status`: it prints each skill's folder,
// the digest of its content as the hub computes it, and its recorded audit, so
// the skill-audit script never reads hub metadata or hashes skill files itself.
func runSkillAuditStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	parsed, err := auditStatusFlags(args)
	if err != nil {
		var resErr *WorkspaceResolutionError
		if errors.As(err, &resErr) {
			return writeWorkspaceResolutionError(stdout, stderr, hasJSONFlag(args), resErr)
		}
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), err.Error(), auditStatusUsage)
	}
	result, err := (app.SkillAuditService{}).Status(ctx, parsed.workspace, parsed.ids, parsed.rulesDigest)
	if err != nil {
		var structured *app.Error
		if errors.As(err, &structured) {
			writeStructuredSkillError(stdout, stderr, parsed.jsonOutput, structured)
			return 2
		}
		return writeSkillErrorFor(strings.Join(parsed.ids, " "), stdout, stderr, parsed.jsonOutput, err)
	}
	return writeResult(stdout, stderr, parsed.jsonOutput, result, func(p *termui.Printer) {
		for _, skill := range result.Skills {
			line := skill.SkillID + ": " + strings.ReplaceAll(skill.SinceLastAudit, "-", " ")
			if skill.Recorded != nil {
				line += fmt.Sprintf(" (recorded %s on %s)", skill.Recorded.Verdict, skill.Recorded.AuditedAt)
			}
			p.Line(line)
		}
	})
}

func auditStatusFlags(args []string) (auditStatusArgs, error) {
	var parsed auditStatusArgs
	all := false
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; arg {
		case "--workspace", "--rules-digest":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return parsed, fmt.Errorf("%s requires a value", arg)
			}
			index++
			if arg == "--workspace" {
				parsed.workspace = args[index]
			} else {
				parsed.rulesDigest = args[index]
			}
		case "--all":
			all = true
		case "--json":
			parsed.jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return parsed, fmt.Errorf("unknown argument %q", arg)
			}
			parsed.ids = append(parsed.ids, arg)
		}
	}
	if all == (len(parsed.ids) > 0) {
		return parsed, errors.New("audit-status takes skill ids or --all")
	}
	resolved, resErr := resolveWorkspace(parsed.workspace)
	if resErr != nil {
		return parsed, resErr
	}
	parsed.workspace = resolved
	return parsed, nil
}

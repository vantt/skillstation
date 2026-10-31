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

const auditRecordUsage = "Run `skillstation skill audit-record <id> --verdict ok|review|block --content-digest <d> --rules-digest <d>`."

// runSkillAuditRecord is `skill audit-record`: the skill-audit system skill
// records a finished audit through it, so the record goes through the hub's
// mutation path and becomes its own commit.
func runSkillAuditRecord(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	workspacePath, id, record, jsonOutput, err := auditRecordFlags(args)
	if err != nil {
		var resErr *WorkspaceResolutionError
		if errors.As(err, &resErr) {
			return writeWorkspaceResolutionError(stdout, stderr, hasJSONFlag(args), resErr)
		}
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), err.Error(), auditRecordUsage)
	}
	result, err := (app.SkillAuditService{}).Record(commitContext(ctx, false, stdout), workspacePath, id, record)
	if err != nil {
		if refusal := app.UncommittedEditsError(err); refusal != nil {
			writeStructuredSkillError(stdout, stderr, jsonOutput, refusal)
			return 2
		}
		var structured *app.Error
		if errors.As(err, &structured) {
			writeStructuredSkillError(stdout, stderr, jsonOutput, structured)
			return 2
		}
		return writeSkillErrorFor(id, stdout, stderr, jsonOutput, err)
	}
	return writeResult(stdout, stderr, jsonOutput, result, func(p *termui.Printer) {
		p.Line(result.Summary)
		writeCommitLines(p, result.Commit)
	})
}

func auditRecordFlags(args []string) (workspacePath, id string, record app.AuditRecord, jsonOutput bool, err error) {
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; arg {
		case "--workspace", "--verdict", "--content-digest", "--rules-digest":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return "", "", record, false, fmt.Errorf("%s requires a value", arg)
			}
			index++
			switch arg {
			case "--workspace":
				workspacePath = args[index]
			case "--verdict":
				record.Verdict = args[index]
			case "--content-digest":
				record.ContentDigest = args[index]
			case "--rules-digest":
				record.RulesDigest = args[index]
			}
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", "", record, false, fmt.Errorf("unknown argument %q", arg)
			}
			if id != "" {
				return "", "", record, false, errors.New("audit-record takes one skill id")
			}
			id = arg
		}
	}
	if id == "" || record.Verdict == "" || record.ContentDigest == "" || record.RulesDigest == "" {
		return "", "", record, false, errors.New("audit-record requires a skill id, --verdict, --content-digest and --rules-digest")
	}
	resolved, resErr := resolveWorkspace(workspacePath)
	if resErr != nil {
		return "", "", record, false, resErr
	}
	return resolved, id, record, jsonOutput, nil
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
	"github.com/vantt/skillstation/internal/shellquote"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/workspace"
	"gopkg.in/yaml.v3"
)

type skillFlags struct {
	workspace, id, collection, name, description, contentFile, minScope, rationale string
	fromSuggestion                                                                 string
	proposalID, proposalDigest, baseVersion, idempotencyKey, state                 string
	locator, ref, subPath, approveContent, runtimeFile, target, writeConflicts     string
	// label is --label: the name a hub on another machine shows for this
	// machine; nil when the flag is not given.
	label *string
	// labels, topics and technologies are the repeatable --label, --topic and
	// --technology values; the *Set flags tell an empty value (clear) from
	// no flag. edit replaces the stored lists; list filters by them.
	labels, topics, technologies          []string
	labelsSet, topicsSet, technologiesSet bool
	// origin, search and sortBy are the skill list filters and ordering.
	origin, search, sortBy                                                       string
	operations, triggers, notFor, examples, counterExamples, skills, positionals []string
	accepts, manuals                                                             []string
	jsonOutput, yes, diff, editor, verbose, all, check, noCheck, exitCode        bool
	noCommit, editRouting, discard                                               bool
	// remote is the hub on another machine, set for the commands that work
	// from a project connected to one: commit and checkout.
	remote *app.RemoteHub
	// rerun is the command the person typed, for the hint lines of a preview.
	rerun string
}

// skillSubcommands lists the skill subcommands in the order `skillstation help
// skill` shows them; every "which subcommand" hint is built from it.
var skillSubcommands = []string{"add", "create", "checkout", "pull", "commit", "show", "review", "doctor", "env", "audit-record", "audit-status", "edit", "lint", "outdated", "upstream", "update", "activate", "deprecate", "archive", "confirm", "list"}

var skillSubcommandFix = "Run `skillstation skill " + strings.Join(skillSubcommands, "|") + "`."

// hubChangingSkillSubcommands lists the skill subcommands that write to the hub.
var hubChangingSkillSubcommands = map[string]bool{
	"add": true, "commit": true, "edit": true, "update": true, "activate": true, "deprecate": true, "archive": true, "confirm": true,
}

func runSkill(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), "skill requires a subcommand", skillSubcommandFix)
	}
	subcommand := args[0]
	if subcommand == "audit-record" {
		return runSkillAuditRecord(ctx, args[1:], stdout, stderr)
	}
	if subcommand == "audit-status" {
		return runSkillAuditStatus(ctx, args[1:], stdout, stderr)
	}
	flags, err := parseSkillFlags(subcommand, args[1:])
	if err != nil {
		var resErr *WorkspaceResolutionError
		if errors.As(err, &resErr) {
			return writeWorkspaceResolutionError(stdout, stderr, hasJSONFlag(args), resErr)
		}
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), err.Error(), "Review `skillstation skill "+subcommand+"` arguments and retry.")
	}
	service := app.SkillService{}
	switch subcommand {
	case "create":
		return runSkillCreate(ctx, flags, stdout, stderr)
	case "doctor":
		return runSkillDoctor(ctx, flags, stdout, stderr)
	case "env":
		return runSkillEnv(ctx, flags, stdout, stderr)
	case "add":
		return runSkillAdd(ctx, service, flags, stdout, stderr)
	case "checkout":
		if flags.remote != nil {
			return runRemoteCheckout(ctx, flags, stdout, stderr)
		}
		return runSkillCheckout(ctx, flags, stdout, stderr)
	case "pull":
		return runSkillPull(ctx, flags, stdout, stderr)
	case "lint":
		return runSkillLint(ctx, flags, stdout, stderr)
	case "commit":
		if flags.remote != nil {
			return runRemoteCommit(ctx, flags, stdout, stderr)
		}
		return runSkillCommit(ctx, flags, stdout, stderr)
	case "outdated":
		return runSkillOutdated(ctx, flags, stdout, stderr)
	case "upstream":
		return runSkillUpstream(ctx, flags, stdout, stderr)
	case "update":
		return runSkillUpdate(ctx, flags, stdout, stderr)
	case "review":
		return runSkillReview(ctx, service, flags, stdout, stderr)
	case "list":
		return runSkillList(ctx, service, flags, stdout, stderr)
	case "show":
		return runSkillShow(ctx, service, flags, stdout, stderr)
	case "confirm":
		return runSkillConfirm(ctx, service, flags, stdout, stderr)
	}
	return runSkillMutation(ctx, service, subcommand, flags, stdout, stderr)
}

func runSkillReview(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) int {
	if _, isSubmission := app.SubmissionIDOf(flags.id); isSubmission {
		return runSubmissionReview(ctx, flags, stdout, stderr)
	}
	result, err := service.ReviewSkill(ctx, flags.workspace, flags.id)
	if err != nil {
		return writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	code := writeSkillReview(stdout, stderr, flags.jsonOutput, flags.verbose, result)
	if code == 0 && !flags.jsonOutput {
		writeWaitingSubmissions(stdout, flags.workspace, flags.id)
	}
	return code
}

func runSkillShow(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) int {
	result, err := service.ReadSkill(ctx, flags.workspace, flags.id)
	if err != nil {
		return writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	if flags.jsonOutput {
		if err := writeJSON(stdout, result); err != nil {
			p := termui.New(stderr)
			p.Line(err.Error())
			return 1
		}
		return 0
	}
	routing, _ := service.ReadSkillRouting(ctx, flags.workspace, flags.id)
	filePath := ""
	for _, res := range result.Manifest.Resources {
		if strings.HasSuffix(res.Path, "/SKILL.md") {
			filePath = res.Path
			break
		}
	}
	orNone := func(value string) string {
		if value == "" {
			return "(none)"
		}
		return value
	}
	p := termui.New(stdout)
	var fields []termui.Field
	fields = append(fields,
		termui.Field{Label: "Skill", Value: fmt.Sprintf("%s (%s)", result.Manifest.Name, flags.id)},
		termui.Field{Label: "State", Value: result.Manifest.Status},
	)
	if filePath != "" {
		fields = append(fields, termui.Field{Label: "File", Value: filePath})
	}
	fields = append(fields,
		termui.Field{Label: "Triggers", Value: orNone(strings.Join(routing.Triggers, ", "))},
		termui.Field{Label: "Not for", Value: orNone(strings.Join(routing.NotFor, ", "))},
		termui.Field{Label: "Min scope", Value: orNone(routing.MinScope)},
	)
	if flags.verbose {
		fields = append(fields, termui.Field{Label: "Catalog snapshot", Value: result.Manifest.CatalogSnapshot})
	}
	p.Fields(fields...)
	p.Blank()
	p.Raw(result.Content)
	if !strings.HasSuffix(result.Content, "\n") {
		p.Raw("\n")
	}
	return 0
}

func runSkillConfirm(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) int {
	var pins *app.ConfirmationPins
	if flags.proposalDigest != "" && flags.baseVersion != "" {
		pins = &app.ConfirmationPins{
			ProposalID:     flags.proposalID,
			ProposalDigest: flags.proposalDigest,
			BaseVersion:    flags.baseVersion,
		}
	}
	res, err := service.DispatchConfirmProposal(commitContext(ctx, flags.noCommit, stdout), flags.workspace, flags.proposalID, pins)
	if err != nil {
		return writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	switch r := res.(type) {
	case app.SkillMutationResult:
		return writeSkillMutation(stdout, stderr, flags.jsonOutput, flags.verbose, r, nextStepForSkill(ctx, service, r, flags.workspace))
	case app.SkillAddResult:
		return writeResult(stdout, stderr, flags.jsonOutput, r, func(p *termui.Printer) {
			writeSkillAddResult(p, flags.verbose, r)
		})
	case app.UpstreamUpdateResult:
		return writeResult(stdout, stderr, flags.jsonOutput, r, func(p *termui.Printer) {
			p.Line(r.Summary)
			if r.TrustImpact.ReviewRequiredAfterApply {
				p.Next("review the updated skill:", "skillstation skill review "+shellquote.Arg(r.SkillID))
			}
		})
	default:
		if flags.jsonOutput {
			if err := writeJSON(stdout, res); err != nil {
				p := termui.New(stderr)
				p.Line(err.Error())
				return 1
			}
			return 0
		}
		p := termui.New(stdout)
		p.Line(fmt.Sprintf("Proposal %s confirmed.", flags.proposalID))
		return 0
	}
}

// skillMutationPreview is the stored proposal of a create, edit or state change.
type skillMutationPreview struct {
	proposal   app.SkillProposal
	suggestion *app.RoutingSuggestion
}

// runSkillMutation previews a create, edit or state change, asks for
// confirmation unless --yes, and applies it.
func runSkillMutation(ctx context.Context, service app.SkillService, subcommand string, flags skillFlags, stdout, stderr io.Writer) int {
	staged, code := previewSkillMutation(ctx, service, subcommand, flags, stdout, stderr)
	if code != 0 {
		return code
	}
	preview, suggestion := staged.proposal, staged.suggestion
	if !flags.yes {
		apply, code := confirmInPlace(stdout, flags.jsonOutput, preview.Error == nil, func(interactive bool) int {
			if flags.editor && !flags.jsonOutput {
				writeEditorSkillPreview(stdout, preview, flags.rerun, interactive)
				return 0
			}
			if suggestion != nil {
				return writeSuggestionPreview(stdout, stderr, flags, preview, *suggestion, interactive)
			}
			return writeSkillPreview(stdout, stderr, flags.jsonOutput, preview, previewHints{rerun: flags.rerun, diffShown: flags.diff, interactive: interactive})
		})
		if !apply {
			return code
		}
	}
	if !flags.jsonOutput {
		p := termui.New(stdout)
		if preview.RoutingImpact != nil {
			for _, warning := range preview.RoutingImpact.Warnings {
				p.Warning(warning)
			}
		}
		for _, warning := range preview.Warnings {
			p.Warning(warning.Summary)
		}
	}
	result, err := service.ConfirmSkillMutation(commitContext(ctx, flags.noCommit, stdout), flags.workspace, preview, preview.Confirmation.Confirmation.Pins)
	if err != nil {
		return writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	if flags.jsonOutput {
		result.Proposal = &preview
	}
	return writeSkillMutation(stdout, stderr, flags.jsonOutput, flags.verbose, result, nextStepForSkill(ctx, service, result, flags.workspace))
}

// previewSkillMutation stores the proposal of a create, edit or state change.
// It returns a non-zero exit code when the command is over.
func previewSkillMutation(ctx context.Context, service app.SkillService, subcommand string, flags skillFlags, stdout, stderr io.Writer) (skillMutationPreview, int) {
	var staged skillMutationPreview
	var err error
	switch subcommand {
	case "create":
		content, readErr := readContentFile(flags.contentFile)
		if readErr != nil {
			return staged, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, readErr)
		}
		staged.proposal, err = service.PreviewCreate(ctx, flags.workspace, skill.CreateInput{
			ID: flags.id, IdempotencyKey: flags.idempotencyKey, Collection: flags.collection, Name: flags.name, Description: flags.description, Content: content,
			Routing:   skill.RoutingInput{Operations: flags.operations, Triggers: flags.triggers, NotFor: flags.notFor, MinScope: flags.minScope, Examples: flags.examples, CounterExamples: flags.counterExamples},
			Rationale: flags.rationale,
		}, flags.diff)
	case "edit":
		if flags.fromSuggestion != "" {
			var code int
			staged.proposal, staged.suggestion, code = previewSuggestionEdit(ctx, service, flags, stdout, stderr)
			return staged, code
		}
		var code int
		staged.proposal, code = previewSkillEdit(ctx, service, flags, stdout, stderr)
		return staged, code
	case "activate", "deprecate", "archive":
		staged.proposal, err = service.PreviewTransitionWithKey(ctx, flags.workspace, flags.id, subcommandTarget(subcommand), flags.diff, flags.idempotencyKey)
	default:
		return staged, writeInvalidRequest(stdout, stderr, flags.jsonOutput, fmt.Sprintf("unsupported skill subcommand %q", subcommand), skillSubcommandFix)
	}
	if err != nil {
		return staged, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	return staged, 0
}

// previewSkillEdit stores the proposal of a skill edit. With --editor the
// proposal names the saved editor content, which confirming it removes. It
// returns a non-zero exit code when the command is over.
func previewSkillEdit(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) (app.SkillProposal, int) {
	var content []byte
	var setContent bool
	var expectedDigest, recoveryID, recPath string
	if flags.editor {
		session, sessErr := openEditorSession(ctx, service, flags.workspace, flags.id)
		if sessErr != nil {
			return app.SkillProposal{}, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, sessErr)
		}
		content = session.updated
		setContent = true
		expectedDigest = session.expectedDigest
		recoveryID = session.recoveryID
		recPath = session.recoveryPath
	} else if flags.contentFile != "" {
		var readErr error
		content, readErr = readContentFile(flags.contentFile)
		if readErr != nil {
			return app.SkillProposal{}, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, readErr)
		}
		setContent = true
	}
	update, code := skillUpdateInput(ctx, service, flags, stdout, stderr)
	if code != 0 {
		return app.SkillProposal{}, code
	}
	update.Content, update.SetContent, update.ExpectedContentDigest = content, setContent, expectedDigest
	update.RecoveryID = recoveryID
	fullDiff := flags.diff || flags.editor
	preview, err := service.PreviewSkillUpdate(ctx, flags.workspace, flags.id, update, fullDiff)
	if err != nil {
		if recoveryID != "" && !flags.jsonOutput {
			p := termui.New(stderr)
			p.Line(fmt.Sprintf("Edited content saved to %s", recPath))
		}
		return app.SkillProposal{}, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	if preview.Error != nil {
		if recoveryID != "" && !flags.jsonOutput {
			p := termui.New(stderr)
			p.Line(fmt.Sprintf("Edited content saved to %s", recPath))
		}
		if flags.jsonOutput {
			_ = writeJSON(stdout, preview)
			return app.SkillProposal{}, 2
		}
		p := termui.New(stderr)
		p.Error(preview.Error.Render.Error, preview.Error.Render.Why, preview.Error.Render.Fix)
		return app.SkillProposal{}, 2
	}
	return preview, 0
}

// skillUpdateInput builds the field changes of a skill edit from its flags,
// without the new content. It returns a non-zero exit code when the command is
// over.
func skillUpdateInput(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) (skill.UpdateInput, int) {
	update := skill.UpdateInput{IdempotencyKey: flags.idempotencyKey}
	if flags.name != "" {
		update.Name = &flags.name
	}
	if flags.description != "" {
		update.Description = &flags.description
	}
	if flags.rationale != "" {
		update.Rationale = &flags.rationale
	}
	if flags.approveContent != "" {
		update.ContentReviewedDigest = &flags.approveContent
	}
	if flags.runtimeFile != "" {
		block, readErr := readRuntimeFile(flags.runtimeFile)
		if readErr != nil {
			return update, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, readErr)
		}
		update.Runtime = block
	}
	update.Labels, update.Topics, update.Technologies = flags.facetEdits()
	if flags.hasRoutingFlags() {
		current, err := service.ReadSkillRouting(ctx, flags.workspace, flags.id)
		if err != nil {
			return update, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
		}
		update.Routing = mergeRouting(current, flags)
	}
	return update, 0
}

func subcommandTarget(subcommand string) string {
	return map[string]string{"activate": "active", "deprecate": "deprecated", "archive": "archived"}[subcommand]
}

// readRuntimeFile reads a YAML runtime block. An empty mapping ({}) is returned
// as an empty non-nil map, which removes the block on edit.
func readRuntimeFile(path string) (map[string]any, error) {
	contents, err := readContentFile(path)
	if err != nil {
		return nil, err
	}
	var block map[string]any
	if err := yaml.Unmarshal(contents, &block); err != nil {
		return nil, fmt.Errorf("runtime file is not valid YAML: %w", err)
	}
	if block == nil {
		return nil, errors.New("runtime file must contain a YAML mapping; use {} to remove the runtime block")
	}
	return block, nil
}

// maxContentFileBytes bounds an input content file; it is the largest file
// a hub accepts.
const maxContentFileBytes = workspace.MaxCanonicalFileBytesV1

// readContentFile reads the user's own input file from any path (relative to the
// current directory or absolute). The skill's canonical file is still written
// only through the mutation service. A symbolic link or non-regular file is
// refused, and the read is size-bounded.
func readContentFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read content file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("content file must be a regular file, not a symbolic link or directory")
	}
	handle, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read content file: %w", err)
	}
	defer handle.Close()
	opened, err := handle.Stat()
	if err != nil {
		return nil, fmt.Errorf("read content file: %w", err)
	}
	if !os.SameFile(info, opened) {
		return nil, errors.New("content file changed while it was being opened")
	}
	contents, err := io.ReadAll(io.LimitReader(handle, maxContentFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read content file: %w", err)
	}
	if int64(len(contents)) > maxContentFileBytes {
		return nil, fmt.Errorf("content file is larger than %d MiB", maxContentFileBytes>>20)
	}
	return contents, nil
}

// contentDigestPattern matches the content digest a human approves with
// --approve-content; it mirrors quality.content_reviewed_digest validation.
var contentDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// hasFacetEditFlags reports whether skill edit was asked to replace labels,
// topics or technologies.
func (flags skillFlags) hasFacetEditFlags() bool {
	return flags.labelsSet || flags.topicsSet || flags.technologiesSet
}

// facetEdits returns the lists skill edit replaces: nil keeps the stored
// list, a single empty value clears it.
func (flags skillFlags) facetEdits() (labels, topics, technologies *[]string) {
	pick := func(set bool, values []string) *[]string {
		if !set {
			return nil
		}
		kept := []string{}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				kept = append(kept, value)
			}
		}
		return &kept
	}
	return pick(flags.labelsSet, flags.labels), pick(flags.topicsSet, flags.topics), pick(flags.technologiesSet, flags.technologies)
}

// checkFacetFlags rejects the classification flags on a subcommand they do not
// belong to. --label also names this machine for skill commit.
func (flags skillFlags) checkFacetFlags(subcommand string) error {
	editOrList := subcommand == "edit" || subcommand == "list"
	switch {
	case flags.labelsSet && !editOrList && subcommand != "commit":
		return errors.New("--label is available only for skill list, skill edit, and skill commit to a hub on another machine")
	case flags.topicsSet && !editOrList:
		return errors.New("--topic is available only for skill list and skill edit")
	case flags.technologiesSet && subcommand != "edit":
		return errors.New("--technology is available only for skill edit")
	case (flags.origin != "" || flags.search != "" || flags.sortBy != "") && subcommand != "list":
		return errors.New("--origin, --search, and --sort are available only for skill list")
	case subcommand == "list" && (len(flags.labels) > 1 || len(flags.topics) > 1):
		return errors.New("skill list filters by one --label and one --topic")
	}
	return nil
}

// hasRoutingFlags reports whether any routing field flag was supplied.
func (flags skillFlags) hasRoutingFlags() bool {
	return len(flags.operations)+len(flags.triggers)+len(flags.notFor)+len(flags.examples)+len(flags.counterExamples) > 0 || flags.minScope != ""
}

// mergeRouting overlays the routing flags the user passed onto the stored routing.
func mergeRouting(current skill.RoutingInput, flags skillFlags) *skill.RoutingInput {
	if len(flags.operations) > 0 {
		current.Operations = flags.operations
	}
	if len(flags.triggers) > 0 {
		current.Triggers = flags.triggers
	}
	if len(flags.notFor) > 0 {
		current.NotFor = flags.notFor
	}
	if flags.minScope != "" {
		current.MinScope = flags.minScope
	}
	if len(flags.examples) > 0 {
		current.Examples = flags.examples
	}
	if len(flags.counterExamples) > 0 {
		current.CounterExamples = flags.counterExamples
	}
	return &current
}

func writeSkillPreview(stdout, stderr io.Writer, jsonOutput bool, preview app.SkillProposal, hints previewHints) int {
	return writeResult(stdout, stderr, jsonOutput, preview, func(p *termui.Printer) {
		p.Line(preview.Summary)
		p.Bullets(fmt.Sprintf("%d added, %d modified, %d deleted file(s).", len(preview.Diff.Added), len(preview.Diff.Modified), len(preview.Diff.Deleted)))
		pins := preview.Confirmation.Confirmation.Pins
		if !hints.interactive {
			p.Fields(
				termui.Field{Label: "Proposal", Value: pins.ProposalID},
				termui.Field{Label: "Digest", Value: pins.ProposalDigest},
				termui.Field{Label: "Base version", Value: pins.BaseVersion},
			)
		}
		if preview.RoutingImpact != nil {
			p.Fields(termui.Field{Label: "Routing impact", Value: preview.RoutingImpact.Summary})
			for _, warning := range preview.RoutingImpact.Warnings {
				p.Warning(warning)
			}
		}
		for _, warning := range preview.Warnings {
			p.Warning(warning.Summary)
		}
		if preview.FullDiff != "" {
			p.Raw(preview.FullDiff)
			if !strings.HasSuffix(preview.FullDiff, "\n") {
				p.Raw("\n")
			}
		}
		previewTail(p, hints.rerun, hints.diffShown, hints.interactive)
	})
}

// skillConfirmCommand renders the confirm command with every pin the
// application layer requires, so a printed next step can be run as shown.
func skillConfirmCommand(pins app.ConfirmationPins) string {
	return fmt.Sprintf("skillstation skill confirm --proposal %s --proposal-digest %s --base-version %s", shellquote.Arg(pins.ProposalID), shellquote.Arg(pins.ProposalDigest), shellquote.Arg(pins.BaseVersion))
}

func writeSkillMutation(stdout, stderr io.Writer, jsonOutput, verbose bool, result app.SkillMutationResult, next string) int {
	return writeResult(stdout, stderr, jsonOutput, result, func(p *termui.Printer) {
		p.Line(result.Summary)
		writeCommitLines(p, result.Commit)
		if verbose {
			p.Fields(
				termui.Field{Label: "Operation", Value: result.OperationID},
				termui.Field{Label: "Catalog snapshot", Value: result.CatalogSnapshot},
				termui.Field{Label: "Generation", Value: result.Generation},
				termui.Field{Label: "Git dirty", Value: fmt.Sprintf("%t", result.GitDirty)},
			)
		}
		if next != "" {
			p.Next(next, "")
		}
	})
}

// nextStepForSkill names what to do after a skill change. A draft gets the
// step the review computes for its current state; every other state ends with
// the commit command `skillstation status` prints for the same hub, unless the
// change was committed already or the commit line named the manual command.
func nextStepForSkill(ctx context.Context, service app.SkillService, result app.SkillMutationResult, workspacePath string) string {
	root := workspacePath
	if resolved, err := skill.ResolveWorkspace(workspacePath); err == nil {
		root = resolved
	}
	commit := "`" + app.GitCommitCommand(root, result.ChangedPaths) + "`"
	summary := result.Summary
	switch {
	case strings.Contains(summary, "Draft skill") && strings.Contains(summary, "saved"):
		if review, err := service.ReviewSkill(ctx, workspacePath, result.SkillID); err == nil {
			return review.NextAction
		}
		return fmt.Sprintf("preview activation with `skillstation skill activate %s`.", result.SkillID)
	case strings.Contains(summary, "active") || strings.Contains(summary, "activated"):
		if result.Commit != nil {
			return fmt.Sprintf("ask your agent to use it, or inspect with `skillstation skill show %s`.", result.SkillID)
		}
		return fmt.Sprintf("ask your agent to use it, or inspect with `skillstation skill show %s`. Commit with %s.", result.SkillID, commit)
	case result.Commit != nil:
		return ""
	default:
		return fmt.Sprintf("commit with %s.", commit)
	}
}

func writeSkillError(stdout, stderr io.Writer, jsonOutput bool, err error) int {
	return writeSkillErrorFor("", stdout, stderr, jsonOutput, err)
}

// writeSkillErrorFor renders err with fix hints that name the skill being changed.
func writeSkillErrorFor(id string, stdout, stderr io.Writer, jsonOutput bool, err error) int {
	if errors.Is(err, context.Canceled) {
		writeStructuredSkillError(stdout, stderr, jsonOutput, app.NewOperationCancelledError())
		return 130
	}
	if errors.Is(err, skill.ErrSnapshotExpired) {
		writeStructuredSkillError(stdout, stderr, jsonOutput, app.NewSnapshotExpiredError(err.Error()))
		return 2
	}
	if errors.Is(err, skill.ErrResourceDigestMismatch) {
		writeStructuredSkillError(stdout, stderr, jsonOutput, app.NewResourceDigestMismatchError(err.Error()))
		return 2
	}
	if refusal := app.UncommittedEditsError(err); refusal != nil {
		writeStructuredSkillError(stdout, stderr, jsonOutput, refusal)
		return 2
	}
	if structured := (*app.Error)(nil); errors.As(err, &structured) && structured.Render.Error != "" {
		// The service already said what happened, why and what to run.
		writeStructuredSkillError(stdout, stderr, jsonOutput, structured)
		return 2
	}
	var missingErr *app.MissingActivationRequirementsError
	if errors.As(err, &missingErr) {
		var editFlags []string
		needsContent := false
		for _, m := range missingErr.Missing {
			switch {
			case strings.Contains(m, "trigger"):
				editFlags = append(editFlags, `--trigger "<when to use>"`)
			case strings.Contains(m, "not_for") || strings.Contains(m, "rationale"):
				editFlags = append(editFlags, `--not-for "<when not to use>"`)
			case strings.Contains(m, "min_scope"):
				editFlags = append(editFlags, `--min-scope single_step`)
			case strings.Contains(m, "content"):
				needsContent = true
			}
		}
		targetID := missingErr.SkillID
		if targetID == "" {
			targetID = id
		}
		why := fmt.Sprintf("skill %s requires: %s", targetID, strings.Join(missingErr.Missing, ", "))
		var fixes []string
		if len(editFlags) > 0 {
			fixes = append(fixes, fmt.Sprintf("`skillstation skill edit %s %s`", targetID, strings.Join(editFlags, " ")))
		}
		if needsContent {
			fixes = append(fixes, fmt.Sprintf("`skillstation skill edit %s --editor` (or `--content-file <file>`) to replace the untouched scaffold instructions", targetID))
		}
		if len(fixes) == 0 {
			fixes = append(fixes, fmt.Sprintf("`skillstation skill review %s`", targetID))
		}
		fix := fmt.Sprintf("Preview the fix with %s, review and confirm it, then preview `skillstation skill activate %s`.", strings.Join(fixes, " and "), targetID)
		return writeInvalidRequest(stdout, stderr, jsonOutput, why, fix)
	}
	writeStructuredSkillError(stdout, stderr, jsonOutput, app.SkillCommandError(err, id))
	return 2
}

func writeStructuredSkillError(stdout, stderr io.Writer, jsonOutput bool, structured *app.Error) {
	result := app.ErrorResult(structured)
	if jsonOutput {
		if err := writeJSON(stdout, result); err != nil {
			p := termui.New(stderr)
			p.Line(err.Error())
		}
		return
	}
	p := termui.New(stderr)
	p.Error(structured.Render.Error, structured.Render.Why, structured.Render.Fix)
}

func runSkillList(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) int {
	query := app.SkillListQuery{State: flags.state, Origin: flags.origin, Search: flags.search, Sort: flags.sortBy}
	if len(flags.labels) == 1 {
		query.Label = flags.labels[0]
	}
	if len(flags.topics) == 1 {
		query.Topic = flags.topics[0]
	}
	result, err := service.ListSkillCatalog(ctx, flags.workspace, query)
	if err != nil {
		return writeSkillError(stdout, stderr, flags.jsonOutput, err)
	}
	if !flags.jsonOutput && !result.UsageAvailable && (flags.sortBy == "uses" || flags.sortBy == "last-used") {
		termui.New(stderr).Line("No usage data yet: usage counts skills loaded through the hub in the last 30 days.")
	}
	return writeResult(stdout, stderr, flags.jsonOutput, result, func(p *termui.Printer) {
		if len(result.Skills) == 0 {
			p.Line("No skills found.")
			return
		}
		headers := []string{"ID", "STATE", "ORIGIN", "USES 30D", "LABELS", "NAME"}
		rows := make([][]string, 0, len(result.Skills))
		for _, entry := range result.Skills {
			rows = append(rows, []string{entry.ID, entry.State, entry.Origin, strconv.FormatInt(entry.Uses30d, 10), strings.Join(entry.Labels, ", "), entry.Name})
		}
		p.Table(headers, rows)
	})
}

// previewSuggestionEdit stores the edit preview of a suggestion. It returns a
// non-zero exit code when the command is over.
func previewSuggestionEdit(ctx context.Context, service app.SkillService, flags skillFlags, stdout, stderr io.Writer) (app.SkillProposal, *app.RoutingSuggestion, int) {
	proposal, err := service.PreviewRoutingSuggestion(ctx, flags.workspace, flags.fromSuggestion)
	if err != nil {
		return app.SkillProposal{}, nil, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, err)
	}
	if proposal.Suggestion.SkillID != flags.id {
		mismatch := app.NewInvalidRequestError(
			fmt.Sprintf("Suggestion %s is for skill %s, not %s.", flags.fromSuggestion, proposal.Suggestion.SkillID, flags.id),
			"Run `skillstation skill edit "+shellquote.Arg(proposal.Suggestion.SkillID)+" --from-suggestion "+shellquote.Arg(flags.fromSuggestion)+"`.")
		return app.SkillProposal{}, nil, writeSkillErrorFor(flags.id, stdout, stderr, flags.jsonOutput, mismatch)
	}
	if proposal.Error != nil {
		if flags.jsonOutput {
			_ = writeJSON(stdout, proposal)
			return app.SkillProposal{}, nil, 2
		}
		p := termui.New(stderr)
		p.Error(proposal.Error.Render.Error, proposal.Error.Render.Why, proposal.Error.Render.Fix)
		return app.SkillProposal{}, nil, 2
	}
	return proposal.SkillProposal, &proposal.Suggestion, 0
}

// writeSuggestionPreview prints a suggestion's reason and evidence before the
// ordinary edit preview. The preview always holds the full diff, so it is shown
// only when --diff asked for it. Phrasings are untrusted: the printer escapes
// them, and the only command printed holds IDs.
func writeSuggestionPreview(stdout, stderr io.Writer, flags skillFlags, preview app.SkillProposal, suggestion app.RoutingSuggestion, interactive bool) int {
	if flags.jsonOutput {
		return writeResult(stdout, stderr, true, app.RoutingSuggestionProposal{SkillProposal: preview, Suggestion: suggestion}, nil)
	}
	p := termui.New(stdout)
	p.Fields(
		termui.Field{Label: "Suggestion", Value: fmt.Sprintf("%s  %s  %s", suggestion.ID, suggestion.Field, strconv.Quote(suggestion.Text))},
		termui.Field{Label: "Why", Value: suggestion.Evidence.Reason},
		termui.Field{Label: "Evidence", Value: suggestionEvidenceLine(suggestion.Evidence)},
	)
	shown := preview
	if !flags.diff {
		shown.FullDiff = ""
	}
	code := writeSkillPreview(stdout, stderr, false, shown, previewHints{rerun: flags.rerun, diffShown: flags.diff, interactive: interactive})
	if code == 0 && !interactive {
		termui.New(stdout).Raw("Or apply this proposal later: " + skillConfirmCommand(preview.Confirmation.Confirmation.Pins) + "\n")
	}
	return code
}

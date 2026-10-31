package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/workspace"
)

// SkillCommitApplyOptions carries the person's answers to the prompts.
type SkillCommitApplyOptions struct {
	// Approve approves the new content of a third-party skill in the same
	// change. Without it a third-party commit is refused, because content
	// nobody approved would stop agents from getting the skill.
	Approve bool
	// Activate activates a new skill right after it is added, in the same commit.
	Activate bool
}

// SkillCommitResult reports what a commit changed on the hub.
type SkillCommitResult struct {
	Result
	SkillID         string                   `json:"skill_id"`
	Case            string                   `json:"case"`
	Collection      string                   `json:"collection,omitempty"`
	OperationID     string                   `json:"operation_id,omitempty"`
	ChangedPaths    []string                 `json:"changed_paths"`
	Files           []SkillCommitFile        `json:"files"`
	Approved        bool                     `json:"approved"`
	Activated       bool                     `json:"activated"`
	CatalogSnapshot string                   `json:"catalog_snapshot,omitempty"`
	Generation      string                   `json:"generation,omitempty"`
	GitDirty        bool                     `json:"git_dirty"`
	Commit          *workspace.CommitOutcome `json:"commit,omitempty"`
	// Checkout is the marker the project folder carries after the commit.
	// It is nil when the commit closed the draft.
	Checkout *CheckoutMarker `json:"checkout,omitempty"`
	// DraftClosed says the project's checkout folder, its Claude Code pointer
	// and the hub's record of it were removed: the hub now holds the work.
	DraftClosed bool `json:"checkout_closed"`
	// LeftOut lists the files the commit left out (such as secrets), which keep
	// the checkout open in the project.
	LeftOut []string `json:"left_out,omitempty"`
}

func commitResultError(err *Error) SkillCommitResult {
	return SkillCommitResult{Result: ErrorResult(err), ChangedPaths: []string{}, Files: []SkillCommitFile{}}
}

// NewContentReviewRequiredError reports that new third-party content needs a
// person's approval before agents may get it.
func NewContentReviewRequiredError(reason, fix string) *Error {
	return &Error{Code: ErrorContentReviewRequired, Render: ErrorRender{Error: "The new content needs approval.", Why: reason, Fix: fix}}
}

// Apply writes a previewed commit to the hub as one mutation, and one Git
// commit when the context asks for one. A third-party skill's new content is
// approved in the same change when options.Approve is set. A new skill can be
// activated in the same commit.
func (service SkillCommitService) Apply(ctx context.Context, path string, preview SkillCommitPreview, options SkillCommitApplyOptions) (SkillCommitResult, error) {
	if service.Clock == nil {
		service.Clock = SystemClock{}
	}
	if preview.Error != nil {
		return commitResultError(preview.Error), nil
	}
	plan := preview.plan
	if plan == nil {
		return commitResultError(NewInvalidRequestError("there is no previewed commit to apply", "Run `skillstation skill commit` again.")), nil
	}
	if len(preview.Conflicts) > 0 {
		return commitResultError(NewInvalidRequestError(
			fmt.Sprintf("%s need a decision before this can be applied", pluralize(len(preview.Conflicts), "file")),
			"Decide each with `--accept <path>=hub` or `--accept <path>=project`, or give your own with `--manual <path>=<file>`.")), nil
	}
	if preview.NothingToCommit {
		return SkillCommitResult{Result: NewResult(StatusOK, nothingToCommitSummary(plan)), SkillID: plan.id, Case: plan.caseName, ChangedPaths: []string{}, Files: []SkillCommitFile{}}, nil
	}
	if len(preview.Uncommitted) > 0 {
		return finishUnfinishedCommit(ctx, plan, preview), nil
	}
	if plan.thirdParty && !options.Approve {
		return commitResultError(NewContentReviewRequiredError(
			fmt.Sprintf("%s is third-party (%s); applying new content without approval would stop agents from getting it", plan.id, plan.origin),
			"Run the commit in a terminal and answer the approval question, or pass `--approve-content "+plan.contentDigest+"` (a person only).")), nil
	}
	approving := options.Approve && plan.thirdParty
	changes, variant, err := approvedChanges(plan, approving)
	if err != nil {
		return SkillCommitResult{}, err
	}
	planned, err := mutation.PlanMutation(plan.root, commitWriteSet(plan, changes, variant))
	if err != nil {
		return SkillCommitResult{}, err
	}
	activate := options.Activate && plan.caseName == commitCaseNew
	message := commitMessage(preview, approving, activate)
	receipt, outcome, activated, activationNote, err := service.writeCommit(ctx, path, plan, planned, message, activate)
	if errors.Is(err, mutation.ErrConflict) {
		return commitResultError(NewStaleProposalError("the hub changed after the preview; nothing was applied", "Run `skillstation skill commit` again.")), nil
	}
	if err != nil {
		return SkillCommitResult{}, err
	}
	noteAgentReach(ctx, plan.root, outcome, plan.id)
	var marker CheckoutMarker
	draftClosed := false
	var leftOut []string
	if plan.submission == nil {
		projectRoot, draftRel, isDraft := draftLocation(plan.folder, plan.id)
		// A change Git refused to commit is finished by committing the draft
		// again, so the draft stays until Git has it.
		if outcome == nil || outcome.Status != workspace.CommitFailed {
			if draftClosed, leftOut, err = closeCommittedDraft(ctx, plan); err != nil {
				return SkillCommitResult{}, err
			}
		}
		if !draftClosed {
			if marker, err = syncProjectFolder(ctx, service.Clock, plan, outcome); err != nil {
				return SkillCommitResult{}, err
			}
			if isDraft {
				_, claude := projectSkillFolders(projectRoot, plan.id)
				record := DraftRecord{SkillID: plan.id, ProjectRoot: projectRoot, Folder: draftRel, BaseVersion: marker.BaseVersion, StartedAt: service.Clock.Now().UTC()}
				if claude.Kind == folderPointer {
					record.Pointer = claude.Rel + "/SKILL.md"
				}
				if err := recordDraft(ctx, plan.root, record); err != nil {
					return SkillCommitResult{}, err
				}
			}
		}
	}
	result := SkillCommitResult{
		Result:          NewResult(StatusApplied, appliedCommitSummary(plan, approving, activated)),
		SkillID:         plan.id,
		Case:            plan.caseName,
		Collection:      plan.collection,
		OperationID:     receipt.OperationID,
		ChangedPaths:    receipt.ChangedPaths,
		Files:           preview.Files,
		Approved:        approving,
		Activated:       activated,
		CatalogSnapshot: receipt.CatalogSnapshot,
		Generation:      receipt.Generation,
		GitDirty:        receipt.GitDirty,
		Commit:          outcome,
	}
	if plan.submission == nil && !draftClosed {
		result.Checkout = &marker
	}
	result.DraftClosed, result.LeftOut = draftClosed, leftOut
	if activationNote != "" {
		result.Warnings = append(result.Warnings, Warning{Code: "activation_failed", Summary: activationNote})
	}
	if outcome != nil {
		result.Items = append(result.Items, commitItem(*outcome, receipt.GitDirty))
	}
	return result, nil
}

// approvedChanges returns the changes to write, with the approval of the new
// content recorded in the same metadata change when approve is set.
func approvedChanges(plan *commitPlan, approve bool) ([]mutation.Change, string, error) {
	if !approve {
		return plan.changes, "plain", nil
	}
	meta := plan.metaAfter
	if meta == nil {
		meta = plan.metaBefore
	}
	approved, err := setMetaApproval(meta, plan.contentDigest)
	if err != nil {
		return nil, "", err
	}
	changes := make([]mutation.Change, 0, len(plan.changes)+1)
	replaced := false
	for _, change := range plan.changes {
		if change.Path == plan.metaPath {
			change.Contents, replaced = approved, true
		}
		changes = append(changes, change)
	}
	if !replaced {
		changes = append(changes, mutation.Change{Path: plan.metaPath, BeforeDigest: digestOf(plan.metaBefore), Contents: approved})
	}
	return changes, "approve:" + plan.contentDigest, nil
}

// writeCommit confirms the planned mutation and, for a new skill that is
// activated at once, chains the activation so both land in one Git commit.
func (service SkillCommitService) writeCommit(ctx context.Context, path string, plan *commitPlan, planned mutation.Proposal, message string, activate bool) (mutation.Receipt, *workspace.CommitOutcome, bool, string, error) {
	if !activate {
		receipt, err := confirmAndPublish(withCommitMessage(ctx, message), plan.root, planned)
		return receipt, commitView(receipt.Commit), false, "", err
	}
	chain, err := beginCommitChain(ctx, plan.root, message, []string{plan.relDir})
	if err != nil {
		return mutation.Receipt{}, nil, false, "", err
	}
	receipt, err := confirmAndPublish(chain.Context(), plan.root, planned)
	if err != nil {
		return receipt, nil, false, "", err
	}
	chain.Record(receipt)
	activated, note := activateInChain(chain, path, plan.id)
	if !activated {
		chain.SetMessage(strings.TrimSuffix(message, ", activate"))
	}
	outcome := chain.Finish()
	if outcome != nil && outcome.Status == workspace.CommitCommitted {
		dirty, dirtyErr := workspace.DirtyPaths(ctx, plan.root, []string{plan.relDir})
		receipt.GitDirty = dirtyErr != nil || len(dirty) > 0
	}
	return receipt, outcome, activated, note, nil
}

// activateInChain activates the skill just added. A skill that cannot be
// activated stays a draft and the reason is reported; it never loses the draft.
func activateInChain(chain *commitChain, path, id string) (bool, string) {
	service := SkillService{}
	preview, err := service.PreviewActivate(chain.Context(), path, id, false)
	if err != nil {
		return false, "Activation was skipped: " + err.Error()
	}
	result, err := service.ConfirmSkillMutation(chain.Context(), path, preview, preview.Confirmation.Confirmation.Pins)
	if err != nil {
		return false, "Activation failed: " + err.Error()
	}
	if result.Error != nil {
		return false, "Activation failed: " + result.Error.Render.Why
	}
	chain.Record(mutation.Receipt{OperationID: result.OperationID, ChangedPaths: result.ChangedPaths})
	return true, ""
}

// withCommitMessage makes the Git commit of this change carry message, when the
// context asks for a commit at all.
func withCommitMessage(ctx context.Context, message string) context.Context {
	policy, asked := workspace.CommitPolicyFrom(ctx)
	if !asked {
		return ctx
	}
	policy.Message = message
	return workspace.WithCommitPolicy(ctx, policy)
}

func commitMessage(preview SkillCommitPreview, approved, activated bool) string {
	id := preview.SkillID
	if submission := preview.plan.submission; submission != nil {
		return submissionCommitMessage(preview, submission.ClientLabel, approved, activated)
	}
	if preview.Case == commitCaseNew {
		message := "Add " + id + " from a project"
		if activated {
			message += ", activate"
		}
		return message
	}
	message := fmt.Sprintf("Commit %s from a project (%s)", id, pluralize(len(preview.Files), "file"))
	if approved {
		message += ", approve content"
	}
	return message
}

func appliedCommitSummary(plan *commitPlan, approved, activated bool) string {
	switch {
	case plan.caseName == commitCaseNew && activated:
		return fmt.Sprintf("Added %s and activated it.", plan.id)
	case plan.caseName == commitCaseNew:
		return fmt.Sprintf("Added draft %s.", plan.id)
	case approved:
		return fmt.Sprintf("Committed %s and approved its content.", plan.id)
	}
	return fmt.Sprintf("Committed %s.", plan.id)
}

// syncProjectFolder brings the project folder to what the hub now has and
// moves its marker to the new base, so the next edit continues from here. A
// folder that was never a checkout gets a marker, and its marker file is kept
// out of the project's Git.
func syncProjectFolder(ctx context.Context, clock Clock, plan *commitPlan, outcome *workspace.CommitOutcome) (CheckoutMarker, error) {
	if err := writeFolderTree(plan.folder, plan.projectTree, plan.final); err != nil {
		return CheckoutMarker{}, err
	}
	marker := CheckoutMarker{
		SkillID:      plan.id,
		Hub:          plan.root,
		BaseVersion:  plan.finalVersion,
		CheckedOutAt: clock.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if outcome != nil && outcome.Status == workspace.CommitCommitted {
		marker.BaseCommit = outcome.Hash
	}
	if err := WriteCheckoutMarker(plan.folder, marker); err != nil {
		return CheckoutMarker{}, err
	}
	if plan.marker == nil {
		if top, inGit := gitTopLevel(ctx, plan.folder); inGit && filepath.Clean(top) != filepath.Clean(plan.folder) {
			if _, err := ensureGitignored(top, CheckoutMarkerName); err != nil {
				return CheckoutMarker{}, err
			}
		}
	}
	return marker, nil
}

// writeFolderTree turns a skill folder that holds current into one that holds
// final: it writes every file that differs and removes every file final lacks.
// Files current does not list, such as secrets a commit leaves out, stay. It
// works inside the folder only, so a symlink in it that points out of the
// folder fails the write instead of being followed.
func writeFolderTree(folder string, current, final map[string]treeFile) error {
	out, err := os.OpenRoot(folder)
	if err != nil {
		return err
	}
	defer out.Close()
	for rel, file := range final {
		if existing, ok := current[rel]; ok && bytes.Equal(existing.Data, file.Data) && existing.Executable == file.Executable {
			continue
		}
		if !safeTreePath(rel) {
			return fmt.Errorf("skill file path %q leaves the skill folder", rel)
		}
		name := filepath.FromSlash(rel)
		if err := out.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if file.Executable {
			mode = 0o755
		}
		if err := out.WriteFile(name, file.Data, mode); err != nil {
			return err
		}
		if err := out.Chmod(name, mode); err != nil {
			return err
		}
	}
	for rel := range current {
		if _, kept := final[rel]; !kept {
			if err := out.Remove(filepath.FromSlash(rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// submissionCommitMessage is the Git subject of a reviewed submission: it names
// the machine the content came from.
func submissionCommitMessage(preview SkillCommitPreview, label string, approved, activated bool) string {
	var message string
	if preview.Case == commitCaseNew {
		message = fmt.Sprintf("Add %s (submitted from %s)", preview.SkillID, label)
	} else {
		message = fmt.Sprintf("Commit %s submitted from %s (%s)", preview.SkillID, label, pluralize(len(preview.Files), "file"))
	}
	if approved {
		message += ", approve content"
	}
	if activated {
		message += ", activate"
	}
	return message
}

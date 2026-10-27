package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
)

// Cases of a commit, in the words of the person committing.
const (
	commitCaseUpdate  = "update"  // checkout, the hub did not change
	commitCaseMerge   = "merge"   // checkout, the hub changed too
	commitCaseNew     = "new"     // a folder the hub has no skill for
	commitCaseReplace = "replace" // a folder that was never checked out, for a skill the hub has
)

// SkillCommitResolution decides one file the three-way merge could not.
type SkillCommitResolution struct {
	Path string `json:"path"`
	// Choice is "hub", "project" or "manual".
	Choice  string `json:"choice"`
	Content []byte `json:"-"`
}

// SkillCommitInput names what to commit and how to settle what needs a choice.
type SkillCommitInput struct {
	// Target is a skill id (read from the project's .claude/skills) or a folder.
	Target string `json:"target"`
	// ProjectDir is the directory the command ran in.
	ProjectDir string `json:"project_dir"`
	// Collection files a new skill; it defaults to "default".
	Collection string `json:"collection,omitempty"`
	// Routing holds explicit routing fields; each one set replaces the
	// suggestion or the hub's value.
	Routing *skill.RoutingInput `json:"routing,omitempty"`
	// Runtime is the runtime block of a new skill; an empty mapping leaves it
	// without one. A skill the hub already has keeps its block, so it is refused.
	Runtime     map[string]any          `json:"runtime,omitempty"`
	Resolutions []SkillCommitResolution `json:"resolutions,omitempty"`
	FullDiff    bool                    `json:"full_diff,omitempty"`
}

// SkillCommitFile is one file of the proposed change.
type SkillCommitFile struct {
	Path string `json:"path"`
	// Change is "A" (added), "M" (modified) or "D" (deleted).
	Change  string `json:"change"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Size    int64  `json:"size"`
	// Status says, for a merge, whose change this is.
	Status string `json:"status,omitempty"`
}

// SkillCommitMerge is one file the hub and the project both have an opinion on.
type SkillCommitMerge struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	// Action is "project", "hub", "merged" or "manual"; empty means the file
	// needs a decision.
	Action string `json:"action,omitempty"`
}

// SkillCommitCheckout identifies the checkout a commit came from.
type SkillCommitCheckout struct {
	Path        string `json:"path"`
	BaseVersion string `json:"base_version"`
}

// SkillCommitTrust says whether the new content needs a person's approval.
type SkillCommitTrust struct {
	ThirdParty bool   `json:"third_party"`
	Origin     string `json:"origin,omitempty"`
	// ContentDigest is the digest of the proposed content, which an approval binds to.
	ContentDigest string `json:"content_digest"`
	// Approved says the hub's current content is approved today.
	Approved bool `json:"approved"`
	// ContentKept says the commit leaves the approved content as it is, for
	// example when only skill.yaml routing or labels change; no new approval
	// is needed then.
	ContentKept bool `json:"content_kept"`
	// NeedsApproval says applying the commit needs a person's approval of the
	// new content: third-party content that changed, or a submission.
	NeedsApproval bool `json:"needs_approval"`
}

// SkillCommitLint is one routing lint finding of the proposed skill.
type SkillCommitLint struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

// SkillCommitPreview is what a commit would change, before anything is written.
type SkillCommitPreview struct {
	Result
	SkillID string `json:"skill_id"`
	// Case is "update", "merge", "new" or "replace".
	Case       string `json:"case"`
	Collection string `json:"collection"`
	State      string `json:"state,omitempty"`
	Folder     string `json:"folder"`
	// ProjectChanged and HubChanged count the files each side changed since
	// the checkout.
	ProjectChanged     int                 `json:"project_changed"`
	HubChanged         int                 `json:"hub_changed"`
	Files              []SkillCommitFile   `json:"files"`
	Excluded           []ExcludedFile      `json:"excluded"`
	Merge              []SkillCommitMerge  `json:"merge"`
	Conflicts          []string            `json:"conflicts,omitempty"`
	RoutingSuggestions *skill.RoutingInput `json:"routing_suggestions,omitempty"`
	// HubChanges lists, newest first, what changed the skill on the hub since
	// the folder was checked out, when the hub knows where that was.
	HubChanges []SkillHubChange `json:"hub_changes,omitempty"`
	// Routing is the routing the hub keeps after the commit, from the folder's
	// skill.yaml and the routing flags. It is set from hub layout 6, where
	// nothing is suggested.
	Routing     *skill.RoutingInput  `json:"routing,omitempty"`
	RoutingLint []SkillCommitLint    `json:"routing_lint"`
	Trust       *SkillCommitTrust    `json:"trust,omitempty"`
	Checkout    *SkillCommitCheckout `json:"checkout,omitempty"`
	TotalBytes  int64                `json:"total_bytes"`
	// ActivationReady says a new skill passes the activation checks with the
	// proposed routing; it is what decides whether "activate now" is offered.
	ActivationReady   bool     `json:"activation_ready"`
	ActivationMissing []string `json:"activation_missing,omitempty"`
	NothingToCommit   bool     `json:"nothing_to_commit"`
	// Uncommitted lists the hub files of a skill that already match the
	// folder but were never committed to Git, such as after a commit Git
	// refused; applying commits them.
	Uncommitted    []string `json:"uncommitted,omitempty"`
	uncommittedOps []string
	FullDiff       string `json:"full_diff,omitempty"`
	// MergeFiles carries the merged text with conflict markers for
	// --write-conflicts.
	MergeFiles []UpstreamFile `json:"-"`
	plan       *commitPlan
}

// commitPlan is the proposed change, ready to be planned as one mutation.
type commitPlan struct {
	root, relDir, metaPath, id, collection, caseName, folder string
	hub                                                      *hubSkill
	final                                                    map[string]treeFile
	changes                                                  []mutation.Change
	metaBefore, metaAfter                                    []byte
	marker                                                   *CheckoutMarker
	projectTree                                              map[string]treeFile
	contentDigest                                            string
	thirdParty                                               bool
	origin                                                   string
	hubVersion                                               string
	finalVersion                                             string
	nonce                                                    string
	// submission is set when the files came from another machine; the folder
	// is then the hub's queue, which nothing is synced back into.
	submission *Submission
}

// hubSkill is a skill as the hub holds it today.
type hubSkill struct {
	collection, relDir, metaPath string
	meta                         []byte
	tree                         map[string]treeFile
	version                      string
}

// SkillCommitService brings a project folder back to the hub.
type SkillCommitService struct {
	Clock Clock
}

func commitPreviewError(err *Error) SkillCommitPreview {
	return SkillCommitPreview{Result: ErrorResult(err), Files: []SkillCommitFile{}, Excluded: []ExcludedFile{}, Merge: []SkillCommitMerge{}, RoutingLint: []SkillCommitLint{}}
}

// Preview plans a commit without writing anything: it reads the folder,
// settles which of the four cases it is, merges when the hub changed, and
// validates the result as the hub would.
func (service SkillCommitService) Preview(ctx context.Context, path string, input SkillCommitInput) (SkillCommitPreview, error) {
	if service.Clock == nil {
		service.Clock = SystemClock{}
	}
	root, err := workspace.Discover(path)
	if err != nil {
		return SkillCommitPreview{}, err
	}
	target, refusal := resolveCommitTarget(ctx, input.Target, input.ProjectDir)
	if refusal != nil {
		return commitPreviewError(refusal), nil
	}
	projectTree, excluded, refusal, err := readProjectFolder(target.dir)
	if err != nil {
		return SkillCommitPreview{}, err
	}
	if refusal != nil {
		return commitPreviewError(refusal), nil
	}
	return service.previewTree(ctx, root, target, folderContents{tree: projectTree, excluded: excluded}, input, nil)
}

// folderContents is a skill folder as read: its files and what was left out.
type folderContents struct {
	tree     map[string]treeFile
	excluded []ExcludedFile
}

// previewTree plans the commit of a folder already read. A submission is the
// same plan over the files another machine sent, with its own checkout base.
func (service SkillCommitService) previewTree(ctx context.Context, root string, target commitTarget, contents folderContents, input SkillCommitInput, submission *Submission) (SkillCommitPreview, error) {
	projectTree, excluded := contents.tree, contents.excluded
	hub, err := loadHubSkill(root, target.id)
	if err != nil {
		return SkillCommitPreview{}, err
	}
	if hub != nil {
		if refusal := normalizeProjectSkillMD(projectTree, target.id, hub); refusal != nil {
			return commitPreviewError(refusal), nil
		}
		if submission != nil {
			keepHubExecutableBits(projectTree, hub.tree)
		}
	}
	plan := &commitPlan{root: root, id: target.id, folder: target.dir, marker: target.marker, hub: hub, projectTree: copyTree(projectTree), nonce: newCommitNonce(), submission: submission}
	preview := SkillCommitPreview{SkillID: target.id, Folder: target.dir, Excluded: excluded, Files: []SkillCommitFile{}, Merge: []SkillCommitMerge{}, RoutingLint: []SkillCommitLint{}, plan: plan}
	if preview.Excluded == nil {
		preview.Excluded = []ExcludedFile{}
	}
	sort.Slice(preview.Excluded, func(i, j int) bool { return preview.Excluded[i].Path < preview.Excluded[j].Path })
	if target.marker != nil && submission == nil {
		preview.Checkout = &SkillCommitCheckout{Path: target.dir, BaseVersion: target.marker.BaseVersion}
		if !sameHub(target.marker.Hub, root) {
			preview.Warnings = append(preview.Warnings, Warning{Code: "other_hub", Summary: fmt.Sprintf("This folder was checked out from the hub at %s; you are committing to %s.", target.marker.Hub, root)})
		}
	}
	var refusal *Error
	if hub == nil {
		refusal, err = service.planNewSkill(&preview, input, projectTree)
	} else {
		refusal, err = service.planExistingSkill(ctx, &preview, input, projectTree)
	}
	if err != nil {
		return SkillCommitPreview{}, err
	}
	if refusal == nil && len(input.Resolutions) > 0 && preview.Case != commitCaseMerge {
		refusal = NewInvalidRequestError("there is nothing to decide: the hub did not change since the checkout", "Run `skillstation skill commit` without --accept or --manual.")
	}
	if refusal != nil {
		return commitPreviewError(refusal), nil
	}
	return service.finishPreview(ctx, preview, input)
}

// keepHubExecutableBits keeps the execute bit of a file the hub already has
// executable: a bundle the hub serves carries no modes, so a machine that
// checked the skill out remotely cannot tell which scripts were executable.
func keepHubExecutableBits(tree, hub map[string]treeFile) {
	for rel, file := range tree {
		if hubFile, ok := hub[rel]; ok && hubFile.Executable && !file.Executable {
			file.Executable = true
			tree[rel] = file
		}
	}
}

// newCommitNonce makes the idempotency key of one preview its own: applying
// the same preview twice replays, but a later commit of content the hub once
// held, and lost again, is never taken for the earlier one.
func newCommitNonce() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(random[:])
}

func copyTree(tree map[string]treeFile) map[string]treeFile {
	clone := make(map[string]treeFile, len(tree))
	for rel, file := range tree {
		clone[rel] = file
	}
	return clone
}

// loadHubSkill reads a skill as the hub holds it, or nil when the hub has none.
func loadHubSkill(root, id string) (*hubSkill, error) {
	collection, relDir, meta, err := locateSkillDir(root, id)
	if errors.Is(err, skill.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tree, err := readHubSkillTree(root, relDir)
	if err != nil {
		return nil, err
	}
	// From layout 6 the whole document is written to the pre-split path and
	// stored as skill.yaml and .meta/hub.yaml by the mutation.
	metaPath := relDir + "/.meta/skill.yaml"
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(metaPath))); statErr != nil && !hubIsSplit(root) {
		metaPath = relDir + "/skill.meta.yaml"
	}
	return &hubSkill{collection: collection, relDir: relDir, metaPath: metaPath, meta: meta, tree: tree, version: skillBaseVersion(tree)}, nil
}

// normalizeProjectSkillMD gives the folder's SKILL.md of an existing skill the
// form the hub keeps: LF line endings and a frontmatter name equal to the id. A
// file that already has that form is left byte for byte, because normalizing
// again would rewrite it and show a change nobody made.
func normalizeProjectSkillMD(tree map[string]treeFile, id string, hub *hubSkill) *Error {
	entry := tree["SKILL.md"]
	name, description, _ := parseSkillMDFrontmatter(entry.Data)
	hubDescription := parseSkillReviewMeta(hub.meta).Description
	if !bytes.Contains(entry.Data, []byte("\r\n")) && name == id && (description != "" || hubDescription == "") {
		return nil
	}
	if description == "" {
		description = hubDescription
	}
	normalized, _, err := ensureImportedSkillFrontmatter(entry.Data, id, description)
	if err != nil {
		return NewInvalidRequestError(err.Error(), "Fix the frontmatter at the top of SKILL.md and run commit again.")
	}
	entry.Data = normalized
	tree["SKILL.md"] = entry
	return nil
}

// planExistingSkill settles the update, merge and replace cases.
func (service SkillCommitService) planExistingSkill(ctx context.Context, preview *SkillCommitPreview, input SkillCommitInput, projectTree map[string]treeFile) (*Error, error) {
	plan := preview.plan
	hub := plan.hub
	if input.Runtime != nil {
		return NewInvalidRequestError("--runtime-file applies to a new skill only", "Change the runtime block of a skill on the hub with `skillstation skill edit "+plan.id+" --runtime-file <yaml>`."), nil
	}
	plan.relDir, plan.metaPath, plan.collection = hub.relDir, hub.metaPath, hub.collection
	plan.metaBefore, plan.hubVersion = hub.meta, hub.version
	preview.Collection, preview.State = hub.collection, parseSkillReviewMeta(hub.meta).Status
	plan.final = projectTree
	switch {
	case plan.marker == nil:
		plan.caseName = commitCaseReplace
		summary := fmt.Sprintf("This folder was not checked out; it replaces the hub copy of %s.", plan.id)
		if plan.submission != nil {
			summary = fmt.Sprintf("It was sent without a checkout of the hub's copy; it replaces the hub copy of %s.", plan.id)
		}
		preview.Warnings = append(preview.Warnings, Warning{Code: "folder_not_checked_out", Summary: summary})
	case plan.marker.BaseVersion == hub.version:
		plan.caseName = commitCaseUpdate
	default:
		plan.caseName = commitCaseMerge
		refusal, err := service.mergeWithHub(ctx, preview, input, projectTree)
		if refusal != nil || err != nil {
			return refusal, err
		}
		preview.HubChanges = hubChangesSince(ctx, plan.root, hub.relDir, plan.marker.BaseCommit)
	}
	preview.Case = plan.caseName
	split := hubIsSplit(plan.root)
	if split {
		if plan.caseName != commitCaseMerge {
			keepHubSettings(plan.final, hub.tree)
		}
	}
	if refusal := service.proposeExistingMeta(preview, input, split); refusal != nil {
		return refusal, nil
	}
	if split {
		plan.changes = changesBetween(plan.relDir, withoutSettings(hub.tree), withoutSettings(plan.final))
	} else {
		plan.changes = changesBetween(plan.relDir, hub.tree, plan.final)
	}
	if !bytes.Equal(plan.metaBefore, plan.metaAfter) {
		plan.changes = append(plan.changes, mutation.Change{Path: plan.metaPath, BeforeDigest: digestOf(plan.metaBefore), Contents: plan.metaAfter})
	}
	return nil, nil
}

// proposeExistingMeta decides the metadata of an update: the hub's own, with
// routing changed only where the person asked, or where the skill had none.
func (service SkillCommitService) proposeExistingMeta(preview *SkillCommitPreview, input SkillCommitInput, split bool) *Error {
	plan := preview.plan
	base := plan.metaBefore
	if split {
		// The folder's skill.yaml is the routing, runtime and labels the
		// person wants; the hub part stays the hub's.
		withFolder, err := metaWithSettings(plan.metaBefore, plan.final[skillmeta.SettingsFile].Data, plan.id)
		if err != nil {
			return NewInvalidRequestError("skill.yaml cannot be read: "+err.Error(), "Fix skill.yaml in the folder and run commit again.")
		}
		base = withFolder
	}
	current := metaRouting(base)
	proposed := current
	// From layout 6 routing comes only from skill.yaml and the flags; a hub
	// before it suggests routing for a skill that has none.
	if !split && len(current.Triggers) == 0 {
		suggested := skill.SuggestRouting(descriptionOf(plan.final["SKILL.md"].Data), string(plan.final["SKILL.md"].Data))
		preview.RoutingSuggestions = &suggested
		proposed = suggested
	}
	if input.Routing != nil {
		proposed = overrideRouting(proposed, *input.Routing)
	}
	plan.metaAfter = base
	if split && !sameRouting(metaRouting(plan.metaBefore), proposed) {
		// Shown only when the commit changes the routing on the hub.
		kept := proposed
		preview.Routing = &kept
	}
	if !sameRouting(current, proposed) {
		meta, err := setMetaRouting(base, proposed)
		if err != nil {
			return NewInvalidRequestError(err.Error(), "Check the routing flags.")
		}
		plan.metaAfter = meta
		if split {
			if err := syncSettingsFile(plan.final, base, meta); err != nil {
				return NewInvalidRequestError(err.Error(), "Check the routing flags.")
			}
		}
	}
	return nil
}

func descriptionOf(skillMD []byte) string {
	_, description, _ := parseSkillMDFrontmatter(skillMD)
	return description
}

func digestOf(data []byte) string { return sourcepkg.Digest(data) }

// changesBetween lists the hub changes that turn the hub's tree into final:
// every added, modified and deleted file, under the skill folder.
func changesBetween(relDir string, hub, final map[string]treeFile) []mutation.Change {
	var changes []mutation.Change
	paths := make([]string, 0, len(final))
	for rel := range final {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		file := final[rel]
		before, existed := hub[rel]
		if existed && bytes.Equal(before.Data, file.Data) {
			continue
		}
		change := mutation.Change{Path: relDir + "/" + rel, Contents: file.Data, Executable: file.Executable}
		if existed {
			change.BeforeDigest = digestOf(before.Data)
		}
		changes = append(changes, change)
	}
	var removed []string
	for rel := range hub {
		if _, kept := final[rel]; !kept {
			removed = append(removed, rel)
		}
	}
	sort.Strings(removed)
	for _, rel := range removed {
		changes = append(changes, mutation.Change{Path: relDir + "/" + rel, BeforeDigest: digestOf(hub[rel].Data), Delete: true})
	}
	return changes
}

// SkillHubChange is one hub commit that changed a skill while a draft of it
// was open.
type SkillHubChange struct {
	Commit  string   `json:"commit"`
	Date    string   `json:"date"`
	Subject string   `json:"subject"`
	Files   []string `json:"files"`
}

// hubChangesSince lists the hub commits that changed the skill in relDir after
// base, newest first. It returns nothing when base is unknown or Git cannot
// tell.
func hubChangesSince(ctx context.Context, root, relDir, base string) []SkillHubChange {
	if base == "" {
		return nil
	}
	output, err := gitOutput(ctx, root, "log", "--format=%H%x09%cs%x09%s", base+"..HEAD", "--", relDir)
	if err != nil {
		return nil
	}
	var changes []SkillHubChange
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		change := SkillHubChange{Commit: fields[0][:min(12, len(fields[0]))], Date: fields[1], Subject: fields[2], Files: []string{}}
		if files, err := gitOutput(ctx, root, "show", "--name-only", "--format=", fields[0], "--", relDir); err == nil {
			for _, file := range strings.Split(strings.TrimSpace(string(files)), "\n") {
				if rel, inside := strings.CutPrefix(file, relDir+"/"); inside && rel != "" {
					change.Files = append(change.Files, rel)
				}
			}
		}
		changes = append(changes, change)
	}
	return changes
}

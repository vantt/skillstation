package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
)

// DraftService lists the drafts projects on this machine hold and lets a
// person discard one. Discarding is a person's action in the web UI: it
// deletes work that was never committed, so it shows that work first.
type DraftService struct{}

// DraftView is one open draft as the web UI lists it.
type DraftView struct {
	SkillID     string    `json:"skill_id"`
	ProjectRoot string    `json:"project_root"`
	Folder      string    `json:"folder"`
	Pointer     string    `json:"pointer,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	// State is "open", or "folder_gone" when the project no longer has the
	// draft folder.
	State string `json:"state"`
	// NewSkill says the hub does not have the skill yet.
	NewSkill bool `json:"new_skill"`
	// ChangedFiles counts the files that differ from the version the draft
	// started from (every file, for a new skill).
	ChangedFiles int `json:"changed_files"`
	// OfficialMoved says the hub's version changed since the draft started.
	OfficialMoved bool `json:"official_moved"`
}

// DraftList is the open drafts, newest first.
type DraftList struct {
	Result
	Drafts []DraftView `json:"checkouts"`
}

// DraftDiscardPreview is what discarding a draft loses.
type DraftDiscardPreview struct {
	Result
	Draft DraftView `json:"checkout"`
	// Unsaved lists the files that differ from the hub and were never
	// committed; discarding loses those changes.
	Unsaved []string `json:"unsaved"`
	// LeftOut lists the files a commit would leave out, such as secrets; they
	// exist only in the draft and are deleted with it.
	LeftOut []string `json:"left_out"`
	// Digest pins the folder as previewed; a discard of a folder that changed
	// since is refused.
	Digest string `json:"digest"`
}

// ErrDraftNotFound means the hub has no record of that draft.
var ErrDraftNotFound = errors.New("the hub has no record of that checkout")

// List returns the drafts the hub knows, newest first.
func (DraftService) List(ctx context.Context, path string) (DraftList, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return DraftList{}, err
	}
	records, err := loadDrafts(root)
	if err != nil {
		return DraftList{}, err
	}
	views := make([]DraftView, 0, len(records))
	for _, record := range records {
		view, _, _ := describeDraft(ctx, root, record)
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].StartedAt.After(views[j].StartedAt) })
	return DraftList{Result: NewResult(StatusOK, fmt.Sprintf("%s open.", pluralize(len(views), "checkout"))), Drafts: views}, nil
}

// PreviewDiscard says what discarding the draft of skillID in projectRoot
// loses, without changing anything.
func (DraftService) PreviewDiscard(ctx context.Context, path, skillID, projectRoot string) (DraftDiscardPreview, error) {
	root, record, err := findDraft(path, skillID, projectRoot)
	if err != nil {
		return DraftDiscardPreview{}, err
	}
	view, tree, err := describeDraft(ctx, root, record)
	if err != nil {
		return DraftDiscardPreview{}, err
	}
	preview := DraftDiscardPreview{Draft: view, Unsaved: []string{}, LeftOut: []string{}}
	if view.State == "folder_gone" {
		preview.Result = NewResult(StatusActionRequired, fmt.Sprintf("The folder of this checkout is gone; discarding removes only the hub's record of %s in %s.", skillID, record.ProjectRoot))
		preview.Digest = draftFolderDigest(record.FolderPath())
		return preview, nil
	}
	hub, err := loadHubSkill(root, skillID)
	if err != nil {
		return DraftDiscardPreview{}, err
	}
	var base map[string]treeFile
	if hub != nil {
		base = hub.tree
	}
	preview.Unsaved = differingFiles(base, tree)
	if preview.LeftOut, err = leftOutFiles(record.FolderPath(), tree); err != nil {
		return DraftDiscardPreview{}, err
	}
	preview.Digest = draftFolderDigest(record.FolderPath())
	preview.Result = NewResult(StatusActionRequired, fmt.Sprintf("Discarding deletes %s and its Claude Code pointer; the hub keeps its own version of %s.", record.FolderPath(), skillID))
	return preview, nil
}

// ConfirmDiscard deletes the draft folder, the Claude Code pointer that points
// to it and the hub's record, after checking the folder is still as
// previewed. It deletes only the draft folder the hub recorded, in the
// project's .agents/skills/<id> or .claude/skills/<id>, and refuses a
// symlinked folder. Nothing on the hub changes.
func (DraftService) ConfirmDiscard(ctx context.Context, path, skillID, projectRoot, digest string) (Result, error) {
	root, record, err := findDraft(path, skillID, projectRoot)
	if err != nil {
		return Result{}, err
	}
	folder := record.FolderPath()
	if draftFolderDigest(folder) != digest {
		return ErrorResult(NewStaleProposalError("the checkout folder changed since you looked at it; nothing was deleted", "Open the checkout again and review what discarding loses.")), nil
	}
	if record.Folder != projectSkillsDir+"/"+skillID && record.Folder != claudeSkillsDir+"/"+skillID {
		return ErrorResult(NewInvalidRequestError("the hub records this checkout in "+record.Folder+", which is not a skill folder Skillstation manages", "Delete the folder by hand if you no longer want it.")), nil
	}
	if info, err := os.Lstat(folder); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return ErrorResult(NewInvalidRequestError(folder+" is not a plain folder, so it was left in place", "Delete it by hand if you no longer want it.")), nil
		}
		if err := os.RemoveAll(folder); err != nil {
			return Result{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	shared, claude := projectSkillFolders(record.ProjectRoot, skillID)
	for _, candidate := range []projectFolder{shared, claude} {
		if candidate.Kind == folderPointer {
			if err := os.Remove(filepath.Join(candidate.Dir, "SKILL.md")); err != nil && !errors.Is(err, os.ErrNotExist) {
				return Result{}, err
			}
			if err := removeEmptyFolders(candidate.Dir); err != nil {
				return Result{}, err
			}
		}
	}
	if err := forgetDraft(ctx, root, skillID, record.ProjectRoot); err != nil {
		return Result{}, err
	}
	return NewResult(StatusApplied, fmt.Sprintf("Discarded the checkout of %s in %s.", skillID, record.ProjectRoot)), nil
}

func findDraft(path, skillID, projectRoot string) (string, DraftRecord, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return "", DraftRecord{}, err
	}
	drafts, err := draftsOfSkill(root, skillID)
	if err != nil {
		return "", DraftRecord{}, err
	}
	for _, record := range drafts {
		if record.ProjectRoot == filepath.Clean(projectRoot) {
			return root, record, nil
		}
	}
	return "", DraftRecord{}, ErrDraftNotFound
}

// describeDraft reads a draft for the list and returns its files.
func describeDraft(ctx context.Context, root string, record DraftRecord) (DraftView, map[string]treeFile, error) {
	view := DraftView{SkillID: record.SkillID, ProjectRoot: record.ProjectRoot, Folder: record.FolderPath(), Pointer: record.Pointer, StartedAt: record.StartedAt, State: "open"}
	tree, _, refusal, err := readProjectFolder(record.FolderPath())
	if errors.Is(err, fs.ErrNotExist) || (err == nil && refusal == nil && len(tree) == 0) {
		view.State = "folder_gone"
		return view, nil, nil
	}
	if err != nil {
		return view, nil, err
	}
	if refusal != nil {
		return view, nil, errors.New(refusal.Render.Error)
	}
	hub, err := loadHubSkill(root, record.SkillID)
	if err != nil {
		return view, tree, err
	}
	if hub == nil {
		view.NewSkill, view.ChangedFiles = true, len(tree)
		return view, tree, nil
	}
	view.OfficialMoved = record.BaseVersion != "" && hub.version != record.BaseVersion
	base := treeBytes(hub.tree)
	if view.OfficialMoved {
		marker, found, _ := ReadCheckoutMarker(record.FolderPath())
		if found {
			if known, ok := findCheckoutBase(ctx, root, hub.relDir, record.SkillID, marker); ok {
				base = known
			}
		}
	}
	view.ChangedFiles = countTreeDifferences(base, tree)
	return view, tree, nil
}

// differingFiles lists the files of tree that differ from base or base lacks,
// and the files base has that tree lacks.
func differingFiles(base, tree map[string]treeFile) []string {
	var out []string
	for rel, file := range tree {
		if before, ok := base[rel]; !ok || string(before.Data) != string(file.Data) {
			out = append(out, rel)
		}
	}
	for rel := range base {
		if _, ok := tree[rel]; !ok {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// draftFolderDigest identifies the files in a folder by path and content, so
// a discard confirms the folder the person saw.
func draftFolderDigest(folder string) string {
	var lines []string
	_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(folder, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines = append(lines, filepath.ToSlash(rel)+"\x00"+sourcepkg.Digest(data))
		return nil
	})
	sort.Strings(lines)
	return sourcepkg.Digest([]byte(strings.Join(lines, "\n")))
}

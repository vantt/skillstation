package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/fsutil"
	"github.com/vantt/skillstation/internal/mutation"
)

// A draft is a skill checked out into a project on this machine: the folder a
// person edits until a commit makes it official or they discard it. The
// project knows its draft from the checkout marker; the hub knows every draft
// from this registry, so it can serve a draft to its project's agents, warn
// other doors that a draft is open, and list drafts in the web UI.
//
// The registry is runtime state of this machine, never part of the hub's Git
// tree. A project on another machine has no entry.

const (
	draftRegistryPath    = "runtime/checkouts.json"
	draftRegistryLock    = "checkouts"
	draftRegistryVersion = 1
)

// DraftRecord is one draft the hub knows about.
type DraftRecord struct {
	SkillID string `json:"skill_id"`
	// ProjectRoot is the absolute, cleaned folder of the project.
	ProjectRoot string `json:"project_root"`
	// Folder is the draft folder relative to ProjectRoot, slash-separated.
	Folder string `json:"folder"`
	// Pointer is the Claude Code pointer file relative to ProjectRoot, if
	// Skillstation wrote one.
	Pointer string `json:"pointer,omitempty"`
	// BaseVersion is the hub version the draft started from; empty for a skill
	// the hub does not have yet.
	BaseVersion string    `json:"base_version,omitempty"`
	StartedAt   time.Time `json:"started_at"`
}

// FolderPath is the absolute draft folder.
func (record DraftRecord) FolderPath() string {
	return filepath.Join(record.ProjectRoot, filepath.FromSlash(record.Folder))
}

type draftRegistryFile struct {
	SchemaVersion int           `json:"schema_version"`
	Drafts        []DraftRecord `json:"checkouts"`
}

// loadDrafts returns the drafts the hub knows, sorted by skill then project.
// A missing registry is empty.
func loadDrafts(root string) ([]DraftRecord, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(draftRegistryPath)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the draft registry: %w", err)
	}
	var file draftRegistryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("read the draft registry %s: %w", draftRegistryPath, err)
	}
	if file.SchemaVersion != draftRegistryVersion {
		return nil, fmt.Errorf("the draft registry %s has schema version %d; this binary reads %d", draftRegistryPath, file.SchemaVersion, draftRegistryVersion)
	}
	sortDrafts(file.Drafts)
	return file.Drafts, nil
}

// updateDrafts changes the registry under its lock and writes it atomically.
func updateDrafts(ctx context.Context, root string, change func([]DraftRecord) []DraftRecord) error {
	lock, err := mutation.AcquireStateLock(ctx, root, draftRegistryLock)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Unlock() }()
	drafts, err := loadDrafts(root)
	if err != nil {
		return err
	}
	drafts = change(drafts)
	sortDrafts(drafts)
	if drafts == nil {
		drafts = []DraftRecord{}
	}
	encoded, err := json.MarshalIndent(draftRegistryFile{SchemaVersion: draftRegistryVersion, Drafts: drafts}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(draftRegistryPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(encoded, '\n'), 0o600)
}

// recordDraft adds a draft, replacing the record of the same skill in the same
// project but keeping when that draft started.
func recordDraft(ctx context.Context, root string, record DraftRecord) error {
	record.ProjectRoot = filepath.Clean(record.ProjectRoot)
	return updateDrafts(ctx, root, func(drafts []DraftRecord) []DraftRecord {
		drafts = slices.DeleteFunc(drafts, func(existing DraftRecord) bool {
			same := existing.SkillID == record.SkillID && existing.ProjectRoot == record.ProjectRoot
			if same && !existing.StartedAt.IsZero() {
				record.StartedAt = existing.StartedAt
			}
			return same
		})
		return append(drafts, record)
	})
}

// forgetDraft removes the record of skillID in projectRoot, if any.
func forgetDraft(ctx context.Context, root, skillID, projectRoot string) error {
	projectRoot = filepath.Clean(projectRoot)
	return updateDrafts(ctx, root, func(drafts []DraftRecord) []DraftRecord {
		return slices.DeleteFunc(drafts, func(existing DraftRecord) bool {
			return existing.SkillID == skillID && existing.ProjectRoot == projectRoot
		})
	})
}

// draftsOfSkill returns the open drafts of one skill.
func draftsOfSkill(root, skillID string) ([]DraftRecord, error) {
	drafts, err := loadDrafts(root)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(drafts, func(record DraftRecord) bool { return record.SkillID != skillID }), nil
}

// draftForFolder finds the draft whose project contains dir: the record of
// skillID with the deepest project root that is dir or one of its parents.
func draftForFolder(drafts []DraftRecord, skillID, dir string) (DraftRecord, bool) {
	dir = filepath.Clean(dir)
	var best DraftRecord
	found := false
	for _, record := range drafts {
		if record.SkillID != skillID || !withinFolder(dir, record.ProjectRoot) {
			continue
		}
		if !found || len(record.ProjectRoot) > len(best.ProjectRoot) {
			best, found = record, true
		}
	}
	return best, found
}

// withinFolder reports whether path is folder or inside it.
func withinFolder(path, folder string) bool {
	rel, err := filepath.Rel(folder, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func sortDrafts(drafts []DraftRecord) {
	slices.SortFunc(drafts, func(a, b DraftRecord) int {
		if c := strings.Compare(a.SkillID, b.SkillID); c != 0 {
			return c
		}
		return strings.Compare(a.ProjectRoot, b.ProjectRoot)
	})
}

// openDraftWarnings warns a door that changes skill id on the hub while a
// project on this machine holds a draft of it: the change reaches the draft
// only when the draft is committed. except is the draft folder the change
// comes from, if any; its own commit is not warned about.
func openDraftWarnings(root, id, except string) []Warning {
	drafts, err := draftsOfSkill(root, id)
	if err != nil {
		return nil
	}
	var warnings []Warning
	for _, record := range drafts {
		if except != "" && sameHub(record.FolderPath(), except) {
			continue
		}
		warnings = append(warnings, Warning{Code: "open_draft", Summary: fmt.Sprintf(
			"%s is checked out in %s since %s; this change will have to be merged into that checkout when it is committed.",
			id, record.ProjectRoot, record.StartedAt.Format("2006-01-02"))})
	}
	return warnings
}

package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// draftLocation says where a committed folder sits when it is a project's
// draft: <project>/.agents/skills/<id> or <project>/.claude/skills/<id>.
func draftLocation(folder, id string) (projectRoot, rel string, ok bool) {
	folder = filepath.Clean(folder)
	if filepath.Base(folder) != id {
		return "", "", false
	}
	parent := filepath.Dir(folder)
	for _, skillsDir := range []string{projectSkillsDir, claudeSkillsDir} {
		suffix := filepath.FromSlash(skillsDir)
		if strings.HasSuffix(parent, string(filepath.Separator)+suffix) {
			root := strings.TrimSuffix(parent, string(filepath.Separator)+suffix)
			return root, skillsDir + "/" + id, true
		}
	}
	return "", "", false
}

// leftOutFiles lists the files in folder, relative and slash-separated, that
// the commit did not read: secrets and build output a commit leaves out. The
// checkout marker is not one of them.
func leftOutFiles(folder string, committed map[string]treeFile) ([]string, error) {
	var left []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == CheckoutMarkerName || rel == CheckoutMarkerName+".tmp" {
			return nil
		}
		if _, ok := committed[rel]; !ok {
			left = append(left, rel)
		}
		return nil
	})
	sort.Strings(left)
	return left, err
}

// closeCommittedDraft closes the draft a commit came from when it is a
// project's draft and holds nothing the commit left out. It returns whether it
// closed the draft and the left-out files that kept it open.
func closeCommittedDraft(ctx context.Context, plan *commitPlan) (bool, []string, error) {
	projectRoot, _, isDraft := draftLocation(plan.folder, plan.id)
	if !isDraft || plan.submission != nil {
		return false, nil, nil
	}
	leftOut, err := leftOutFiles(plan.folder, plan.projectTree)
	if err != nil || len(leftOut) > 0 {
		return false, leftOut, err
	}
	if err := closeDraft(ctx, plan.root, projectRoot, plan.id, plan.folder, plan.projectTree); err != nil {
		return false, nil, err
	}
	return true, nil, nil
}

// closeDraft removes a draft folder whose every file the commit just made
// official, the Claude Code pointer Skillstation wrote for it, and the hub's
// record. It removes only those files and the folders they leave empty, and
// refuses a symlinked folder. The project's .gitignore lines stay; they only
// name the folders.
func closeDraft(ctx context.Context, hubRoot, projectRoot, id, folder string, committed map[string]treeFile) error {
	info, err := os.Lstat(folder)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("the draft folder " + folder + " is not a plain folder, so it was left in place")
	}
	paths := make([]string, 0, len(committed)+2)
	for rel := range committed {
		if !safeTreePath(rel) {
			continue
		}
		paths = append(paths, rel)
	}
	paths = append(paths, CheckoutMarkerName, CheckoutMarkerName+".tmp")
	for _, rel := range paths {
		if err := os.Remove(filepath.Join(folder, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := removeEmptyFolders(folder); err != nil {
		return err
	}
	_, claude := projectSkillFolders(projectRoot, id)
	if claude.Kind == folderPointer {
		if err := os.Remove(filepath.Join(claude.Dir, "SKILL.md")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := removeEmptyFolders(claude.Dir); err != nil {
			return err
		}
	}
	return forgetDraft(ctx, hubRoot, id, projectRoot)
}

// removeEmptyFolders removes folder and the folders below it that hold no
// file, deepest first. A folder that still holds a file stays.
func removeEmptyFolders(folder string) error {
	var dirs []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	slices.SortFunc(dirs, func(a, b string) int { return len(b) - len(a) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

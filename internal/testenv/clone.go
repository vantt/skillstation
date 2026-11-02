package testenv

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RemoveEmptyDirectories removes every empty folder below root except inside
// .git, leaving the tree a fresh clone has: Git stores no empty folders, so a
// cloned hub lacks the ones init created. Folders that hold a file stay.
func RemoveEmptyDirectories(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Deepest first, so a folder that only held empty folders goes too.
	depth := func(dir string) int { return strings.Count(dir, string(filepath.Separator)) }
	sort.Slice(dirs, func(i, j int) bool { return depth(dirs[i]) > depth(dirs[j]) })
	for _, dir := range dirs {
		_ = os.Remove(dir) // fails, and is ignored, while the folder holds a file
	}
	return nil
}

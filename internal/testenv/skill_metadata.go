package testenv

import (
	"os"
	"path/filepath"

	"github.com/vantt/skillstation/internal/skillmeta"
)

// WriteSkillMetadata writes merged metadata of skillDir (workspace-relative)
// below root in the files of the given layout, creating folders as needed, to
// set up a hub fixture. Changes to a live hub go through a mutation.
func WriteSkillMetadata(root, skillDir string, layout int, merged []byte) error {
	files, err := skillmeta.Files(skillDir, layout, merged)
	if err != nil {
		return err
	}
	for _, file := range files {
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, file.Contents, 0o644); err != nil {
			return err
		}
	}
	return nil
}

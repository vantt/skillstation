package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/vantt/skillstation/internal/distill"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

// skillDistillFile is where a skill keeps its distilled knowledge, relative to
// the skill folder.
const skillDistillFile = ".meta/distill.yaml"

// GetSkillDistill returns the distilled knowledge of one skill. The skill is
// found through its metadata, never by a pattern built from the id, so an id
// can name only a skill folder of the hub.
func (SkillService) GetSkillDistill(ctx context.Context, path, id string) (*distill.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := workspace.Discover(path)
	if err != nil {
		return nil, err
	}
	if !skillmeta.ValidSkillID(id) {
		return nil, NewInvalidRequestError("A skill id is a lowercase kebab-case identifier.", "Open the skill from the skill list.")
	}
	loaded, err := skill.Load(root, id)
	skillRelDir := loaded.Dir
	if errors.Is(err, skill.ErrNotFound) {
		return nil, NewNotFoundError(fmt.Sprintf("The hub has no skill %s.", id), "Open the skill from the skill list.")
	}
	if err != nil {
		return nil, err
	}
	doc, err := readSkillDistill(root, skillRelDir)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, NewNotFoundError(fmt.Sprintf("Skill %s has no distilled knowledge yet.", id), "Distill a source into the skill first.")
	}
	return doc, nil
}

// readSkillDistill reads the distilled knowledge of the skill folder
// skillRelDir (the Dir of skill.Load). The read stays inside the hub.
// A skill without the file has none: the result is nil with no error.
func readSkillDistill(root, skillRelDir string) (*distill.Document, error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	data, err := handle.ReadFile(path.Join(skillRelDir, skillDistillFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return distill.UnmarshalDocument(data)
}

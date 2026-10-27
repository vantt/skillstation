package app

import (
	"context"
	"path/filepath"
	"sort"

	catalogpkg "github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skillmeta"
)

// unrecordedHubMetadata is the record doctor --fix gives a skill folder made
// by hand in the hub: a draft nobody has reviewed. The mutation adds its
// dates.
const unrecordedHubMetadata = "schema_version: 1\nstatus: draft\nquality:\n    reviewed: false\n"

// unrecordedSkillDirs lists the skill folders of a layout-6 hub that have a
// SKILL.md but no metadata at all, as skills/<collection>/<id>. Earlier
// layouts are left to their own validation.
func unrecordedSkillDirs(root string) []string {
	read := skillmeta.DirReader(root)
	if skillmeta.HubLayout(read) < skillmeta.SplitLayout {
		return nil
	}
	entrypoints, err := filepath.Glob(filepath.Join(root, "skills", "*", "*", "SKILL.md"))
	if err != nil {
		return nil
	}
	var dirs []string
	for _, entrypoint := range entrypoints {
		rel, err := filepath.Rel(root, filepath.Dir(entrypoint))
		if err != nil {
			continue
		}
		dir := filepath.ToSlash(rel)
		if _, found, err := skillmeta.Locate(read, dir); err == nil && !found {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// recordUnrecordedSkills gives every skill folder made by hand in the hub its
// .meta/hub.yaml, as one change: the skill becomes a draft with the date it
// was recorded. It returns the folders it recorded.
func recordUnrecordedSkills(ctx context.Context, root string) ([]string, error) {
	dirs := unrecordedSkillDirs(root)
	if len(dirs) == 0 {
		return nil, nil
	}
	changes := make([]mutation.Change, 0, len(dirs))
	for _, dir := range dirs {
		changes = append(changes, mutation.Change{Path: dir + "/" + skillmeta.HubFile, Contents: []byte(unrecordedHubMetadata)})
	}
	proposal, err := mutation.PlanMutation(root, mutation.WriteSet{Command: "skill_record", Changes: changes})
	if err != nil {
		return nil, err
	}
	_, err = mutation.ConfirmMutationWithOptions(root, proposal, mutation.Confirmation{ProposalID: proposal.ID, ProposalDigest: proposal.Digest, BaseCatalogSnapshot: proposal.BaseCatalogSnapshot}, mutation.Options{
		Context:       ctx,
		CommitMessage: "Record skills made in the hub folder",
		PostCanonical: func(expected string) (mutation.Publication, error) {
			built, err := catalogpkg.BuildCatalogGenerationWhileLocked(ctx, root, expected, catalogpkg.BuildOptions{})
			if err != nil {
				return mutation.Publication{}, err
			}
			return mutation.Publication{CatalogSnapshot: built.Pointer.CatalogSnapshot, Generation: built.Pointer.Generation}, nil
		},
	})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

// plannedDiff lists the paths a planned mutation adds, modifies and deletes.
// Previews take it from the plan rather than from the changes a command
// built, because planning stores skill metadata in the hub's own files.
func plannedDiff(planned mutation.Proposal) SourceDiff {
	diff := SourceDiff{Added: []string{}, Modified: []string{}, Deleted: []string{}}
	for _, change := range planned.WriteSet.Changes {
		switch {
		case change.Delete:
			diff.Deleted = append(diff.Deleted, change.Path)
		case change.BeforeDigest == "":
			diff.Added = append(diff.Added, change.Path)
		default:
			diff.Modified = append(diff.Modified, change.Path)
		}
	}
	return diff
}

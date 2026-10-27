package app

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/vantt/skillstation/internal/skillmeta"
	"gopkg.in/yaml.v3"
)

// writeSkillMetadata stores metadata in the pre-split shape for the skill
// folder skillDir (workspace-relative, slash-separated) in the files the hub's
// layout uses. A hub without a layout marker gets the current layout.
func writeSkillMetadata(tb testing.TB, root, skillDir, merged string) {
	tb.Helper()
	layout := skillmeta.HubLayout(skillmeta.DirReader(root))
	if layout == 0 {
		layout = skillmeta.SplitLayout
	}
	if err := skillmeta.WriteFiles(root, skillDir, layout, []byte(merged)); err != nil {
		tb.Fatal(err)
	}
}

// readSkillMetadata returns a skill's metadata as one document in the
// pre-split shape.
func readSkillMetadata(tb testing.TB, root, skillDir string) []byte {
	tb.Helper()
	merged, _, found, err := skillmeta.Read(skillmeta.DirReader(root), skillDir)
	if err != nil {
		tb.Fatal(err)
	}
	if !found {
		tb.Fatalf("%s has no metadata", skillDir)
	}
	return merged
}

// updateSkillMetadata rewrites a skill's metadata in the files it is stored in.
func updateSkillMetadata(tb testing.TB, root, skillDir string, mutate func(map[string]any)) {
	tb.Helper()
	merged, source, found, err := skillmeta.Read(skillmeta.DirReader(root), skillDir)
	if err != nil {
		tb.Fatal(err)
	}
	if !found {
		tb.Fatalf("%s has no metadata", skillDir)
	}
	var document map[string]any
	if err := yaml.Unmarshal(merged, &document); err != nil {
		tb.Fatal(err)
	}
	mutate(document)
	encoded, err := yaml.Marshal(document)
	if err != nil {
		tb.Fatal(err)
	}
	if err := skillmeta.WriteFiles(root, skillDir, source.Layout, encoded); err != nil {
		tb.Fatal(err)
	}
}

// writeSkillMetadataAt is writeSkillMetadata for an absolute skill folder
// <root>/skills/<collection>/<id>.
func writeSkillMetadataAt(tb testing.TB, folder, merged string) {
	tb.Helper()
	root, skillDir := splitSkillFolder(tb, folder)
	writeSkillMetadata(tb, root, skillDir, merged)
}

// splitSkillFolder splits an absolute skill folder <root>/skills/<collection>/<id>
// into the hub root and the workspace-relative skill folder.
func splitSkillFolder(tb testing.TB, folder string) (root, skillDir string) {
	tb.Helper()
	collectionDir := filepath.Dir(folder)
	skillsDir := filepath.Dir(collectionDir)
	if filepath.Base(skillsDir) != "skills" {
		tb.Fatalf("%s is not a skill folder of a hub", folder)
	}
	return filepath.Dir(skillsDir), path.Join("skills", filepath.Base(collectionDir), filepath.Base(folder))
}

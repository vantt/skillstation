package app

import (
	"maps"

	"github.com/vantt/skillstation/internal/skillmeta"
)

// From hub layout 6 a skill folder carries its own skill.yaml (routing,
// runtime, labels). A commit takes it from the folder like any other file the
// person edited, while the hub's own metadata stays on the hub. The commit
// writes both through one metadata document, so skill.yaml is kept out of the
// plain file changes.

// hubIsSplit reports whether the hub keeps skill metadata in skill.yaml and
// .meta/hub.yaml.
func hubIsSplit(root string) bool {
	return skillmeta.HubLayout(skillmeta.DirReader(root)) >= skillmeta.SplitLayout
}

// metaWithSettings returns merged metadata whose skill.yaml part is settings,
// keeping the hub part of merged.
func metaWithSettings(merged, settings []byte, id string) ([]byte, error) {
	_, hub, err := skillmeta.Split(merged)
	if err != nil {
		return nil, err
	}
	return skillmeta.Merge(settings, hub, id)
}

// settingsOf returns the skill.yaml part of merged metadata.
func settingsOf(merged []byte) ([]byte, error) {
	settings, _, err := skillmeta.Split(merged)
	return settings, err
}

// withoutSettings returns tree without skill.yaml.
func withoutSettings(tree map[string]treeFile) map[string]treeFile {
	if _, ok := tree[skillmeta.SettingsFile]; !ok {
		return tree
	}
	out := maps.Clone(tree)
	delete(out, skillmeta.SettingsFile)
	return out
}

// keepHubSettings gives a folder without skill.yaml the hub's copy: a folder
// checked out before the hub moved to layout 6 has none, and committing it
// must not erase the skill's routing.
func keepHubSettings(final, hub map[string]treeFile) {
	if _, ok := final[skillmeta.SettingsFile]; ok {
		return
	}
	if file, ok := hub[skillmeta.SettingsFile]; ok {
		final[skillmeta.SettingsFile] = file
	}
}

// syncSettingsFile puts the skill.yaml part of meta into final when it differs
// from what final holds, so the folder shows what the hub will keep.
func syncSettingsFile(final map[string]treeFile, before, meta []byte) error {
	if string(before) == string(meta) {
		return nil
	}
	settings, err := settingsOf(meta)
	if err != nil {
		return err
	}
	final[skillmeta.SettingsFile] = treeFile{Data: settings}
	return nil
}

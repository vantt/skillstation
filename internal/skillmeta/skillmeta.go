// Package skillmeta knows where a hub skill's metadata lives and how its files
// fit together.
//
// From layout 6 a skill keeps what a person edits in skill.yaml beside SKILL.md
// (routing, runtime, labels and classification) and what the hub decides in
// .meta/hub.yaml (status, trust, sources, history, dates). The skill id and
// collection come from the folder, the name and description from SKILL.md.
// Earlier layouts keep everything in .meta/skill.yaml (3 to 5) or
// skill.meta.yaml (before 3).
//
// Most code works on one merged document in the pre-split shape, with the id
// filled in from the folder. Read builds it, Split takes it apart again for
// writing, and comments stay with their keys either way.
package skillmeta

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// SettingsFile is the person-editable file beside SKILL.md.
	SettingsFile = "skill.yaml"
	// HubFile is the hub-owned file, relative to the skill folder.
	HubFile = ".meta/hub.yaml"
	// LegacyFile holds all metadata in layouts 3 to 5.
	LegacyFile = ".meta/skill.yaml"
	// V2File holds all metadata before layout 3.
	V2File = "skill.meta.yaml"

	// SplitLayout is the first hub layout with skill.yaml and .meta/hub.yaml.
	SplitLayout = 6
)

// settingsKeys are the top-level keys that belong in skill.yaml.
var settingsKeys = map[string]bool{
	"routing": true, "topics": true, "technologies": true, "domain": true,
	"aliases": true, "runtime": true, "labels": true,
}

// derivedKeys are no longer stored from layout 6: the folder gives the id and
// collection, SKILL.md the name and description.
var derivedKeyList = []string{"id", "collection", "collection_id", "name", "description"}

var derivedKeys = func() map[string]bool {
	keys := make(map[string]bool, len(derivedKeyList))
	for _, key := range derivedKeyList {
		keys[key] = true
	}
	return keys
}()

// SettingsKeys returns the top-level keys that belong in skill.yaml.
func SettingsKeys() []string {
	return []string{"schema_version", "routing", "topics", "technologies", "domain", "aliases", "runtime", "labels"}
}

// HubKeys returns the top-level keys that belong in .meta/hub.yaml.
func HubKeys() []string {
	return []string{"schema_version", "status", "quality", "audit", "sources", "provenance", "history", "created_at", "updated_at", "content"}
}

// MetadataKeys returns every top-level key of the merged metadata document:
// the skill.yaml and .meta/hub.yaml keys plus the derived keys that earlier
// layouts still store.
func MetadataKeys() []string {
	seen := map[string]bool{}
	var keys []string
	for _, list := range [][]string{SettingsKeys(), HubKeys(), derivedKeyList} {
		for _, key := range list {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// ValidSkillID reports whether id has the form of a skill or collection id:
// a lowercase letter or digit, then up to 62 lowercase letters, digits or
// hyphens. The id names a folder, so the form also keeps it a safe path part.
func ValidSkillID(id string) bool {
	if len(id) == 0 || len(id) > 63 || id[0] == '-' {
		return false
	}
	for index := 0; index < len(id); index++ {
		char := id[index]
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

// IsSettingsKey reports whether a top-level metadata key belongs in skill.yaml.
func IsSettingsKey(key string) bool { return settingsKeys[key] }

// ReadFunc reads a workspace-relative, slash-separated path. A missing file
// returns an error matching fs.ErrNotExist.
type ReadFunc func(rel string) ([]byte, error)

// DirReader reads paths below root.
func DirReader(root string) ReadFunc {
	return func(rel string) ([]byte, error) {
		return os.ReadFile(root + "/" + rel)
	}
}

// Source says where a skill's metadata was read from.
type Source struct {
	// Layout is SplitLayout for skill.yaml plus .meta/hub.yaml, 3 for
	// .meta/skill.yaml and 2 for skill.meta.yaml.
	Layout int
	// Paths are the workspace-relative files read, the anchor file first.
	Paths []string
}

// Anchor returns the file whose presence registers the skill.
func (source Source) Anchor() string {
	if len(source.Paths) == 0 {
		return ""
	}
	return source.Paths[0]
}

// Split reports whether the metadata uses skill.yaml and .meta/hub.yaml.
func (source Source) Split() bool { return source.Layout >= SplitLayout }

// AnchorPaths returns the files that register a skill in skillDir, in the
// order they are looked for.
func AnchorPaths(skillDir string) []string {
	return []string{skillDir + "/" + HubFile, skillDir + "/" + LegacyFile, skillDir + "/" + V2File}
}

// Locate finds which metadata files a skill folder uses. found is false when
// the folder has none.
func Locate(read ReadFunc, skillDir string) (Source, bool, error) {
	candidates := AnchorPaths(skillDir)
	if marker, err := read(".skillstation/schema-version"); err == nil {
		// Before layout 3 skill.meta.yaml comes first.
		var version int
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(string(marker)), "%d", &version); scanErr == nil && version < 3 {
			candidates = []string{candidates[2], candidates[1]}
		}
	}
	for _, candidate := range candidates {
		if _, err := read(candidate); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return Source{}, false, err
		}
		switch {
		case strings.HasSuffix(candidate, "/"+HubFile):
			return Source{Layout: SplitLayout, Paths: []string{candidate, skillDir + "/" + SettingsFile}}, true, nil
		case strings.HasSuffix(candidate, "/"+LegacyFile):
			return Source{Layout: 3, Paths: []string{candidate}}, true, nil
		default:
			return Source{Layout: 2, Paths: []string{candidate}}, true, nil
		}
	}
	return Source{}, false, nil
}

// Read returns a skill's metadata as one document in the pre-split shape. For
// a split skill the id comes from the folder name and a missing skill.yaml
// counts as empty. found is false when the folder has no metadata.
func Read(read ReadFunc, skillDir string) ([]byte, Source, bool, error) {
	source, found, err := Locate(read, skillDir)
	if err != nil || !found {
		return nil, source, found, err
	}
	anchor, err := read(source.Anchor())
	if err != nil {
		return nil, source, false, err
	}
	if !source.Split() {
		return anchor, source, true, nil
	}
	settings, err := read(source.Paths[1])
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, source, false, err
	}
	merged, err := Merge(settings, anchor, path.Base(skillDir))
	if err != nil {
		return nil, source, false, err
	}
	return merged, source, true, nil
}

// Merge combines skill.yaml and .meta/hub.yaml into one document in the
// pre-split shape, with schema_version 1 and the id first. Either input may be
// empty.
func Merge(settings, hub []byte, id string) ([]byte, error) {
	hubMap, err := mappingOf(hub, HubFile)
	if err != nil {
		return nil, err
	}
	settingsMap, err := mappingOf(settings, SettingsFile)
	if err != nil {
		return nil, err
	}
	merged := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	merged.Content = append(merged.Content, scalarPair("schema_version", "1", "!!int")...)
	merged.Content = append(merged.Content, scalarPair("id", id, "!!str")...)
	for _, mapping := range []*yaml.Node{hubMap, settingsMap} {
		for index := 0; index+1 < len(mapping.Content); index += 2 {
			key := mapping.Content[index].Value
			if key == "schema_version" || key == "id" {
				continue
			}
			merged.Content = append(merged.Content, mapping.Content[index], mapping.Content[index+1])
		}
	}
	return Encode(merged)
}

// Split takes a document in the pre-split shape apart into skill.yaml and
// .meta/hub.yaml contents. Derived keys (id, collection, name, description)
// are dropped; any other key that is not a settings key goes to hub.yaml, where
// validation reports it if it is unknown.
func Split(merged []byte) (settings, hub []byte, err error) {
	mapping, err := mappingOf(merged, "skill metadata")
	if err != nil {
		return nil, nil, err
	}
	settingsMap := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: scalarPair("schema_version", "1", "!!int")}
	hubMap := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: scalarPair("schema_version", "1", "!!int")}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key := mapping.Content[index].Value
		switch {
		case key == "schema_version" || derivedKeys[key]:
		case settingsKeys[key]:
			settingsMap.Content = append(settingsMap.Content, mapping.Content[index], mapping.Content[index+1])
		default:
			hubMap.Content = append(hubMap.Content, mapping.Content[index], mapping.Content[index+1])
		}
	}
	if settings, err = Encode(settingsMap); err != nil {
		return nil, nil, err
	}
	if hub, err = Encode(hubMap); err != nil {
		return nil, nil, err
	}
	return settings, hub, nil
}

// File is one metadata file to write, by workspace-relative path.
type File struct {
	Path     string
	Contents []byte
}

// Files returns the files that store merged metadata for skillDir in the given
// layout: skill.yaml and .meta/hub.yaml from SplitLayout, the single legacy
// file before it.
func Files(skillDir string, layout int, merged []byte) ([]File, error) {
	switch {
	case layout >= SplitLayout:
		settings, hub, err := Split(merged)
		if err != nil {
			return nil, err
		}
		return []File{{Path: skillDir + "/" + HubFile, Contents: hub}, {Path: skillDir + "/" + SettingsFile, Contents: settings}}, nil
	case layout >= 3:
		return []File{{Path: skillDir + "/" + LegacyFile, Contents: merged}}, nil
	default:
		return []File{{Path: skillDir + "/" + V2File, Contents: merged}}, nil
	}
}

// HubLayout reads the hub's layout version from .skillstation/schema-version.
// A missing or unreadable marker counts as 0.
func HubLayout(read ReadFunc) int {
	contents, err := read(".skillstation/schema-version")
	if err != nil {
		return 0
	}
	var version int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(contents)), "%d", &version); err != nil {
		return 0
	}
	return version
}

// SkillDirOf returns skills/<collection>/<id> for a path inside a skill folder.
func SkillDirOf(rel string) (string, bool) {
	parts := strings.Split(rel, "/")
	if len(parts) < 4 || parts[0] != "skills" {
		return "", false
	}
	return strings.Join(parts[:3], "/"), true
}

func mappingOf(contents []byte, label string) (*yaml.Node, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	if document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse %s: the document must be a mapping", label)
	}
	return document.Content[0], nil
}

func scalarPair(key, value, tag string) []*yaml.Node {
	return []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		{Kind: yaml.ScalarNode, Tag: tag, Value: value},
	}
}

// Encode writes a mapping node as a YAML document with the hub's indent.
func Encode(mapping *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(4)
	if err := encoder.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WriteFiles writes merged metadata of skillDir (workspace-relative) below root
// in the files of the given layout, creating folders as needed. It is for
// setting up a hub, such as in tests; changes to a live hub go through a
// mutation.
func WriteFiles(root, skillDir string, layout int, merged []byte) error {
	files, err := Files(skillDir, layout, merged)
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

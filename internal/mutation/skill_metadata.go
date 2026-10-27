package mutation

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/skillmeta"
)

// ErrUpstreamShipsSkillYAML stops a write that would put a source's own
// skill.yaml over the one the hub keeps for a skill's routing, runtime and
// labels.
var ErrUpstreamShipsSkillYAML = errors.New("the source ships its own skill.yaml")

// metadataClock is replaced in tests.
var metadataClock = time.Now

// storeSkillMetadata applies the hub's rules for skill metadata to a write on
// a hub at layout 6 or later; earlier layouts are left as they are.
//
// A change that carries a skill's whole metadata document at the pre-split
// path (.meta/skill.yaml or skill.meta.yaml) becomes changes to skill.yaml and
// .meta/hub.yaml; its pinned before-digest is checked against the current
// merged metadata. Every skill whose files the write touches gets its dates in
// .meta/hub.yaml: created_at when the skill is new, updated_at now. Files under
// .meta/ other than hub.yaml (such as distill.yaml) are not a change to the
// skill and change no date.
func storeSkillMetadata(root string, changes []Change) ([]Change, error) {
	read := skillmeta.DirReader(root)
	if skillmeta.HubLayout(read) < skillmeta.SplitLayout {
		return changes, nil
	}
	written := map[string]int{}
	for index, change := range changes {
		written[change.Path] = index
	}
	out := make([]Change, 0, len(changes)+1)
	for _, change := range changes {
		skillDir, ok := legacyMetadataDir(change.Path)
		if ok {
			if _, err := read(change.Path); err == nil {
				// A stray file of an earlier layout is written or deleted as it is.
				ok = false
			}
		}
		if !ok {
			out = append(out, change)
			continue
		}
		converted, err := splitLegacyChange(read, skillDir, change, written)
		if err != nil {
			return nil, err
		}
		out = append(out, converted...)
	}
	return stampSkillDates(read, out, metadataClock())
}

func splitLegacyChange(read skillmeta.ReadFunc, skillDir string, change Change, written map[string]int) ([]Change, error) {
	current, source, found, err := skillmeta.Read(read, skillDir)
	if err != nil {
		return nil, err
	}
	if found && !source.Split() {
		return nil, fmt.Errorf("skill %s still uses the metadata layout before 6; run `skillstation doctor --fix` first", skillDir)
	}
	if change.BeforeDigest != "" {
		actual := ""
		if found {
			actual = digest(current)
		}
		if actual != change.BeforeDigest {
			return nil, fmt.Errorf("%w: %s changed since it was read", ErrConflict, skillDir)
		}
	}
	if change.Delete {
		var out []Change
		for _, path := range []string{skillDir + "/" + skillmeta.HubFile, skillDir + "/" + skillmeta.SettingsFile} {
			if _, err := read(path); err == nil {
				out = append(out, Change{Path: path, Delete: true})
			}
		}
		return out, nil
	}
	if _, clash := written[skillDir+"/"+skillmeta.SettingsFile]; clash {
		return nil, fmt.Errorf("%w (%s); remove it from the source or add the skill from a copy without it", ErrUpstreamShipsSkillYAML, skillDir)
	}
	files, err := skillmeta.Files(skillDir, skillmeta.SplitLayout, change.Contents)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", change.Path, err)
	}
	out := make([]Change, 0, len(files))
	for _, file := range files {
		// A file whose values do not change keeps its bytes, so a write to the
		// hub's part never reformats the person's skill.yaml or drops its
		// comments.
		if current, err := read(file.Path); err == nil && sameYAML(current, file.Contents) {
			continue
		}
		out = append(out, Change{Path: file.Path, Contents: file.Contents})
	}
	return out, nil
}

// sameYAML reports whether two YAML documents hold the same values.
func sameYAML(a, b []byte) bool {
	var left, right any
	if yaml.Unmarshal(a, &left) != nil || yaml.Unmarshal(b, &right) != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

// stampSkillDates writes created_at and updated_at into .meta/hub.yaml of
// every skill the changes touch.
func stampSkillDates(read skillmeta.ReadFunc, changes []Change, now time.Time) ([]Change, error) {
	hubIndex := map[string]int{}
	touched := map[string]bool{}
	for index, change := range changes {
		skillDir, ok := skillmeta.SkillDirOf(change.Path)
		if !ok {
			continue
		}
		rel := strings.TrimPrefix(change.Path, skillDir+"/")
		switch {
		case rel == skillmeta.HubFile:
			hubIndex[skillDir] = index
			if !change.Delete {
				touched[skillDir] = true
			}
		case strings.HasPrefix(rel, ".meta/"):
		default:
			touched[skillDir] = true
		}
	}
	dirs := make([]string, 0, len(touched))
	for dir := range touched {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	stamp := now.UTC().Format(time.RFC3339)
	for _, dir := range dirs {
		hubPath := dir + "/" + skillmeta.HubFile
		current, err := read(hubPath)
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		index, changing := hubIndex[dir]
		next := current
		if changing {
			next = changes[index].Contents
		} else if !exists {
			// Content of a folder the hub has not recorded: the hub records it
			// when it is added, not as a side effect of another write.
			continue
		}
		stamped, err := stampHub(next, current, exists, stamp)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", hubPath, err)
		}
		if changing {
			changes[index].Contents = stamped
		} else {
			changes = append(changes, Change{Path: hubPath, Contents: stamped})
		}
	}
	return changes, nil
}

// stampHub sets the dates in hub.yaml contents: created_at kept from the stored
// file or set now for a new skill, updated_at now. Key order and comments of
// next are kept.
func stampHub(next, current []byte, exists bool, stamp string) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(next, &document); err != nil {
		return nil, err
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("hub metadata must be one YAML mapping")
	}
	mapping := document.Content[0]
	// created_at is the stored date. A new skill gets now, or the date its
	// writer gave; a skill recorded before the hub kept dates stays without one.
	created := ""
	if exists {
		var stored struct {
			CreatedAt string `yaml:"created_at"`
		}
		if err := yaml.Unmarshal(current, &stored); err == nil {
			created = stored.CreatedAt
		}
	} else if value := mappingValue(mapping, "created_at"); value != nil && value.Value != "" {
		created = value.Value
	} else {
		created = stamp
	}
	removeMappingKey(mapping, "created_at")
	removeMappingKey(mapping, "updated_at")
	if created != "" {
		setMappingString(mapping, "created_at", created)
	}
	setMappingString(mapping, "updated_at", stamp)
	return skillmeta.Encode(mapping)
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func removeMappingKey(mapping *yaml.Node, key string) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
			return
		}
	}
}

func setMappingString(mapping *yaml.Node, key, value string) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: yaml.DoubleQuotedStyle}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content[index+1] = node
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, node)
}

// legacyMetadataDir returns the skill folder of a skills/<c>/<id>/.meta/skill.yaml
// or skills/<c>/<id>/skill.meta.yaml path.
func legacyMetadataDir(path string) (string, bool) {
	for _, suffix := range []string{"/" + skillmeta.LegacyFile, "/" + skillmeta.V2File} {
		if dir, ok := strings.CutSuffix(path, suffix); ok && strings.HasPrefix(path, "skills/") && strings.Count(dir, "/") == 2 {
			return dir, true
		}
	}
	return "", false
}

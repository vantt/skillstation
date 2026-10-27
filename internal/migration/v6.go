package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skillmeta"
)

// ErrMetadataDisagrees stops the move to layout 6 when a field the new layout
// no longer stores holds a value its source does not: the id or collection
// differs from the folder, or the name or description from SKILL.md. Moving
// on would lose that value, so the person decides first.
var ErrMetadataDisagrees = errors.New("skill metadata disagrees with its source")

// Disagreement is one value the move to layout 6 would lose.
type Disagreement struct {
	Skill  string `json:"skill"`
	Path   string `json:"path"`
	Field  string `json:"field"`
	Stored string `json:"stored"`
	Source string `json:"source"`
	Fix    string `json:"fix"`
}

// DisagreementError lists every value the move would lose. Nothing is changed.
type DisagreementError struct {
	Items []Disagreement
}

func (e *DisagreementError) Error() string {
	lines := make([]string, 0, len(e.Items)+1)
	lines = append(lines, fmt.Sprintf("%d value(s) in skill metadata disagree with their source; nothing was changed:", len(e.Items)))
	for _, item := range e.Items {
		lines = append(lines, fmt.Sprintf("- %s: %s is %q in %s but %s. %s", item.Skill, item.Field, item.Stored, item.Path, item.Source, item.Fix))
	}
	return strings.Join(lines, "\n")
}

func (e *DisagreementError) Unwrap() error { return ErrMetadataDisagrees }

// planV5ToV6 moves every skill's .meta/skill.yaml into skill.yaml (what a
// person edits) and .meta/hub.yaml (what the hub decides), and drops the id,
// collection, name and description, which the folder and SKILL.md already
// give. Other files under .meta/ are not touched. When earlier steps of the
// same migration planned a .meta/skill.yaml (a hub coming from layout 2), the
// planned contents are split instead of the files on disk.
func planV5ToV6(root string, pending map[string]mutation.Change) ([]mutation.Change, []FileDiff, error) {
	markerRel := ".skillstation/schema-version"
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(markerRel)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("read schema-version marker: %w", err)
	}
	marker := []byte("6\n")
	changes := []mutation.Change{{Path: markerRel, Contents: marker}}
	diffs := []FileDiff{{Path: markerRel, Before: string(before), After: string(marker), Diff: fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -1 +1 @@\n-%s+%s", markerRel, markerRel, before, marker)}}

	metaFiles, err := filepath.Glob(filepath.Join(root, "skills", "*", "*", ".meta", "skill.yaml"))
	if err != nil {
		return nil, nil, err
	}
	sources := map[string]bool{}
	for _, metaFile := range metaFiles {
		rel, err := filepath.Rel(root, metaFile)
		if err != nil {
			return nil, nil, err
		}
		sources[filepath.ToSlash(rel)] = true
	}
	for path, change := range pending {
		if dir, ok := strings.CutSuffix(path, "/"+skillmeta.LegacyFile); ok && strings.HasPrefix(path, "skills/") && strings.Count(dir, "/") == 2 {
			sources[path] = !change.Delete
		}
	}
	rels := make([]string, 0, len(sources))
	for rel, keep := range sources {
		if keep {
			rels = append(rels, rel)
		}
	}
	sort.Strings(rels)
	read := func(rel string) ([]byte, bool, error) {
		if change, ok := pending[rel]; ok && !change.Delete {
			return change.Contents, false, nil
		}
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return contents, err == nil, err
	}
	var disagreements []Disagreement
	for _, rel := range rels {
		skillDir := strings.TrimSuffix(rel, "/"+skillmeta.LegacyFile)
		skillChanges, skillDiffs, found, err := splitSkillMetadata(read, skillDir, rel)
		if err != nil {
			return nil, nil, err
		}
		disagreements = append(disagreements, found...)
		changes = append(changes, skillChanges...)
		diffs = append(diffs, skillDiffs...)
	}
	if len(disagreements) > 0 {
		return nil, nil, &DisagreementError{Items: disagreements}
	}
	return changes, diffs, nil
}

// readPlanned returns a file as the migration so far leaves it, and whether
// those bytes are the file on disk.
type readPlanned func(rel string) ([]byte, bool, error)

func splitSkillMetadata(read readPlanned, skillDir, metaRel string) ([]mutation.Change, []FileDiff, []Disagreement, error) {
	contents, onDisk, err := read(metaRel)
	if err != nil {
		return nil, nil, nil, err
	}
	parts := strings.Split(skillDir, "/")
	collection, id := parts[1], parts[2]
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s: %w", metaRel, err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, nil, nil, fmt.Errorf("%s is not one YAML mapping", metaRel)
	}
	values := map[string]string{}
	mapping := document.Content[0]
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if value := mapping.Content[index+1]; value.Kind == yaml.ScalarNode {
			values[mapping.Content[index].Value] = strings.TrimSpace(value.Value)
		}
	}
	skillMD, _, err := read(skillDir + "/SKILL.md")
	if err != nil {
		return nil, nil, nil, err
	}
	frontmatter := skillFrontmatter(skillMD)
	var found []Disagreement
	disagree := func(field, stored, source, fix string) {
		found = append(found, Disagreement{Skill: id, Path: metaRel, Field: field, Stored: stored, Source: source, Fix: fix})
	}
	if stored, ok := values["id"]; ok && stored != id {
		disagree("id", stored, fmt.Sprintf("the folder is %q", id), "Rename the folder or correct the id, then run the migration again.")
	}
	for _, field := range []string{"collection", "collection_id"} {
		if stored, ok := values[field]; ok && stored != "" && stored != collection {
			disagree(field, stored, fmt.Sprintf("the skill is in the collection folder %q", collection), "Move the skill folder into skills/"+stored+"/ or remove the "+field+" line, then run the migration again.")
		}
	}
	if stored, ok := values["name"]; ok && stored != "" && stored != id && stored != frontmatter["name"] {
		disagree("name", stored, fmt.Sprintf("the skill's name is its id %q", id), "Remove the name line from "+metaRel+" (the name is the id from now on), then run the migration again.")
	}
	if stored, ok := values["description"]; ok && stored != "" && stored != frontmatter["description"] {
		source := "SKILL.md has no description"
		if frontmatter["description"] != "" {
			source = fmt.Sprintf("SKILL.md says %q", frontmatter["description"])
		}
		disagree("description", stored, source, "Put the description you want in the SKILL.md frontmatter and remove the description line from "+metaRel+", then run the migration again.")
	}
	settingsRel := skillDir + "/" + skillmeta.SettingsFile
	if existing, _, _ := read(settingsRel); existing != nil {
		disagree("skill.yaml", settingsRel, "layout 6 keeps the skill's routing, runtime and labels in that file", "Rename or remove "+settingsRel+" (it came with the skill's content), then run the migration again.")
	}
	hubRel := skillDir + "/" + skillmeta.HubFile
	if existing, _, _ := read(hubRel); existing != nil {
		disagree("hub.yaml", hubRel, "layout 6 keeps the hub's own record of the skill in that file", "Rename or remove "+hubRel+", then run the migration again.")
	}
	if len(found) > 0 {
		return nil, nil, found, nil
	}
	settings, hub, err := skillmeta.Split(contents)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", metaRel, err)
	}
	removal := mutation.Change{Path: metaRel, Delete: true}
	if onDisk {
		sum := sha256.Sum256(contents)
		removal.BeforeDigest = "sha256:" + hex.EncodeToString(sum[:])
	}
	changes := []mutation.Change{
		removal,
		{Path: hubRel, Contents: hub},
		{Path: settingsRel, Contents: settings},
	}
	diffs := []FileDiff{
		{Path: metaRel, Before: string(contents), Diff: fmt.Sprintf("--- a/%s\n+++ /dev/null\n@@ moved into %s and %s @@\n", metaRel, skillmeta.SettingsFile, skillmeta.HubFile)},
		{Path: hubRel, After: string(hub), Diff: newFileDiff(hubRel, hub)},
		{Path: settingsRel, After: string(settings), Diff: newFileDiff(settingsRel, settings)},
	}
	return changes, diffs, nil, nil
}

func newFileDiff(rel string, contents []byte) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "--- /dev/null\n+++ b/%s\n", rel)
	for _, line := range strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n") {
		builder.WriteString("+" + line + "\n")
	}
	return builder.String()
}

// skillFrontmatter returns the name and description in a SKILL.md
// frontmatter, empty when there is none or the field is missing.
func skillFrontmatter(contents []byte) map[string]string {
	result := map[string]string{}
	contents = bytes.ReplaceAll(contents, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(contents, []byte("---\n")) {
		return result
	}
	end := bytes.Index(contents[4:], []byte("\n---"))
	if end < 0 {
		return result
	}
	var fields map[string]any
	if yaml.Unmarshal(contents[4:4+end], &fields) != nil {
		return result
	}
	for _, key := range []string{"name", "description"} {
		if value, ok := fields[key].(string); ok {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

package migration

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/distill"
	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skillmeta"
	sourcepkg "github.com/vantt/skillstation/internal/source"
)

// linkDirectory held one file per skill-to-source link up to layout 6.
const linkDirectory = "sources/skills"

// ErrSourceLinksUnmovable stops the move to layout 7 when a link file holds
// something the sources block of a skill's distill.yaml cannot: a link to a
// skill or a source the hub no longer has, a role other than learning, or a
// field the block has no place for. Moving on would lose it, so the person
// decides first.
var ErrSourceLinksUnmovable = errors.New("skill-to-source links cannot be moved")

// SourceLinkProblem is one link file the move to layout 7 cannot carry over.
type SourceLinkProblem struct {
	Path    string `json:"path"`
	Problem string `json:"problem"`
	Fix     string `json:"fix"`
}

// SourceLinkError lists every link file the move cannot carry over. Nothing
// is changed.
type SourceLinkError struct {
	Items []SourceLinkProblem
}

func (e *SourceLinkError) Error() string {
	lines := make([]string, 0, len(e.Items)+1)
	lines = append(lines, fmt.Sprintf("%d skill-to-source link file(s) cannot be moved into distill.yaml; nothing was changed:", len(e.Items)))
	for _, item := range e.Items {
		lines = append(lines, fmt.Sprintf("- %s: %s %s", item.Path, item.Problem, item.Fix))
	}
	return strings.Join(lines, "\n")
}

func (e *SourceLinkError) Unwrap() error { return ErrSourceLinksUnmovable }

// linkFile is the content of a link file of layout 6.
type linkFile struct {
	SchemaVersion int    `yaml:"schema_version"`
	ID            string `yaml:"id"`
	SkillID       string `yaml:"skill_id"`
	SourceID      string `yaml:"source_id"`
	Role          string `yaml:"role"`
}

// movedLink is a link file and the skill folder whose sources block takes it.
type movedLink struct {
	rel      string
	contents []byte
	skillDir string
	entry    distill.Source
}

// planV6ToV7 moves every link file sources/skills/*.yaml into the sources
// block of its skill's .meta/distill.yaml and removes the file. The address
// of an entry comes from the source record in sources/catalog/, which stays
// where it is. A distill.yaml is created for a skill that has none, and an
// entry the block already holds is kept as it is, so a hub moved partly by
// hand ends the same. A hub without link files only gets its new marker.
func planV6ToV7(root string, pending map[string]mutation.Change) ([]mutation.Change, []FileDiff, error) {
	markerRel := ".skillstation/schema-version"
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(markerRel)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("read schema-version marker: %w", err)
	}
	marker := []byte("7\n")
	changes := []mutation.Change{{Path: markerRel, Contents: marker}}
	diffs := []FileDiff{{Path: markerRel, Before: string(before), After: string(marker), Diff: fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -1 +1 @@\n-%s+%s", markerRel, markerRel, before, marker)}}

	read := func(rel string) ([]byte, bool, error) {
		if change, ok := pending[rel]; ok {
			if change.Delete {
				return nil, false, nil
			}
			return change.Contents, false, nil
		}
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return contents, err == nil, err
	}
	links, problems, err := readLinkFiles(root, pending, read)
	if err != nil {
		return nil, nil, err
	}
	bySkill := map[string][]movedLink{}
	for _, link := range links {
		bySkill[link.skillDir] = append(bySkill[link.skillDir], link)
	}
	skillDirs := make([]string, 0, len(bySkill))
	for dir := range bySkill {
		skillDirs = append(skillDirs, dir)
	}
	sort.Strings(skillDirs)
	for _, dir := range skillDirs {
		change, diff, problem, err := mergeSkillSources(read, dir, bySkill[dir])
		if err != nil {
			return nil, nil, err
		}
		if problem != nil {
			problems = append(problems, *problem)
			continue
		}
		changes = append(changes, change)
		diffs = append(diffs, diff)
	}
	if len(problems) > 0 {
		sort.Slice(problems, func(i, j int) bool { return problems[i].Path < problems[j].Path })
		return nil, nil, &SourceLinkError{Items: problems}
	}
	for _, link := range links {
		changes = append(changes, mutation.Change{Path: link.rel, BeforeDigest: sourcepkg.Digest(link.contents), Delete: true})
		diffs = append(diffs, FileDiff{Path: link.rel, Before: string(link.contents),
			Diff: fmt.Sprintf("--- a/%s\n+++ /dev/null\n@@ moved into %s @@\n", link.rel, path.Join(link.skillDir, ".meta", "distill.yaml"))})
	}
	return changes, diffs, nil
}

// readLinkFiles reads every link file and finds the skill and the source
// record it names. What cannot be carried over is returned as problems.
func readLinkFiles(root string, pending map[string]mutation.Change, read readPlanned) ([]movedLink, []SourceLinkProblem, error) {
	var rels []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(linkDirectory)), func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return filepath.SkipDir
			}
			return walkErr
		}
		if entry.IsDir() || (!strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml")) {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", linkDirectory, err)
	}
	sort.Strings(rels)
	skillDirs, err := skillFolders(root, pending)
	if err != nil {
		return nil, nil, err
	}
	var links []movedLink
	var problems []SourceLinkProblem
	for _, rel := range rels {
		contents, _, err := read(rel)
		if err != nil {
			return nil, nil, err
		}
		link, problem := resolveLink(read, skillDirs, rel, contents)
		if problem != nil {
			problems = append(problems, *problem)
			continue
		}
		links = append(links, link)
	}
	return links, problems, nil
}

func resolveLink(read readPlanned, skillDirs map[string][]string, rel string, contents []byte) (movedLink, *SourceLinkProblem) {
	refuse := func(problem, fix string) (movedLink, *SourceLinkProblem) {
		return movedLink{}, &SourceLinkProblem{Path: rel, Problem: problem, Fix: fix}
	}
	var link linkFile
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(&link); err != nil {
		return refuse(fmt.Sprintf("is not a link file the hub wrote (%v), so the sources block has no place for all of it.", err),
			"Move what it holds elsewhere and delete the file, then run the migration again.")
	}
	if link.SchemaVersion != 1 || link.SkillID == "" || link.SourceID == "" {
		return refuse("does not name both a skill_id and a source_id with schema_version 1.", "Correct or delete the file, then run the migration again.")
	}
	if link.Role != "" && link.Role != "learning-source" {
		return refuse(fmt.Sprintf("has role %q; the sources block of distill.yaml holds only the sources a skill learns from.", link.Role),
			"Set the role to learning-source if the skill learns from the source, or delete the file (a skill's upstream is recorded in its .meta/hub.yaml), then run the migration again.")
	}
	dirs := skillDirs[link.SkillID]
	if len(dirs) != 1 {
		problem := fmt.Sprintf("names skill %q, which the hub no longer has.", link.SkillID)
		if len(dirs) > 1 {
			problem = fmt.Sprintf("names skill %q, which is in more than one collection (%s).", link.SkillID, strings.Join(dirs, ", "))
		}
		return refuse(problem, "Restore the skill, or delete the link file if the skill is gone for good (Git keeps its history), then run the migration again.")
	}
	recordRel := "sources/catalog/" + link.SourceID + ".yaml"
	recordBytes, _, err := read(recordRel)
	if err != nil || recordBytes == nil {
		return refuse(fmt.Sprintf("names source %q, which has no record in %s; the record holds the repository the sources block needs.", link.SourceID, recordRel),
			"Restore "+recordRel+" from Git history, or delete the link file, then run the migration again.")
	}
	// Only the address is taken from the record, which stays as it is.
	var record sourcepkg.Record
	if err := yaml.Unmarshal(recordBytes, &record); err != nil {
		return refuse(fmt.Sprintf("names source %q, whose record %s cannot be read: %v.", link.SourceID, recordRel, err),
			"Correct "+recordRel+", then run the migration again.")
	}
	record.ID = link.SourceID
	entry := distill.SourceFromRecord(record)
	if err := distill.ValidateSources([]distill.Source{entry}); err != nil {
		return refuse(fmt.Sprintf("names source %q, which cannot be an entry of the sources block: %v.", link.SourceID, err),
			"Correct "+recordRel+", then run the migration again.")
	}
	return movedLink{rel: rel, contents: contents, skillDir: dirs[0], entry: entry}, nil
}

// skillFolders maps each skill id to its folders: those with .meta/hub.yaml
// on disk or planned by an earlier step of the same migration.
func skillFolders(root string, pending map[string]mutation.Change) (map[string][]string, error) {
	found := map[string]bool{}
	files, err := filepath.Glob(filepath.Join(root, "skills", "*", "*", filepath.FromSlash(skillmeta.HubFile)))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return nil, err
		}
		found[strings.TrimSuffix(filepath.ToSlash(rel), "/"+skillmeta.HubFile)] = true
	}
	for rel, change := range pending {
		if dir, ok := strings.CutSuffix(rel, "/"+skillmeta.HubFile); ok && strings.HasPrefix(dir, "skills/") && strings.Count(dir, "/") == 2 {
			found[dir] = !change.Delete
		}
	}
	dirs := map[string][]string{}
	for dir, exists := range found {
		if exists {
			id := path.Base(dir)
			dirs[id] = append(dirs[id], dir)
		}
	}
	for _, list := range dirs {
		sort.Strings(list)
	}
	return dirs, nil
}

// mergeSkillSources adds the links of one skill to the sources block of its
// distill.yaml. An entry the block already holds stays as it is.
func mergeSkillSources(read readPlanned, skillDir string, links []movedLink) (mutation.Change, FileDiff, *SourceLinkProblem, error) {
	rel := path.Join(skillDir, ".meta", "distill.yaml")
	contents, onDisk, err := read(rel)
	if err != nil {
		return mutation.Change{}, FileDiff{}, nil, err
	}
	refuse := func(problem string) *SourceLinkProblem {
		return &SourceLinkProblem{Path: rel, Problem: problem, Fix: "Correct the file so its sources block can be read, then run the migration again."}
	}
	sources, err := distill.ReadSources(contents)
	if err != nil {
		return mutation.Change{}, FileDiff{}, refuse(fmt.Sprintf("has a sources block the hub cannot read (%v), so the links of the skill cannot be added to it.", err)), nil
	}
	held := map[string]bool{}
	for _, entry := range sources {
		held[entry.ID] = true
	}
	sort.Slice(links, func(i, j int) bool { return links[i].entry.ID < links[j].entry.ID })
	for _, link := range links {
		if !held[link.entry.ID] {
			sources = append(sources, link.entry)
			held[link.entry.ID] = true
		}
	}
	after, err := distill.WithSources(contents, sources)
	if err != nil {
		return mutation.Change{}, FileDiff{}, refuse(fmt.Sprintf("cannot take a sources block: %v.", err)), nil
	}
	change := mutation.Change{Path: rel, Contents: after}
	if onDisk {
		change.BeforeDigest = sourcepkg.Digest(contents)
	}
	diff := FileDiff{Path: rel, Before: string(contents), After: string(after), Diff: newFileDiff(rel, after)}
	if contents != nil {
		block, _ := distill.WithSources(nil, sources)
		diff.Diff = fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ sources @@\n", rel, rel) + strings.TrimPrefix(newFileDiff(rel, block), fmt.Sprintf("--- /dev/null\n+++ b/%s\n", rel))
	}
	return change, diff, nil, nil
}

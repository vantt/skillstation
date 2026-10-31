package distill

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	sourcepkg "github.com/vantt/skillstation/internal/source"
)

// sourcesKey is the top-level key of the block in .meta/distill.yaml that
// names the repositories a skill learns from. In a hub the hub writes it; the
// distill-lab script keeps it as it is.
const sourcesKey = "sources"

// Source is one entry of the sources block: a repository the skill learns
// from. ID is the source id that cursors, coverage and lesson citations use.
type Source struct {
	ID         string `yaml:"id" json:"id"`
	Repository string `yaml:"repository" json:"repository"`
	Ref        string `yaml:"ref,omitempty" json:"ref,omitempty"`
	Path       string `yaml:"path,omitempty" json:"path,omitempty"`
}

// SourceFromRecord is the sources block entry for a hub source record. The
// record is the authority on the address: when an entry names another
// address, readers use the record and doctor reports the difference.
func SourceFromRecord(record sourcepkg.Record) Source {
	locator := record.Locator
	switch {
	case locator.Repository != "":
		return Source{ID: record.ID, Repository: locator.Repository, Ref: locator.Ref, Path: locator.Path}
	case locator.URL != "":
		return Source{ID: record.ID, Repository: locator.URL, Ref: locator.Ref}
	default:
		return Source{ID: record.ID, Repository: locator.Path, Ref: locator.Ref}
	}
}

// ValidateSources checks the entries of a sources block: a kebab-case id
// that no other entry uses and a repository.
func ValidateSources(sources []Source) error {
	seen := make(map[string]bool, len(sources))
	for i, entry := range sources {
		if !KeyPattern.MatchString(entry.ID) {
			return fmt.Errorf("sources[%d]: id %q must be kebab-case", i, entry.ID)
		}
		if seen[entry.ID] {
			return fmt.Errorf("sources: duplicate id %q", entry.ID)
		}
		seen[entry.ID] = true
		if strings.TrimSpace(entry.Repository) == "" {
			return fmt.Errorf("sources.%s: repository must be non-empty text", entry.ID)
		}
	}
	return nil
}

// ReadSources returns the sources block of a distill.yaml. A file without the
// block, or an empty file, has none. Only the block is checked, so the
// sources of a skill whose lessons do not validate can still be read.
func ReadSources(data []byte) ([]Source, error) {
	mapping, err := topMapping(data)
	if err != nil || mapping == nil {
		return nil, err
	}
	_, value := mappingEntry(mapping, sourcesKey)
	if value == nil {
		return nil, nil
	}
	return decodeSources(value)
}

// WithSources returns data with its sources block set to sources: replaced
// where it is, or inserted before the cursors, coverage or lessons block (the
// order the distill-lab script writes), or after the last block. Every other
// line of the file is kept byte for byte. An empty list removes the block. For
// an empty data the result is a file holding only the block.
func WithSources(data []byte, sources []Source) ([]byte, error) {
	if err := ValidateSources(sources); err != nil {
		return nil, err
	}
	mapping, err := topMapping(data)
	if err != nil {
		return nil, err
	}
	block, err := renderSources(sources)
	if err != nil {
		return nil, err
	}
	lines := splitLines(data)
	if mapping == nil {
		// An empty document may hold comments and a null; the comments stay.
		kept := make([][]byte, 0, len(lines))
		for _, line := range lines {
			if isCommentOrBlank(line) {
				kept = append(kept, line)
			}
		}
		return append(joinLines(kept), block...), nil
	}
	if key, value := mappingEntry(mapping, sourcesKey); key != nil {
		if value.Kind == yaml.AliasNode || value.Anchor != "" {
			return nil, errors.New("sources: the block uses a YAML anchor or alias; write it out in full")
		}
		start := key.Line - 1
		end := blockEnd(lines, nextKeyLine(mapping, key))
		out := append([][]byte{}, lines[:start]...)
		out = append(out, splitLines(block)...)
		out = append(out, lines[end:]...)
		return joinLines(out), nil
	}
	if len(sources) == 0 {
		return data, nil
	}
	at := len(lines)
	for _, name := range []string{"cursors", "coverage", "lessons"} {
		if key, _ := mappingEntry(mapping, name); key != nil {
			at = commentStart(lines, key.Line-1)
			break
		}
	}
	out := append([][]byte{}, lines[:at]...)
	out = append(out, splitLines(block)...)
	out = append(out, lines[at:]...)
	return joinLines(out), nil
}

// HasContent reports whether a distill.yaml holds any top-level key, so a
// file the hub emptied can be removed instead of being left behind.
func HasContent(data []byte) (bool, error) {
	mapping, err := topMapping(data)
	if err != nil {
		return false, err
	}
	return mapping != nil && len(mapping.Content) > 0, nil
}

func decodeSources(value *yaml.Node) ([]Source, error) {
	if value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, errors.New("sources: must be a list of {id, repository, ref, path}")
	}
	sources := make([]Source, 0, len(value.Content))
	for i, item := range value.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("sources[%d]: must be a mapping of id, repository, ref and path", i)
		}
		var entry Source
		for j := 0; j+1 < len(item.Content); j += 2 {
			name, field := item.Content[j].Value, item.Content[j+1]
			if field.Kind != yaml.ScalarNode || field.Tag != "!!str" {
				return nil, fmt.Errorf("sources[%d].%s: must be text", i, name)
			}
			switch name {
			case "id":
				entry.ID = field.Value
			case "repository":
				entry.Repository = field.Value
			case "ref":
				entry.Ref = field.Value
			case "path":
				entry.Path = field.Value
			default:
				return nil, fmt.Errorf("sources[%d]: unknown field %q", i, name)
			}
		}
		sources = append(sources, entry)
	}
	if err := ValidateSources(sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// renderSources writes the block as the distill-lab script writes it: block
// style, two-space indentation, keys in the order id, repository, ref, path.
func renderSources(sources []Source) ([]byte, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string][]Source{sourcesKey: sources}); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// topMapping parses data and returns its top-level block mapping, or nil for
// an empty file.
func topMapping(data []byte) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("distill.yaml is not valid YAML: %w", err)
	}
	if document.Kind == 0 || len(document.Content) == 0 {
		return nil, nil
	}
	mapping := document.Content[0]
	if mapping.Kind == yaml.ScalarNode && mapping.Tag == "!!null" {
		return nil, nil
	}
	if mapping.Kind != yaml.MappingNode {
		return nil, errors.New("distill.yaml must be a mapping at the top level")
	}
	if mapping.Style&yaml.FlowStyle != 0 {
		return nil, errors.New("distill.yaml must be written in block style at the top level")
	}
	return mapping, nil
}

func mappingEntry(mapping *yaml.Node, name string) (key, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == name {
			return mapping.Content[i], mapping.Content[i+1]
		}
	}
	return nil, nil
}

// nextKeyLine returns the zero-based line of the top-level key after key, or
// -1 when key is the last one.
func nextKeyLine(mapping, key *yaml.Node) int {
	for i := 0; i+2 < len(mapping.Content); i += 2 {
		if mapping.Content[i] == key {
			return mapping.Content[i+2].Line - 1
		}
	}
	return -1
}

// blockEnd returns the first line after a block that runs until next (or the
// end of the file when next is -1). Comment and blank lines right before next
// belong to what follows, so they are kept.
func blockEnd(lines [][]byte, next int) int {
	if next < 0 {
		next = len(lines)
	}
	end := next
	for end > 0 && isCommentOrBlank(lines[end-1]) {
		end--
	}
	return end
}

// commentStart returns the first line of the comment lines right above line.
func commentStart(lines [][]byte, line int) int {
	for line > 0 && isComment(lines[line-1]) {
		line--
	}
	return line
}

func isComment(line []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(line, " \t"), []byte("#"))
}

func isCommentOrBlank(line []byte) bool {
	trimmed := bytes.TrimSpace(line)
	return len(trimmed) == 0 || trimmed[0] == '#'
}

// splitLines splits data into lines that keep their line endings.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			lines = append(lines, append(data[:len(data):len(data)], '\n'))
			break
		}
		lines = append(lines, data[:index+1])
		data = data[index+1:]
	}
	return lines
}

func joinLines(lines [][]byte) []byte {
	return bytes.Join(lines, nil)
}

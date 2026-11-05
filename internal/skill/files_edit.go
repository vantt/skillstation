package skill

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrEditNeedsHub reports an edit of something only the hub holds or decides:
// the name, the SKILL.md content, the runtime block, the review rationale or
// the content approval.
var ErrEditNeedsHub = errors.New("this field is changed on the hub")

// metadataFile is the author's metadata file of a skill folder, with the
// routing; the hub's own state is not part of a folder.
const metadataFile = "skill.yaml"

// EditFiles applies the routing, description and list fields of an update to
// the files of a skill folder as a project holds them (skill.yaml and
// SKILL.md), with the rules a hub edit uses, and returns the edited files. The
// input is not changed. The description lives in the frontmatter of SKILL.md;
// skill.yaml carries it too only when it already names one.
func EditFiles(files map[string][]byte, id string, input UpdateInput) (map[string][]byte, error) {
	if input.Name != nil || input.SetContent || input.Runtime != nil || input.Rationale != nil || input.ContentReviewedDigest != nil {
		return nil, fmt.Errorf("%w: only the description, the routing, labels, topics and technologies can be edited from a project", ErrEditNeedsHub)
	}
	edited := make(map[string][]byte, len(files)+1)
	maps.Copy(edited, files)

	stored, present := files[metadataFile]
	if !present {
		stored = []byte("schema_version: 1\n")
	}
	var document, original map[string]any
	if err := yaml.Unmarshal(stored, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", metadataFile, err)
	}
	if document == nil {
		document = map[string]any{}
	}
	_ = yaml.Unmarshal(stored, &original)
	fields := input
	fields.Description = nil
	if err := applyUpdateFields(document, metadataFile, id, fields); err != nil {
		return nil, err
	}
	if input.Description != nil {
		if _, named := document["description"]; named {
			document["description"] = strings.TrimSpace(*input.Description)
		}
	}
	// An edit that leaves every value as it was leaves the file as it was too,
	// instead of rewriting its layout.
	if !sameYAML(original, document) || !present {
		overrides := map[string]*yaml.Node{}
		if routing, ok := document["routing"].(map[string]any); ok {
			if node := orderedRouting(stored, routing); node != nil {
				overrides["routing"] = node
			}
		}
		after, err := editedMetadata(stored, document, overrides)
		if err != nil {
			return nil, err
		}
		if present || !bytes.Equal(after, stored) {
			edited[metadataFile] = after
		}
	}

	if input.Description != nil {
		entry, err := editedEntrypoint(files["SKILL.md"], *input.Description)
		if err != nil {
			return nil, err
		}
		edited["SKILL.md"] = entry
	}
	return edited, nil
}

// editedEntrypoint gives SKILL.md the description, changing nothing else.
func editedEntrypoint(current []byte, description string) ([]byte, error) {
	normalized := normalizeText(current)
	header, body, ok := splitSkillFrontmatter(normalized)
	if !ok {
		return nil, errors.New("SKILL.md has no frontmatter to put the description in")
	}
	if frontmatterDescription(header) == strings.TrimSpace(description) {
		return current, nil
	}
	updated, err := withFrontmatterDescription(header, description)
	if err != nil {
		return nil, fmt.Errorf("existing SKILL.md frontmatter is invalid: %w", err)
	}
	return append(append(append([]byte("---\n"), updated...), []byte("---\n")...), body...), nil
}

// RoutingOfFiles reads the routing a skill folder's skill.yaml holds, so an
// edit that names only some fields can keep the others.
func RoutingOfFiles(files map[string][]byte) RoutingInput {
	var parsed struct {
		Routing RoutingInput `yaml:"routing"`
	}
	_ = yaml.Unmarshal(files[metadataFile], &parsed)
	return parsed.Routing
}

// sameYAML reports whether two documents hold the same values, whatever the Go
// types they were built from. An empty value counts as an absent one: a file
// that leaves out `operations: []` and an edit that writes it say the same.
func sameYAML(a, b map[string]any) bool {
	left, errLeft := yaml.Marshal(withoutEmpty(a))
	right, errRight := yaml.Marshal(withoutEmpty(b))
	return errLeft == nil && errRight == nil && bytes.Equal(left, right)
}

// withoutEmpty copies a document without its nil, empty-string, empty-list and
// empty-mapping values, at every depth.
func withoutEmpty(document map[string]any) map[string]any {
	kept := make(map[string]any, len(document))
	for key, value := range document {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			if typed == "" {
				continue
			}
		case []any:
			if len(typed) == 0 {
				continue
			}
		case []string:
			if len(typed) == 0 {
				continue
			}
		case map[string]any:
			if value = withoutEmpty(typed); len(value.(map[string]any)) == 0 {
				continue
			}
		}
		kept[key] = value
	}
	return kept
}

// orderedRouting returns the routing block of stored with the wanted values in
// it, changing only what differs: the fields keep their order and comments, an
// empty value the file never had is not added, and a field that was emptied is
// removed. A reviewer then sees the one trigger that was added, not a block
// rewritten in another order. It returns nil when the file has no routing
// mapping to edit.
func orderedRouting(stored []byte, wanted map[string]any) *yaml.Node {
	var document yaml.Node
	if yaml.Unmarshal(stored, &document) != nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	top := document.Content[0]
	var routing *yaml.Node
	for index := 0; index+1 < len(top.Content); index += 2 {
		if top.Content[index].Value == "routing" {
			routing = top.Content[index+1]
		}
	}
	if routing == nil || routing.Kind != yaml.MappingNode {
		return nil
	}
	kept := make([]*yaml.Node, 0, len(routing.Content)+2)
	seen := map[string]bool{}
	for index := 0; index+1 < len(routing.Content); index += 2 {
		key, valueNode := routing.Content[index], routing.Content[index+1]
		seen[key.Value] = true
		var current any
		_ = valueNode.Decode(&current)
		want, has := wanted[key.Value]
		switch {
		case !has || isEmpty(want):
			if isEmpty(current) {
				kept = append(kept, key, valueNode) // empty before, empty after: leave it
			}
		case sameValue(current, want):
			kept = append(kept, key, valueNode)
		default:
			replacement := &yaml.Node{}
			if replacement.Encode(want) != nil {
				return nil
			}
			replacement.HeadComment, replacement.LineComment, replacement.FootComment = valueNode.HeadComment, valueNode.LineComment, valueNode.FootComment
			kept = append(kept, key, replacement)
		}
	}
	added := make([]string, 0, len(wanted))
	for key, want := range wanted {
		if !seen[key] && !isEmpty(want) {
			added = append(added, key)
		}
	}
	sort.Strings(added)
	for _, key := range added {
		value := &yaml.Node{}
		if value.Encode(wanted[key]) != nil {
			return nil
		}
		kept = append(kept, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	}
	routing.Content = kept
	return routing
}

func isEmpty(value any) bool {
	return len(withoutEmpty(map[string]any{"v": value})) == 0
}

func sameValue(a, b any) bool {
	return sameYAML(map[string]any{"v": a}, map[string]any{"v": b})
}

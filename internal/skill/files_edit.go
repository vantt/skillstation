package skill

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
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
		after, err := editedMetadata(stored, document)
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

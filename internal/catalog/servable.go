package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vantt/skillstation/internal/canonical"
	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
	"gopkg.in/yaml.v3"
)

const (
	// MaxDistributedSkillResources bounds the files one skill may expose over distribution.
	MaxDistributedSkillResources = 512
	// MaxDistributedSkillBytes bounds the total bytes one skill may expose over distribution.
	MaxDistributedSkillBytes = 16 << 20
)

// ErrSkillNotServable marks an active skill whose files cannot be distributed.
var ErrSkillNotServable = errors.New("skill cannot be served")

// ParseSkillFrontmatter decodes SKILL.md frontmatter into the JSON data model
// carried by skill distribution. Scalars that a YAML parser reads as something
// JSON cannot carry faithfully keep the form written in the file: a timestamp
// stays its text (2026-10-12, not an RFC 3339 instant) and an integer beyond
// 2^53 stays an exact json.Number, so a host that parses the same SKILL.md
// bytes itself sees the same values.
func ParseSkillFrontmatter(contents []byte) (map[string]any, error) {
	contents = bytes.ReplaceAll(contents, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(contents, []byte("---\n")) {
		return nil, errors.New("SKILL.md must begin with YAML frontmatter")
	}
	end := bytes.Index(contents[4:], []byte("\n---\n"))
	if end < 0 {
		return nil, errors.New("SKILL.md frontmatter is not terminated")
	}
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(contents[4 : 4+end]))
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}
	var value map[string]any
	if err := root.Decode(&value); err != nil {
		return nil, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}
	if value == nil {
		return nil, errors.New("SKILL.md frontmatter must be an object")
	}
	// Round-trip through JSON to reject YAML-only map keys and normalize values
	// to the exact JSON data model carried by SEP-2640.
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("frontmatter is not JSON-compatible: %w", err)
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, err
	}
	if len(root.Content) == 1 {
		keepWrittenScalars(normalized, root.Content[0])
	}
	return normalized, nil
}

// ValidateServableSkill applies the invariants skill distribution enforces so an
// active skill that could never be served is rejected when the catalog is built.
func ValidateServableSkill(id string, entrypoint []byte, resourceCount int, totalBytes int64) error {
	_, err := ParseServableSkill(id, entrypoint, resourceCount, totalBytes)
	return err
}

// ParseServableSkill validates a skill like ValidateServableSkill and returns
// the parsed frontmatter, so a caller that needs both parses the entrypoint once.
func ParseServableSkill(id string, entrypoint []byte, resourceCount int, totalBytes int64) (map[string]any, error) {
	if resourceCount < 1 || resourceCount > MaxDistributedSkillResources {
		return nil, fmt.Errorf("%w: skill %s has %d distributed resources; supported range is 1..%d", ErrSkillNotServable, id, resourceCount, MaxDistributedSkillResources)
	}
	if totalBytes > MaxDistributedSkillBytes {
		return nil, fmt.Errorf("%w: skill %s exceeds the %d-byte distribution limit", ErrSkillNotServable, id, MaxDistributedSkillBytes)
	}
	frontmatter, err := ParseSkillFrontmatter(entrypoint)
	if err != nil {
		return nil, fmt.Errorf("%w: skill %s is not distributable: %v", ErrSkillNotServable, id, err)
	}
	name, _ := frontmatter["name"].(string)
	description, _ := frontmatter["description"].(string)
	if name != id || strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("%w: skill %s frontmatter must contain name %q and a non-empty description", ErrSkillNotServable, id, id)
	}
	return frontmatter, nil
}

// ServabilityWarnings reads the canonical files under a shared lock and
// returns one warning for each active skill distribution cannot serve, the
// same warnings a rebuild reports. It changes nothing.
func ServabilityWarnings(ctx context.Context, root string) ([]string, error) {
	input, err := captureInput(ctx, root)
	if err != nil {
		return nil, err
	}
	return input.Warnings, nil
}

// ServabilityWarningsWhileLocked is ServabilityWarnings for a caller that
// already holds the workspace lock.
func ServabilityWarningsWhileLocked(ctx context.Context, root string) ([]string, error) {
	input, err := captureInputWhileLocked(ctx, root, false)
	if err != nil {
		return nil, err
	}
	return input.Warnings, nil
}

// servableSkillWarnings reports every active skill that distribution cannot
// serve. Such skills stay published (a hand edit must not block unrelated
// work) but are listed so the author can repair them; distribution and
// resolution skip them instead of failing for every other skill.
func servableSkillWarnings(input buildInput) []string {
	var warnings []string
	for _, item := range input.Entities {
		if item.Kind != "skill" || stringField(item.Document, "status") != "active" {
			continue
		}
		directory := strings.TrimSuffix(item.Path[:strings.LastIndex(item.Path, "/")], "/.meta")
		var entrypoint []byte
		count, total := 0, int64(0)
		for _, file := range input.Files {
			if !strings.HasPrefix(file.Path, directory+"/") || workspace.IsSkillMetadata(file.Path) {
				continue
			}
			count++
			total += int64(len(file.Bytes))
			if file.Path == directory+"/SKILL.md" {
				entrypoint = file.Bytes
			}
		}
		if entrypoint == nil {
			warnings = append(warnings, fmt.Sprintf("%s: skill %s will not be served: %s/SKILL.md is missing", directory, item.ID, directory))
			continue
		}
		if err := ValidateServableSkill(item.ID, entrypoint, count, total); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v; fix SKILL.md frontmatter and rebuild, or the skill is omitted from skills/list", directory, err))
		}
	}
	return warnings
}

// StateBasis identifies whether facts come from current canonical files or a published generation.
type StateBasis string

const (
	BasisCanonical StateBasis = "canonical"
	BasisServed    StateBasis = "served"
)

// CanonicalSkillFacts describes a skill's current state in the canonical workspace.
type CanonicalSkillFacts struct {
	Known          bool     `json:"known"`
	Collection     string   `json:"collection,omitempty"`
	Path           string   `json:"path,omitempty"`
	Status         string   `json:"status,omitempty"`
	Valid          bool     `json:"valid"`
	Issues         []string `json:"issues,omitempty"`
	EntrypointPath string   `json:"entrypoint_path,omitempty"`
}

// ServedSkillFacts describes a skill's state in the currently served/published generation.
type ServedSkillFacts struct {
	Known           bool   `json:"known"`
	Indexed         bool   `json:"indexed"`
	Generation      string `json:"generation,omitempty"`
	CatalogSnapshot string `json:"catalog_snapshot,omitempty"`
	Name            string `json:"name,omitempty"`
	Description     string `json:"description,omitempty"`
	Status          string `json:"status,omitempty"`
	Servable        bool   `json:"servable"`
	ServableReason  string `json:"servable_reason,omitempty"`
	ResourceCount   int    `json:"resource_count"`
	TotalBytes      int64  `json:"total_bytes"`
}

// SkillStateAssessment provides a basis-aware projection comparing canonical and served state.
type SkillStateAssessment struct {
	SkillID          string              `json:"skill_id"`
	ServingMode      ServingMode         `json:"serving_mode"`
	Warning          string              `json:"warning,omitempty"`
	Canonical        CanonicalSkillFacts `json:"canonical"`
	Served           ServedSkillFacts    `json:"served"`
	ResourcesMatch   bool                `json:"resources_match"`
	ChangedResources []string            `json:"changed_resources,omitempty"`
	MissingResources []string            `json:"missing_resources,omitempty"`
	Diverged         bool                `json:"diverged"`
}

// AssessSkillState assesses a skill's canonical and served facts under a shared lock.
func AssessSkillState(ctx context.Context, root, id string) (SkillStateAssessment, error) {
	lock, err := mutation.AcquireSharedLock(ctx, root, mutation.DefaultLockTimeout)
	if err != nil {
		return SkillStateAssessment{}, err
	}
	defer lock.Unlock()
	return AssessSkillStateWhileLocked(ctx, root, id)
}

// AssessSkillStateWhileLocked assesses a skill while the caller already holds a
// shared lock. A failure to read the canonical files or the published
// generation is returned, never shown as a skill whose state matches.
func AssessSkillStateWhileLocked(ctx context.Context, root, id string) (SkillStateAssessment, error) {
	// Overall status & serving mode. The scan that decides freshness also
	// keys the canonical validation below.
	status, scanned, err := inspectWithSnapshotWhileLocked(ctx, root)
	if err != nil {
		return SkillStateAssessment{}, err
	}
	return assessSkillState(ctx, root, id, status, scanned)
}

// AssessSkillStateInView assesses a skill like AssessSkillStateWhileLocked,
// under the lock the view holds, from the freshness verdict and the canonical
// scan the view was observed with. A request that has already observed the
// catalog therefore does not list and stamp every canonical file again. A view
// observed without a canonical scan is inspected here.
func AssessSkillStateInView(ctx context.Context, view *View, id string) (SkillStateAssessment, error) {
	if view.files == nil {
		return AssessSkillStateWhileLocked(ctx, view.root, id)
	}
	return assessSkillState(ctx, view.root, id, view.Status, canonical.Snapshot{Files: view.files})
}

func assessSkillState(ctx context.Context, root, id string, status Status, scanned canonical.Snapshot) (SkillStateAssessment, error) {
	assessment := SkillStateAssessment{
		SkillID:        id,
		ServingMode:    status.ServingMode,
		Warning:        status.Warning,
		ResourcesMatch: true,
	}

	if err := assessCanonicalSkill(root, id, scanned, &assessment.Canonical); err != nil {
		return SkillStateAssessment{}, err
	}
	if err := assessServedSkill(ctx, root, id, &assessment); err != nil {
		return SkillStateAssessment{}, err
	}

	// Compute divergence
	if !assessment.ResourcesMatch {
		assessment.Diverged = true
	} else if assessment.Canonical.Known && assessment.Served.Known && assessment.Canonical.Status != assessment.Served.Status {
		assessment.Diverged = true
	}

	return assessment, nil
}

// assessCanonicalSkill fills the canonical facts of skill id. A skill whose
// metadata cannot be read in a collection is not found there, as everywhere
// else a skill is looked up; validation reports why the files are invalid.
func assessCanonicalSkill(root, id string, scanned canonical.Snapshot, facts *CanonicalSkillFacts) error {
	skillsDir := filepath.Join(root, "skills")
	collections, err := os.ReadDir(skillsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var statusIssue string
	for _, coll := range collections {
		if !coll.IsDir() {
			continue
		}
		metaBytes, _, found, readErr := skillmeta.Read(skillmeta.DirReader(root), "skills/"+coll.Name()+"/"+id)
		if readErr != nil || !found {
			continue
		}
		facts.Known = true
		facts.Collection = coll.Name()
		facts.Path = "skills/" + coll.Name() + "/" + id
		var meta struct {
			Status string `yaml:"status"`
		}
		if err := yaml.Unmarshal(metaBytes, &meta); err != nil {
			statusIssue = facts.Path + ": the skill status cannot be read: " + err.Error()
		}
		facts.Status = meta.Status
		entrypointPath := filepath.Join(skillsDir, coll.Name(), id, "SKILL.md")
		if _, statErr := os.Stat(entrypointPath); statErr == nil {
			facts.EntrypointPath = "skills/" + coll.Name() + "/" + id + "/SKILL.md"
		}
		break
	}
	if !facts.Known {
		return nil
	}
	issues, err := skillCanonicalIssues(root, facts.Path, scanned)
	if err != nil {
		return err
	}
	if statusIssue != "" {
		issues = append(issues, statusIssue)
	}
	facts.Issues = issues
	facts.Valid = len(issues) == 0
	return nil
}

// assessServedSkill fills the served facts of skill id from the published
// generation and compares its resources with the files on disk. A hub without
// a usable generation serves nothing, which is a state, not a failure.
func assessServedSkill(ctx context.Context, root, id string, assessment *SkillStateAssessment) error {
	published, err := inspectPublishedWhileLocked(ctx, root)
	if err != nil {
		return err
	}
	if published.Pointer == nil || published.State == StateCorrupt || published.State == StateIncompatible {
		return nil
	}
	db, err := sql.Open("sqlite", sqliteDSN(generationPath(root, *published.Pointer), true))
	if err != nil {
		return fmt.Errorf("open published catalog generation: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	served := &assessment.Served
	var name, desc, status string
	err = db.QueryRowContext(ctx, `SELECT name, description, status FROM skills WHERE id=?`, id).Scan(&name, &desc, &status)
	if errors.Is(err, sql.ErrNoRows) {
		served.Known = true
		served.Indexed = false
		served.Generation = published.Pointer.Generation
		served.CatalogSnapshot = published.Pointer.CatalogSnapshot
		return nil
	}
	if err != nil {
		return fmt.Errorf("read skill %s from the published catalog: %w", id, err)
	}
	served.Known = true
	served.Indexed = true
	served.Generation = published.Pointer.Generation
	served.CatalogSnapshot = published.Pointer.CatalogSnapshot
	served.Name = name
	served.Description = desc
	served.Status = status

	rows, err := SkillResources(ctx, db, id)
	if err != nil {
		return fmt.Errorf("read the resources of skill %s from the published catalog: %w", id, err)
	}
	served.ResourceCount = len(rows)
	var totalBytes int64
	var entrypointBytes []byte
	for _, res := range rows {
		totalBytes += res.SizeBytes
		// Check live file on disk
		contents, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.Path)))
		if readErr != nil {
			assessment.MissingResources = append(assessment.MissingResources, res.Path)
			assessment.ResourcesMatch = false
			continue
		}
		sum := sha256.Sum256(contents)
		if "sha256:"+hex.EncodeToString(sum[:]) != res.Digest {
			assessment.ChangedResources = append(assessment.ChangedResources, res.Path)
			assessment.ResourcesMatch = false
		}
		if strings.HasSuffix(res.Path, "/SKILL.md") {
			entrypointBytes = contents
		}
	}
	served.TotalBytes = totalBytes

	// Assess servability
	if entrypointBytes == nil {
		served.Servable = false
		served.ServableReason = "SKILL.md is missing or unreadable"
	} else if valErr := ValidateServableSkill(id, entrypointBytes, len(rows), totalBytes); valErr != nil {
		served.Servable = false
		served.ServableReason = valErr.Error()
	} else {
		served.Servable = true
	}
	return nil
}

// maxExactJSONInteger is the largest magnitude a float64 holds without loss.
var maxExactJSONInteger = big.NewInt(1 << 53)

var decimalIntegerText = regexp.MustCompile(`^[-+]?[0-9][0-9_]*$`)

// keepWrittenScalars walks the decoded JSON-model value alongside the YAML
// node it came from and restores the written form of timestamp scalars and
// the exact value of integers float64 cannot hold. Plain strings, booleans,
// small numbers, lists and maps are left exactly as decoded.
func keepWrittenScalars(decoded any, node *yaml.Node) {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	switch typed := decoded.(type) {
	case map[string]any:
		if node.Kind != yaml.MappingNode {
			return
		}
		for key, current := range typed {
			child := mappingValue(node, key, 0)
			if child == nil {
				continue
			}
			if restored, ok := writtenScalar(child); ok {
				typed[key] = restored
				continue
			}
			keepWrittenScalars(current, child)
		}
	case []any:
		if node.Kind != yaml.SequenceNode || len(node.Content) != len(typed) {
			return
		}
		for index, current := range typed {
			if restored, ok := writtenScalar(node.Content[index]); ok {
				typed[index] = restored
				continue
			}
			keepWrittenScalars(current, node.Content[index])
		}
	}
}

// mappingValue finds the value node for key in a mapping, honoring explicit
// keys before merged ones the way the YAML decoder does.
func mappingValue(mapping *yaml.Node, key string, depth int) *yaml.Node {
	if depth > 8 {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if k := mapping.Content[index]; k.Kind == yaml.ScalarNode && k.ShortTag() != "!!merge" && k.Value == key {
			return mapping.Content[index+1]
		}
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if k := mapping.Content[index]; k.Kind != yaml.ScalarNode || k.ShortTag() != "!!merge" {
			continue
		}
		sources := []*yaml.Node{mapping.Content[index+1]}
		if sources[0].Kind == yaml.SequenceNode {
			sources = sources[0].Content
		}
		for _, source := range sources {
			for source.Kind == yaml.AliasNode && source.Alias != nil {
				source = source.Alias
			}
			if source.Kind != yaml.MappingNode {
				continue
			}
			if found := mappingValue(source, key, depth+1); found != nil {
				return found
			}
		}
	}
	return nil
}

// writtenScalar returns the value to serve for a scalar node whose decoded
// form would differ from what it says in the file, and false for any other
// node.
func writtenScalar(node *yaml.Node) (any, bool) {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	if node.Kind != yaml.ScalarNode {
		return nil, false
	}
	switch node.ShortTag() {
	case "!!timestamp":
		return node.Value, true
	case "!!int", "!!float":
		text := strings.TrimPrefix(strings.ReplaceAll(node.Value, "_", ""), "+")
		if node.ShortTag() == "!!float" && !decimalIntegerText.MatchString(node.Value) {
			return nil, false
		}
		number, ok := new(big.Int).SetString(text, 0)
		if !ok || new(big.Int).Abs(number).Cmp(maxExactJSONInteger) <= 0 {
			return nil, false
		}
		return json.Number(number.String()), true
	}
	return nil, false
}

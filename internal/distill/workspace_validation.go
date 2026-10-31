package distill

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vantt/skillstation/internal/source"
	"gopkg.in/yaml.v3"
)

// ValidateWorkspace enforces relationships that cannot be checked from one
// canonical document. It is called for external edits and every catalog rebuild.
func ValidateWorkspace(root string) error {
	paths, _ := filepath.Glob(filepath.Join(root, "distill", "sources", "*", "runs", "*.yaml"))
	hasDistillation := false
	for _, path := range paths {
		if data, readErr := os.ReadFile(path); readErr == nil {
			if _, parseErr := ParseRun(data); parseErr == nil {
				hasDistillation = true
				break
			}
		}
	}
	if !hasDistillation {
		return nil
	}
	workspace := distillWorkspace{
		root:                root,
		skipRuntimePackages: strings.HasPrefix(filepath.Base(root), ".skillstation-validate-"),
		sources:             map[string]source.Record{},
		runs:                map[string]Run{},
		observations:        map[string]Observation{},
		comparisons:         map[string]Comparison{},
		insights:            map[string]Insight{},
		proposals:           map[string]legacyApplicationProposal{},
		incorporations:      map[string]legacyIncorporation{},
		receiptAfter:        map[string]map[string]string{},
	}
	for _, step := range []func() error{
		workspace.load,
		workspace.checkRuns,
		workspace.checkObservations,
		workspace.checkComparisons,
		workspace.checkInsights,
		workspace.checkProposals,
		workspace.checkIncorporations,
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// distillWorkspace holds the distillation records of a hub, by ID, and the
// paths each applied operation receipt recorded.
type distillWorkspace struct {
	root                string
	skipRuntimePackages bool
	sources             map[string]source.Record
	runs                map[string]Run
	observations        map[string]Observation
	comparisons         map[string]Comparison
	insights            map[string]Insight
	proposals           map[string]legacyApplicationProposal
	incorporations      map[string]legacyIncorporation
	receiptAfter        map[string]map[string]string
}

// walk visits every YAML file under the relative directory; a missing
// directory has none.
func (w distillWorkspace) walk(relative string, visit func(string, []byte) error) error {
	base := filepath.Join(w.root, filepath.FromSlash(relative))
	return filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return visit(path, data)
	})
}

// load reads the source records, operation receipts and distillation records.
func (w distillWorkspace) load() error {
	if err := w.walk("sources/catalog", func(_ string, data []byte) error {
		item, err := source.ParseRecord(data)
		if err == nil {
			w.sources[item.ID] = item
		}
		return err
	}); err != nil {
		return err
	}
	if err := w.walk("history/operations", w.loadReceipt); err != nil {
		return err
	}
	return w.walk("distill", w.loadRecord)
}

func (w distillWorkspace) loadReceipt(_ string, data []byte) error {
	var receipt struct {
		ID      string `yaml:"id"`
		Status  string `yaml:"status"`
		Changes []struct {
			Path  string `yaml:"path"`
			After string `yaml:"after"`
		} `yaml:"changes"`
	}
	if err := yaml.Unmarshal(data, &receipt); err != nil {
		return err
	}
	if receipt.ID == "" || receipt.Status != "applied" {
		return errors.New("invalid operation receipt")
	}
	if _, duplicate := w.receiptAfter[receipt.ID]; duplicate {
		return fmt.Errorf("duplicate operation receipt %s", receipt.ID)
	}
	w.receiptAfter[receipt.ID] = map[string]string{}
	for _, change := range receipt.Changes {
		w.receiptAfter[receipt.ID][change.Path] = change.After
	}
	return nil
}

func (w distillWorkspace) loadRecord(path string, data []byte) error {
	var err error
	slash := filepath.ToSlash(path)
	switch {
	case strings.Contains(slash, "/runs/"):
		var item Run
		item, err = ParseRun(data)
		if err == nil {
			w.runs[item.ID] = item
		}
	case strings.Contains(slash, "/observations/"), strings.Contains(slash, "/findings/"):
		var item Observation
		item, err = ParseObservation(data)
		if err == nil {
			w.observations[item.ID] = item
		}
	case strings.Contains(slash, "/comparisons/"):
		var item Comparison
		item, err = ParseComparison(data)
		if err == nil {
			w.comparisons[item.ID] = item
		}
	case strings.Contains(slash, "/insights/"):
		var item Insight
		item, err = ParseInsight(data)
		if err == nil {
			w.insights[item.ID] = item
		}
	case strings.Contains(slash, "/proposals/"):
		var item legacyApplicationProposal
		item, err = parseLegacyApplicationProposal(data)
		if err == nil {
			w.proposals[item.ID] = item
		}
	case strings.Contains(slash, "/incorporations/"):
		var item legacyIncorporation
		item, err = parseLegacyIncorporation(data)
		if err == nil {
			w.incorporations[item.ID] = item
		}
	}
	return err
}

func (w distillWorkspace) checkRuns() error {
	latestFinalized := map[string]Run{}
	for _, run := range w.runs {
		if run.State == "finalized" && (latestFinalized[run.SourceID].FinalizedAt == "" || run.FinalizedAt > latestFinalized[run.SourceID].FinalizedAt) {
			latestFinalized[run.SourceID] = run
		}
		_, ok := w.sources[run.SourceID]
		if !ok {
			return fmt.Errorf("run %s references missing source %s", run.ID, run.SourceID)
		}
		for _, id := range run.FindingIDs {
			item, ok := w.observations[id]
			if !ok || item.SourceID != run.SourceID || (item.RunID == run.ID && item.LastSeen != IdentityOf(run.ToRevision)) {
				return fmt.Errorf("run %s declares invalid finding %s", run.ID, id)
			}
		}
		for _, id := range run.ComparisonIDs {
			if _, ok := w.comparisons[id]; !ok {
				return fmt.Errorf("run %s declares invalid comparison %s", run.ID, id)
			}
		}
		for _, id := range run.InsightIDs {
			if _, ok := w.insights[id]; !ok {
				return fmt.Errorf("run %s declares invalid insight %s", run.ID, id)
			}
		}
	}
	for sourceID, run := range latestFinalized {
		record := w.sources[sourceID]
		if record.DistilledRevision == nil || !SameRevision(*record.DistilledRevision, run.ToRevision) {
			return fmt.Errorf("latest finalized run %s target does not equal source cursor", run.ID)
		}
	}
	return nil
}

func (w distillWorkspace) checkObservations() error {
	for _, observation := range w.observations {
		run, ok := w.runs[observation.RunID]
		if !ok || run.SourceID != observation.SourceID {
			return fmt.Errorf("observation %s is not declared by its producing run", observation.ID)
		}
		if run.State == "finalized" && observation.Status == "active" && !containsID(run.FindingIDs, observation.ID) {
			return fmt.Errorf("observation %s is not declared by its producing run", observation.ID)
		}
	}
	return nil
}

func (w distillWorkspace) checkComparisons() error {
	for _, comparison := range w.comparisons {
		run, ok := w.runs[comparison.RunID]
		if !ok {
			return fmt.Errorf("comparison %s is not declared by its producing run", comparison.ID)
		}
		if run.State == "finalized" && !containsID(run.ComparisonIDs, comparison.ID) {
			return fmt.Errorf("comparison %s is not declared by its producing run", comparison.ID)
		}
		stale := false
		for _, id := range comparison.ObservationIDs {
			observation, ok := w.observations[id]
			if !ok || comparison.BasedOn[id] != observation.LastSeen || observation.Status != "active" {
				stale = true
			}
		}
		if comparison.Stale != stale {
			return fmt.Errorf("comparison %s stale marker is incorrect", comparison.ID)
		}
	}
	return nil
}

func (w distillWorkspace) checkInsights() error {
	for _, insight := range w.insights {
		run, ok := w.runs[insight.RunID]
		if !ok || (run.State == "finalized" && insight.Status == "active" && !containsID(run.InsightIDs, insight.ID)) {
			return fmt.Errorf("insight %s is not declared by its producing run", insight.ID)
		}
		active := false
		for _, id := range insight.ObservationIDs {
			observation, ok := w.observations[id]
			if !ok {
				return fmt.Errorf("insight %s references missing observation %s", insight.ID, id)
			}
			active = active || observation.Status == "active"
		}
		for _, id := range insight.ComparisonIDs {
			comparison, ok := w.comparisons[id]
			if !ok {
				return fmt.Errorf("insight %s references missing comparison %s", insight.ID, id)
			}
			if insight.Status == "pending" && comparison.Stale {
				return fmt.Errorf("pending insight %s relies on stale comparison %s", insight.ID, id)
			}
		}
		if insight.Status == "pending" && !active {
			return fmt.Errorf("pending insight %s has no active observation support", insight.ID)
		}
	}
	return nil
}

func (w distillWorkspace) checkProposals() error {
	for _, proposal := range w.proposals {
		if _, ok := w.insights[proposal.InsightID]; !ok {
			return fmt.Errorf("proposal %s references missing insight %s", proposal.ID, proposal.InsightID)
		}
		for _, pin := range proposal.PathPins {
			_, err := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(pin.Path)))
			if pin.After == "" {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("proposal %s deleted path %s still exists", proposal.ID, pin.Path)
				}
				continue
			}
			if err != nil {
				return fmt.Errorf("proposal %s artifact %s is missing", proposal.ID, pin.Path)
			}
		}
	}
	return nil
}

func (w distillWorkspace) checkIncorporations() error {
	for _, incorporation := range w.incorporations {
		if err := w.checkIncorporation(incorporation); err != nil {
			return err
		}
	}
	return nil
}

func (w distillWorkspace) checkIncorporation(incorporation legacyIncorporation) error {
	proposal, ok := w.proposals[incorporation.ProposalID]
	if !ok || proposal.InsightID != incorporation.InsightID {
		return fmt.Errorf("incorporation %s does not resolve to its insight proposal", incorporation.ID)
	}
	item, ok := w.insights[incorporation.InsightID]
	if !ok || item.Status != "incorporated" {
		return fmt.Errorf("incorporation %s references a missing or non-incorporated insight", incorporation.ID)
	}
	operationChanges, operationExists := w.receiptAfter[incorporation.OperationID]
	if !operationExists && !w.skipRuntimePackages {
		return fmt.Errorf("incorporation %s references missing operation %s", incorporation.ID, incorporation.OperationID)
	}
	pins := make(map[string]legacyPathPin, len(proposal.PathPins))
	for _, pin := range proposal.PathPins {
		if operationExists && operationChanges[pin.Path] != pin.After {
			return fmt.Errorf("incorporation %s target digest is not recorded by its operation", incorporation.ID)
		}
		pins[pin.Path] = pin
	}
	expectedObservations := map[string]bool{}
	for _, id := range item.ObservationIDs {
		expectedObservations[id] = true
	}
	for _, comparisonID := range item.ComparisonIDs {
		comparison, exists := w.comparisons[comparisonID]
		if !exists {
			return fmt.Errorf("incorporation %s references missing comparison %s", incorporation.ID, comparisonID)
		}
		for _, id := range comparison.ObservationIDs {
			expectedObservations[id] = true
		}
	}
	coveredObservations := map[string]bool{}
	coveredTargets := map[string]bool{}
	seenMappings := map[string]bool{}
	for _, mapping := range incorporation.SourceToLocal {
		key := mapping.ObservationID + "\x00" + mapping.ArtifactPath + "\x00" + strings.TrimSpace(mapping.Concept)
		if seenMappings[key] || !expectedObservations[mapping.ObservationID] {
			return fmt.Errorf("incorporation %s contains a dangling or duplicate mapping", incorporation.ID)
		}
		pin, exists := pins[mapping.ArtifactPath]
		if !exists {
			return fmt.Errorf("incorporation %s mapping targets an unpinned artifact", incorporation.ID)
		}
		data, readErr := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(mapping.ArtifactPath)))
		currentMatches := readErr == nil && source.Digest(data) == pin.After
		historicalMatches := w.receiptAfter[incorporation.OperationID][mapping.ArtifactPath] == pin.After
		if !currentMatches && !historicalMatches {
			return fmt.Errorf("incorporation %s mapping artifact digest is neither current nor recorded by its operation", incorporation.ID)
		}
		seenMappings[key] = true
		coveredObservations[mapping.ObservationID] = true
		coveredTargets[mapping.ArtifactPath] = true
	}
	if len(coveredObservations) != len(expectedObservations) || len(coveredTargets) != len(proposal.ChangedFiles) || len(incorporation.Targets) != len(proposal.ChangedFiles) {
		return fmt.Errorf("incorporation %s does not fully cover evidence and changed artifacts", incorporation.ID)
	}
	if proposal.Digest != legacyProposalDigest(item.ID, proposal.BaseCatalogVersion, incorporation.OperationID, proposal.PathPins, incorporation.SourceToLocal) {
		return fmt.Errorf("incorporation %s does not match proposal digest", incorporation.ID)
	}
	return nil
}

func containsID(values []string, id string) bool {
	for _, value := range values {
		if value == id {
			return true
		}
	}
	return false
}

type legacyPathPin struct {
	Path   string `yaml:"path" json:"path"`
	Before string `yaml:"before,omitempty" json:"before,omitempty"`
	After  string `yaml:"after,omitempty" json:"after,omitempty"`
}

type legacySourceToLocalMapping struct {
	ObservationID string `yaml:"observation_id" json:"observation_id"`
	ArtifactPath  string `yaml:"artifact_path" json:"artifact_path"`
	Concept       string `yaml:"concept" json:"concept"`
}

type legacyApplicationProposal struct {
	SchemaVersion      int             `yaml:"schema_version" json:"schema_version"`
	ID                 string          `yaml:"id" json:"id"`
	InsightID          string          `yaml:"insight_id" json:"insight_id"`
	BaseCatalogVersion string          `yaml:"base_catalog_version" json:"base_catalog_version"`
	Digest             string          `yaml:"digest" json:"digest"`
	Status             string          `yaml:"status" json:"status"`
	ChangedFiles       []string        `yaml:"changed_files" json:"changed_files"`
	PathPins           []legacyPathPin `yaml:"path_pins" json:"path_pins"`
	CreatedAt          string          `yaml:"created_at" json:"created_at"`
}

type legacyIncorporation struct {
	SchemaVersion  int                          `yaml:"schema_version" json:"schema_version"`
	ID             string                       `yaml:"id" json:"id"`
	InsightID      string                       `yaml:"insight_id" json:"insight_id"`
	ProposalID     string                       `yaml:"proposal_id" json:"proposal_id"`
	OperationID    string                       `yaml:"operation_id" json:"operation_id"`
	State          string                       `yaml:"state" json:"state"`
	Targets        []string                     `yaml:"targets" json:"targets"`
	SourceToLocal  []legacySourceToLocalMapping `yaml:"source_to_local" json:"source_to_local"`
	IncorporatedAt string                       `yaml:"incorporated_at" json:"incorporated_at"`
}

func parseLegacyApplicationProposal(data []byte) (legacyApplicationProposal, error) {
	var value legacyApplicationProposal
	err := strict(data, &value)
	return value, err
}

func parseLegacyIncorporation(data []byte) (legacyIncorporation, error) {
	var value legacyIncorporation
	err := strict(data, &value)
	return value, err
}

func legacyProposalDigest(insightID, base, operationID string, pins []legacyPathPin, mappings []legacySourceToLocalMapping) string {
	pins = append([]legacyPathPin(nil), pins...)
	mappings = append([]legacySourceToLocalMapping(nil), mappings...)
	sort.Slice(pins, func(i, j int) bool { return pins[i].Path < pins[j].Path })
	sort.Slice(mappings, func(i, j int) bool {
		if mappings[i].ObservationID != mappings[j].ObservationID {
			return mappings[i].ObservationID < mappings[j].ObservationID
		}
		if mappings[i].ArtifactPath != mappings[j].ArtifactPath {
			return mappings[i].ArtifactPath < mappings[j].ArtifactPath
		}
		return mappings[i].Concept < mappings[j].Concept
	})
	var builder strings.Builder
	builder.WriteString(insightID + "\n" + base + "\n" + operationID + "\n")
	for _, pin := range pins {
		builder.WriteString(pin.Path + ":" + pin.Before + "->" + pin.After + "\n")
	}
	for _, mapping := range mappings {
		builder.WriteString(mapping.ObservationID + "->" + mapping.ArtifactPath + "#" + mapping.Concept + "\n")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/catalog"
	"github.com/vantt/skillstation/internal/distill"
	"github.com/vantt/skillstation/internal/mutation"
	"github.com/vantt/skillstation/internal/shellquote"
	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
	"gopkg.in/yaml.v3"
)

type SourceAttachInput struct {
	SkillID           string `json:"skill_id"`
	SourceID          string `json:"source_id,omitempty"`
	Locator           string `json:"locator,omitempty"`
	Ref               string `json:"ref,omitempty"`
	Path              string `json:"path,omitempty"`
	Cadence           string `json:"cadence,omitempty"`
	MonitoringEnabled *bool  `json:"monitoring_enabled,omitempty"`
	Trust             string `json:"trust,omitempty"`
	License           string `json:"license,omitempty"`
	IdempotencyKey    string `json:"idempotency_key,omitempty"`
}

// skillSourceLink is one entry of a skill's sources block: the skill learns
// from the source. SkillDir is the workspace-relative skill folder.
type skillSourceLink struct {
	SkillID  string
	SkillDir string
	Source   distill.Source
}

// skillSourceProblem is a distill.yaml whose sources block cannot be read.
type skillSourceProblem struct {
	Path string
	Err  error
}

// readSkillSourceLinks returns the entries of the sources block of every
// skill, in skill folder order. A distill.yaml whose block cannot be read is
// left out and reported as a problem; doctor shows it.
func readSkillSourceLinks(root string) ([]skillSourceLink, []skillSourceProblem, error) {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	defer handle.Close()
	paths, err := fs.Glob(handle.FS(), "skills/*/*/"+skillDistillFile)
	if err != nil {
		return nil, nil, err
	}
	var links []skillSourceLink
	var problems []skillSourceProblem
	for _, file := range paths {
		data, readErr := handle.ReadFile(file)
		if readErr != nil {
			problems = append(problems, skillSourceProblem{Path: file, Err: readErr})
			continue
		}
		sources, parseErr := distill.ReadSources(data)
		if parseErr != nil {
			problems = append(problems, skillSourceProblem{Path: file, Err: parseErr})
			continue
		}
		dir := path.Dir(path.Dir(file))
		for _, entry := range sources {
			links = append(links, skillSourceLink{SkillID: path.Base(dir), SkillDir: dir, Source: entry})
		}
	}
	return links, problems, nil
}

// learningSourceIDs returns the ids of the sources some skill learns from.
func learningSourceIDs(links []skillSourceLink) map[string]bool {
	set := make(map[string]bool, len(links))
	for _, link := range links {
		set[link.Source.ID] = true
	}
	return set
}

// skillDistillPath is the workspace-relative distill.yaml of a skill folder.
func skillDistillPath(skillRelDir string) string {
	return path.Join(skillRelDir, skillDistillFile)
}

// readSkillSources returns a skill's distill.yaml, nil when it has none, and
// the entries of its sources block.
func readSkillSources(root, skillRelDir string) ([]byte, []distill.Source, error) {
	data, err := readWorkspaceFile(root, skillDistillPath(skillRelDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	sources, err := distill.ReadSources(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", skillDistillPath(skillRelDir), err)
	}
	return data, sources, nil
}

// unreadableSourcesProposal refuses a change to a sources block the hub
// cannot read, so it never overwrites what a person wrote there.
func unreadableSourcesProposal(err error) SourceProposal {
	return SourceProposal{Result: ErrorResult(NewInvalidRequestError(
		fmt.Sprintf("The sources block cannot be read: %v.", err),
		"Correct the sources block of the skill's .meta/distill.yaml, then try again."))}
}

// sourcesChange is the write that sets a skill's sources block to sources.
// The hub creates the file when the skill has none and removes a file that
// the change leaves empty.
func sourcesChange(skillRelDir string, before []byte, sources []distill.Source) (mutation.Change, error) {
	rel := skillDistillPath(skillRelDir)
	after, err := distill.WithSources(before, sources)
	if err != nil {
		return mutation.Change{}, fmt.Errorf("%s: %w", rel, err)
	}
	change := mutation.Change{Path: rel, Contents: after}
	if before == nil {
		return change, nil
	}
	change.BeforeDigest = sourcepkg.Digest(before)
	if hasContent, err := distill.HasContent(after); err != nil {
		return mutation.Change{}, fmt.Errorf("%s: %w", rel, err)
	} else if !hasContent {
		change.Contents, change.Delete = nil, true
	}
	return change, nil
}

// addSourceDiff records a sources change in a proposal diff.
func addSourceDiff(diff *SourceDiff, change mutation.Change) {
	switch {
	case change.Delete:
		diff.Deleted = append(diff.Deleted, change.Path)
	case change.BeforeDigest == "":
		diff.Added = append(diff.Added, change.Path)
	default:
		diff.Modified = append(diff.Modified, change.Path)
	}
}

func linkedSkills(root, sourceID string) ([]string, error) {
	linked := make(map[string]bool)

	allSkills, err := listAllSkillIDs(root)
	if err == nil {
		for _, id := range allSkills {
			_, _, metaBytes, err := locateSkillDir(root, id)
			if err != nil || len(metaBytes) == 0 {
				continue
			}
			var meta struct {
				Provenance struct {
					SourceID string `yaml:"source_id"`
				} `yaml:"provenance"`
			}
			if yaml.Unmarshal(metaBytes, &meta) == nil && meta.Provenance.SourceID == sourceID {
				linked[id] = true
			}
		}
	}

	links, _, err := readSkillSourceLinks(root)
	if err == nil {
		for _, l := range links {
			if l.Source.ID == sourceID {
				linked[l.SkillID] = true
			}
		}
	}

	result := make([]string, 0, len(linked))
	for id := range linked {
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func sourceHasExternalReferences(root, sourceID string) bool {
	distillSourceDir := filepath.Join(root, "distill", "sources", sourceID)
	for _, sub := range []string{"observations", "runs"} {
		if entries, err := os.ReadDir(filepath.Join(distillSourceDir, sub)); err == nil && len(entries) > 0 {
			return true
		}
	}

	handle, err := catalog.OpenCurrent(context.Background(), root)
	if err != nil {
		return false
	}
	defer handle.Close()

	var count int
	pattern := fmt.Sprintf(`%%"source_id":"%s"%%`, sourceID)
	err = handle.DB.QueryRow(`SELECT count(*) FROM canonical_entities WHERE kind NOT IN ('skill', 'skill_source_link') AND (json_extract(content_json, '$.source_id') = ? OR content_json LIKE ?)`, sourceID, pattern).Scan(&count)
	return err == nil && count > 0
}

func (service SourceService) PreviewAttach(ctx context.Context, path string, input SourceAttachInput) (SourceProposal, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return SourceProposal{}, err
	}
	service = service.defaults(root)

	skillID := strings.TrimSpace(input.SkillID)
	if skillID == "" {
		return SourceProposal{Result: ErrorResult(NewInvalidRequestError("skill_id is required", "Provide a target skill ID."))}, nil
	}
	_, skillRelDir, _, err := locateSkillDir(root, skillID)
	if err != nil {
		return SourceProposal{Result: ErrorResult(NewNotFoundError(fmt.Sprintf("skill %q not found", skillID), "Create the skill before attaching a source."))}, nil
	}
	distillBefore, current, err := readSkillSources(root, skillRelDir)
	if err != nil {
		return unreadableSourcesProposal(err), nil
	}

	hasSourceID := strings.TrimSpace(input.SourceID) != ""
	hasLocator := strings.TrimSpace(input.Locator) != ""
	if !hasSourceID && !hasLocator {
		return SourceProposal{Result: ErrorResult(NewInvalidRequestError("specify either source_id or locator", "Pass either source_id to link an existing source or locator to watch a new source."))}, nil
	}

	_, records, err := readSourceRecords(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return SourceProposal{}, err
	}

	sourceRecord, newSource, prop, err := service.resolveAttachSource(ctx, input, records)
	if err != nil {
		return SourceProposal{}, err
	}
	if prop != nil {
		return *prop, nil
	}

	// An entry the skill wrote for a source the hub does not watch yet gives
	// way to the new record's address.
	kept := make([]distill.Source, 0, len(current)+1)
	for _, entry := range current {
		if entry.ID != sourceRecord.ID {
			kept = append(kept, entry)
		} else if !newSource {
			return SourceProposal{
				Result: NewResult(StatusOK, fmt.Sprintf("Skill %q is already linked to %q.", skillID, sourceRecord.ID)),
				Source: sourceRecord,
			}, nil
		}
	}

	var changes []mutation.Change
	diff := SourceDiff{Added: []string{}, Modified: []string{}, Deleted: []string{}}

	if newSource {
		sourceBytes, _ := sourcepkg.MarshalCanonical(sourceRecord)
		sourcePath := "sources/catalog/" + sourceRecord.ID + ".yaml"
		changes = append(changes, mutation.Change{Path: sourcePath, Contents: sourceBytes})
		diff.Added = append(diff.Added, sourcePath)
	}

	linkChange, err := sourcesChange(skillRelDir, distillBefore, append(kept, distill.SourceFromRecord(sourceRecord)))
	if err != nil {
		return SourceProposal{}, err
	}
	changes = append(changes, linkChange)
	addSourceDiff(&diff, linkChange)

	set := mutation.WriteSet{
		Command:        "source_attach",
		IdempotencyKey: input.IdempotencyKey,
		Changes:        changes,
	}
	planned, err := mutation.PlanMutation(root, set)
	if err != nil {
		return SourceProposal{}, err
	}

	pins := ConfirmationPins{
		ProposalID:     planned.ID,
		ProposalDigest: planned.Digest,
		BaseVersion:    planned.BaseCatalogSnapshot,
	}
	expiresAt := service.Clock.Now().UTC().Add(24 * time.Hour)
	proposal := SourceProposal{
		Result: NewResult(StatusActionRequired, fmt.Sprintf("Attach proposal for %s to %s is ready for review.", sourceRecord.ID, skillID)),
		Source: sourceRecord,
		Diff:   diff,
		Confirmation: ConfirmationPolicy{
			PolicyRevision:     "policy_v1",
			ActionClass:        "semantic",
			ApplicationCommand: "source_watch_confirm",
			Confirmation: ConfirmationRequirement{
				Required: true,
				Mode:     "preview-and-approval",
				Pins:     pins,
			},
		},
		planned:   planned,
		expiresAt: expiresAt,
	}

	if err := storeSourceProposal(root, proposal, service.Clock.Now().UTC()); err != nil {
		return SourceProposal{}, err
	}
	return proposal, nil
}

func (service SourceService) resolveAttachSource(ctx context.Context, input SourceAttachInput, records []sourcepkg.Record) (sourcepkg.Record, bool, *SourceProposal, error) {
	if strings.TrimSpace(input.Locator) != "" {
		if prop := validateSourceWatchLocator(input.Locator); prop != nil {
			return sourcepkg.Record{}, false, prop, nil
		}
		adapter, ok := service.Adapters["git"]
		if !ok {
			return sourcepkg.Record{}, false, nil, fmt.Errorf("git adapter is not configured")
		}
		route, rProp := resolveSourceWatchRoute(ctx, adapter, input.Locator, input.Ref, input.Path)
		if rProp != nil {
			return sourcepkg.Record{}, false, rProp, nil
		}
		watchInput := SourceWatchInput{
			Locator:           input.Locator,
			SourceID:          input.SourceID,
			Ref:               input.Ref,
			Path:              input.Path,
			Cadence:           input.Cadence,
			MonitoringEnabled: input.MonitoringEnabled,
			Trust:             input.Trust,
			License:           input.License,
		}
		cfg, cProp := deriveSourceWatchConfig(watchInput, route)
		if cProp != nil {
			return sourcepkg.Record{}, false, cProp, nil
		}
		if collisionProp := checkExistingSourceCollisions(records, cfg); collisionProp != nil {
			if collisionProp.Error != nil {
				return sourcepkg.Record{}, false, collisionProp, nil
			}
			return collisionProp.Source, false, nil, nil
		}
		record, _, recProp, bErr := buildSourceWatchRecord(ctx, adapter, cfg, "")
		if bErr != nil {
			return sourcepkg.Record{}, false, nil, bErr
		}
		if recProp != nil {
			return sourcepkg.Record{}, false, recProp, nil
		}
		return record, true, nil, nil
	}

	for i := range records {
		if records[i].ID == input.SourceID {
			return records[i], false, nil, nil
		}
	}
	prop := SourceProposal{Result: ErrorResult(NewNotFoundError(fmt.Sprintf("source %q not found", input.SourceID), "Check configured sources with skillstation source list."))}
	return sourcepkg.Record{}, false, &prop, nil
}

func (service SourceService) PreviewDetach(ctx context.Context, path, skillID, sourceID string) (SourceProposal, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return SourceProposal{}, err
	}
	service = service.defaults(root)

	notLinked := SourceProposal{Result: ErrorResult(NewNotFoundError(
		fmt.Sprintf("skill %q does not learn from source %q", skillID, sourceID),
		"Check the skill's sources with `skillstation source list`."))}
	_, skillRelDir, _, err := locateSkillDir(root, skillID)
	if err != nil {
		return notLinked, nil
	}
	distillBefore, current, err := readSkillSources(root, skillRelDir)
	if err != nil {
		return unreadableSourcesProposal(err), nil
	}
	kept := make([]distill.Source, 0, len(current))
	for _, entry := range current {
		if entry.ID != sourceID {
			kept = append(kept, entry)
		}
	}
	if len(kept) == len(current) {
		return notLinked, nil
	}
	linkChange, err := sourcesChange(skillRelDir, distillBefore, kept)
	if err != nil {
		return SourceProposal{}, err
	}
	changes := []mutation.Change{linkChange}
	diff := SourceDiff{Added: []string{}, Modified: []string{}, Deleted: []string{}}
	addSourceDiff(&diff, linkChange)

	linked, _ := linkedSkills(root, sourceID)
	remaining := make([]string, 0, len(linked))
	for _, id := range linked {
		if id != skillID {
			remaining = append(remaining, id)
		}
	}

	sourcePath := "sources/catalog/" + sourceID + ".yaml"
	sourceData, sErr := readWorkspaceFile(root, sourcePath)
	var warnings []Warning

	if len(remaining) == 0 && sErr == nil {
		if !sourceHasExternalReferences(root, sourceID) {
			changes = append(changes, mutation.Change{
				Path:         sourcePath,
				BeforeDigest: sourcepkg.Digest(sourceData),
				Delete:       true,
			})
			diff.Deleted = append(diff.Deleted, sourcePath)
		} else {
			var rec sourcepkg.Record
			if err := yaml.Unmarshal(sourceData, &rec); err == nil {
				rec.Monitoring.Enabled = false
				rec.Monitoring.Cadence = "manual"
				updatedBytes, _ := sourcepkg.MarshalCanonical(rec)
				changes = append(changes, mutation.Change{
					Path:         sourcePath,
					BeforeDigest: sourcepkg.Digest(sourceData),
					Contents:     updatedBytes,
				})
				diff.Modified = append(diff.Modified, sourcePath)
				warnings = append(warnings, Warning{
					Code:    "source_kept_referenced",
					Summary: fmt.Sprintf("Source %s is referenced by other canonical entities; retained with monitoring disabled.", sourceID),
				})
			}
		}
	}

	set := mutation.WriteSet{
		Command: "source_detach",
		Changes: changes,
	}
	planned, err := mutation.PlanMutation(root, set)
	if err != nil {
		return SourceProposal{}, err
	}

	pins := ConfirmationPins{
		ProposalID:     planned.ID,
		ProposalDigest: planned.Digest,
		BaseVersion:    planned.BaseCatalogSnapshot,
	}
	expiresAt := service.Clock.Now().UTC().Add(24 * time.Hour)
	proposal := SourceProposal{
		Result: NewResult(StatusActionRequired, fmt.Sprintf("Detach proposal for %s from %s is ready for review.", sourceID, skillID)),
		Source: sourcepkg.Record{ID: sourceID},
		Diff:   diff,
		Confirmation: ConfirmationPolicy{
			PolicyRevision:     "policy_v1",
			ActionClass:        "semantic",
			ApplicationCommand: "source_watch_confirm",
			Confirmation: ConfirmationRequirement{
				Required: true,
				Mode:     "preview-and-approval",
				Pins:     pins,
			},
		},
		planned:   planned,
		expiresAt: expiresAt,
	}
	proposal.Warnings = warnings

	if err := storeSourceProposal(root, proposal, service.Clock.Now().UTC()); err != nil {
		return SourceProposal{}, err
	}
	return proposal, nil
}

func (service SourceService) PreviewUnwatch(ctx context.Context, path, sourceID string) (SourceProposal, error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return SourceProposal{}, err
	}
	service = service.defaults(root)

	sourcePath := "sources/catalog/" + sourceID + ".yaml"
	sourceData, err := readWorkspaceFile(root, sourcePath)
	if err != nil {
		return SourceProposal{Result: ErrorResult(NewNotFoundError(fmt.Sprintf("source %q not found", sourceID), "Verify source exists."))}, nil
	}

	linked, _ := linkedSkills(root, sourceID)
	hasReferences := sourceHasExternalReferences(root, sourceID)

	var changes []mutation.Change
	diff := SourceDiff{Added: []string{}, Modified: []string{}, Deleted: []string{}}

	if len(linked) == 0 && !hasReferences {
		changes = append(changes, mutation.Change{
			Path:         sourcePath,
			BeforeDigest: sourcepkg.Digest(sourceData),
			Delete:       true,
		})
		diff.Deleted = append(diff.Deleted, sourcePath)
	} else {
		var rec sourcepkg.Record
		if err := yaml.Unmarshal(sourceData, &rec); err != nil {
			return SourceProposal{}, err
		}
		rec.Monitoring.Enabled = false
		rec.Monitoring.Cadence = "manual"
		updatedBytes, _ := sourcepkg.MarshalCanonical(rec)
		changes = append(changes, mutation.Change{
			Path:         sourcePath,
			BeforeDigest: sourcepkg.Digest(sourceData),
			Contents:     updatedBytes,
		})
		diff.Modified = append(diff.Modified, sourcePath)
	}

	set := mutation.WriteSet{
		Command: "source_unwatch",
		Changes: changes,
	}
	planned, err := mutation.PlanMutation(root, set)
	if err != nil {
		return SourceProposal{}, err
	}

	pins := ConfirmationPins{
		ProposalID:     planned.ID,
		ProposalDigest: planned.Digest,
		BaseVersion:    planned.BaseCatalogSnapshot,
	}
	expiresAt := service.Clock.Now().UTC().Add(24 * time.Hour)
	proposal := SourceProposal{
		Result: NewResult(StatusActionRequired, fmt.Sprintf("Unwatch proposal for %s is ready for review.", sourceID)),
		Source: sourcepkg.Record{ID: sourceID},
		Diff:   diff,
		Confirmation: ConfirmationPolicy{
			PolicyRevision:     "policy_v1",
			ActionClass:        "semantic",
			ApplicationCommand: "source_watch_confirm",
			Confirmation: ConfirmationRequirement{
				Required: true,
				Mode:     "preview-and-approval",
				Pins:     pins,
			},
		},
		planned:   planned,
		expiresAt: expiresAt,
	}

	if err := storeSourceProposal(root, proposal, service.Clock.Now().UTC()); err != nil {
		return SourceProposal{}, err
	}
	return proposal, nil
}

// onboardingLinkChange is the sources block write that links an accepted
// source to its skill. pending holds the changes planned so far, which create
// the skill when it is new.
func onboardingLinkChange(root, skillID string, pending []mutation.Change, record sourcepkg.Record) (mutation.Change, *SourceProposal, error) {
	skillRelDir := ""
	for _, change := range pending {
		if dir, ok := strings.CutSuffix(change.Path, "/SKILL.md"); ok && path.Base(dir) == skillID && !change.Delete {
			skillRelDir = dir
		}
	}
	var before []byte
	var current []distill.Source
	if skillRelDir == "" {
		_, dir, _, err := locateSkillDir(root, skillID)
		if err != nil {
			prop := SourceProposal{Result: ErrorResult(NewNotFoundError(fmt.Sprintf("skill %q not found", skillID), "Pass the id of an existing skill, or --new-skill <id> to create one."))}
			return mutation.Change{}, &prop, nil
		}
		skillRelDir = dir
		if before, current, err = readSkillSources(root, skillRelDir); err != nil {
			prop := unreadableSourcesProposal(err)
			return mutation.Change{}, &prop, nil
		}
	}
	sources := make([]distill.Source, 0, len(current)+1)
	for _, entry := range current {
		if entry.ID != record.ID {
			sources = append(sources, entry)
		}
	}
	change, err := sourcesChange(skillRelDir, before, append(sources, distill.SourceFromRecord(record)))
	return change, nil, err
}

// sourcesChangeSkill returns the id of the skill whose sources block a
// planned change writes.
func sourcesChangeSkill(changes []mutation.Change) string {
	for _, change := range changes {
		if dir, ok := strings.CutSuffix(change.Path, "/"+skillDistillFile); ok && strings.HasPrefix(dir, "skills/") {
			return path.Base(dir)
		}
	}
	return ""
}

// sourceLinkFindings reports, for doctor, the sources blocks the hub cannot
// read, the entries whose address differs from the hub's source record (the
// record is used) and the entries for a source the hub does not watch.
func sourceLinkFindings(root string, report RoleReport) []Finding {
	const label = "Skill sources"
	links, problems, err := readSkillSourceLinks(root)
	var records []sourcepkg.Record
	if err == nil {
		_, records, err = readSourceRecords(root)
	}
	if err != nil {
		return []Finding{{Code: "hub_skill_sources", Label: label, Severity: SeveritySkipped, Message: "Could not read the skills' sources: " + plainError(err) + "."}}
	}
	byID := make(map[string]sourcepkg.Record, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	var findings []Finding
	for _, problem := range problems {
		findings = append(findings, Finding{Code: "hub_skill_sources_unreadable", Label: label, Severity: SeverityProblem,
			Message: problem.Path + ": the sources block cannot be read: " + plainError(problem.Err) + ".",
			Reason:  "Correct the sources block by hand; source attach and detach leave the file alone until then."})
	}
	for _, link := range links {
		record, watched := byID[link.Source.ID]
		if !watched {
			command := "skillstation source watch " + shellquote.Arg(link.Source.Repository) + " --skill-id " + link.SkillID + " --id " + link.Source.ID
			if link.Source.Ref != "" {
				command += " --ref " + shellquote.Arg(link.Source.Ref)
			}
			if link.Source.Path != "" {
				command += " --path " + shellquote.Arg(link.Source.Path)
			}
			findings = append(findings, Finding{Code: "hub_skill_source_unwatched", Label: label, Severity: SeverityNotice,
				Message: fmt.Sprintf("%s learns from %s (%s), which the hub does not watch.", link.SkillID, link.Source.ID, link.Source.Repository),
				Manual:  report.hubCommand(command)})
			continue
		}
		if want := distill.SourceFromRecord(record); want != link.Source {
			findings = append(findings, Finding{Code: "hub_skill_source_address", Label: label, Severity: SeverityNotice,
				Message: fmt.Sprintf("%s names source %s as %s, but the hub's record sources/catalog/%s.yaml says %s; the record is used.",
					link.SkillID, link.Source.ID, describeSourceAddress(link.Source), link.Source.ID, describeSourceAddress(want)),
				Reason: "Make the entry in " + skillDistillPath(link.SkillDir) + " match the record, or detach and attach the source again."})
		}
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{Code: "hub_skill_sources", Label: label, Severity: SeverityOK, Message: "readable and matching the source records"})
	}
	return findings
}

func describeSourceAddress(entry distill.Source) string {
	text := entry.Repository
	if entry.Ref != "" {
		text += " ref " + entry.Ref
	}
	if entry.Path != "" {
		text += " path " + entry.Path
	}
	return text
}

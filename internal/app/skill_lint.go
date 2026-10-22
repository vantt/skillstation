package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/vantt/skillstation/internal/skill"
)

// SkillLintInput chooses which skills a routing lint reports.
type SkillLintInput struct {
	// SkillID limits the report to one skill, in any state.
	SkillID string
	// All reports skills without findings too.
	All bool
}

// SkillLintRouting is the routing a skill has today, which an edit replaces
// field by field.
type SkillLintRouting struct {
	Triggers        []string `json:"triggers"`
	Examples        []string `json:"examples"`
	CounterExamples []string `json:"counter_examples"`
	NotFor          []string `json:"not_for"`
	MinScope        string   `json:"min_scope,omitempty"`
}

// SkillLintFinding is one routing problem of a skill.
type SkillLintFinding struct {
	Code         string `json:"code"`
	Field        string `json:"field,omitempty"`
	Value        string `json:"value,omitempty"`
	OtherSkillID string `json:"other_skill_id,omitempty"`
	Message      string `json:"message"`
	Fix          string `json:"fix"`
}

// SkillLintEntry is one skill of the report, with what an agent needs to draft
// better routing: the description, where SKILL.md is, the current routing and
// the findings.
type SkillLintEntry struct {
	ID          string             `json:"id"`
	State       string             `json:"state"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	SkillMD     string             `json:"skill_md"`
	Routing     SkillLintRouting   `json:"routing"`
	Findings    []SkillLintFinding `json:"findings"`
	// Edit is the command that previews new routing for the skill.
	Edit string `json:"edit"`
}

// SkillLintResult lists skills by how much their routing needs work: active
// skills first, since agents are routed to them today.
type SkillLintResult struct {
	Result
	Skills []SkillLintEntry `json:"skills"`
}

// skillLintStateOrder puts the skills agents reach today first.
var skillLintStateOrder = map[string]int{"active": 0, "draft": 1, "deprecated": 2, "archived": 3}

// LintRouting runs the routing lint over the hub's skills, archived ones left
// out unless named. It only reads.
func (service SkillService) LintRouting(ctx context.Context, path string, input SkillLintInput) (SkillLintResult, error) {
	root, err := skill.ResolveWorkspace(path)
	if err != nil {
		return SkillLintResult{}, err
	}
	listed, err := service.ListSkills(ctx, path, "")
	if err != nil {
		return SkillLintResult{}, err
	}
	entries := []SkillLintEntry{}
	found := false
	for _, listedSkill := range listed.Skills {
		if listedSkill.ID == bundledOwnerID {
			continue
		}
		if input.SkillID != "" {
			if listedSkill.ID != input.SkillID {
				continue
			}
			found = true
		} else if listedSkill.State == "archived" {
			continue
		}
		entry, err := lintRoutingEntry(ctx, root, listedSkill)
		if err != nil {
			return SkillLintResult{}, err
		}
		if len(entry.Findings) > 0 || input.All || input.SkillID != "" {
			entries = append(entries, entry)
		}
	}
	if input.SkillID != "" && !found {
		return SkillLintResult{Result: ErrorResult(NewInvalidRequestError(fmt.Sprintf("the hub has no skill %s", input.SkillID), "List the skills with `skillstation skill list`.")), Skills: []SkillLintEntry{}}, nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if skillLintStateOrder[a.State] != skillLintStateOrder[b.State] {
			return skillLintStateOrder[a.State] < skillLintStateOrder[b.State]
		}
		if len(a.Findings) != len(b.Findings) {
			return len(a.Findings) > len(b.Findings)
		}
		return a.ID < b.ID
	})
	result := SkillLintResult{Result: NewResult(StatusOK, skillLintSummary(entries)), Skills: entries}
	for _, entry := range entries {
		result.Items = append(result.Items, Item{ID: entry.ID, Summary: fmt.Sprintf("%s, %s", entry.State, pluralize(len(entry.Findings), "finding")), Impact: entry.Edit})
	}
	return result, nil
}

func lintRoutingEntry(ctx context.Context, root string, listed SkillListEntry) (SkillLintEntry, error) {
	_, relDir, meta, err := locateSkillDir(root, listed.ID)
	if err != nil {
		return SkillLintEntry{}, err
	}
	skillMDPath := filepath.Join(root, filepath.FromSlash(relDir), "SKILL.md")
	skillMD, err := os.ReadFile(skillMDPath)
	if err != nil {
		return SkillLintEntry{}, err
	}
	entry := SkillLintEntry{
		ID: listed.ID, State: listed.State, Name: listed.Name, SkillMD: skillMDPath, Findings: []SkillLintFinding{},
		Edit: fmt.Sprintf("skillstation skill edit %s --trigger <t>... --example <request>... --not-for <n>... --counter-example <request>... --json", listed.ID),
	}
	if target, decodeErr := decodeProposedSkill(listed.ID, meta, skillMD); decodeErr == nil {
		entry.Description = target.Description
		entry.Routing = SkillLintRouting{
			Triggers: nonNil(target.Triggers), Examples: nonNil(target.Examples), CounterExamples: nonNil(target.CounterExamples),
			NotFor: nonNil(target.NotFor), MinScope: target.MinScope,
		}
	}
	findings, _, err := lintProposedSkill(ctx, root, listed.ID, meta, skillMD)
	if err != nil {
		entry.Findings = append(entry.Findings, SkillLintFinding{Code: "lint_skipped", Message: "Routing lint skipped: " + err.Error(), Fix: "Fix the skill's metadata, then lint again."})
		return entry, nil
	}
	for _, finding := range findings {
		// A check of another skill's examples is reported under that skill.
		if finding.SkillID != "" && finding.SkillID != listed.ID {
			continue
		}
		entry.Findings = append(entry.Findings, SkillLintFinding{
			Code: finding.Code, Field: finding.Field, Value: finding.Value, OtherSkillID: finding.OtherSkillID,
			Message: finding.Message, Fix: finding.Fix,
		})
	}
	return entry, nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func skillLintSummary(entries []SkillLintEntry) string {
	counts := map[string]int{}
	for _, entry := range entries {
		if len(entry.Findings) > 0 {
			counts[entry.State]++
		}
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	if total == 0 {
		return "Routing lint: no findings."
	}
	verb := "need"
	if total == 1 {
		verb = "needs"
	}
	return fmt.Sprintf("%s %s routing work (%d active, %d draft, %d deprecated).", pluralize(total, "skill"), verb, counts["active"], counts["draft"], counts["deprecated"])
}

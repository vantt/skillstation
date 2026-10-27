package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLintRoutingListsActiveSkillsFirstAndOnlyThoseWithFindings(t *testing.T) {
	t.Parallel()
	hub := newGitHub(t)
	clean := filepath.Join(hub, "skills", "default", "zeta-done")
	writeSkillMetadataAt(t, clean, "schema_version: 1\nid: zeta-done\nstatus: active\nrouting:\n  triggers:\n    - export billing invoices to csv\n    - download invoices for the accountant\n  operations:\n    - operate\n  not_for:\n    - editing invoice templates\n  min_scope: single_step\n  examples:\n    - give me last month's invoices as a spreadsheet\n    - download every invoice from march for the accountant\n    - I need the invoice totals in a csv file\n")
	writeProjectFile(t, clean, "SKILL.md", "---\nname: zeta-done\ndescription: Export billing invoices to CSV files for accounting.\n---\n\n# zeta-done\n", 0o644)
	seedHubSkill(t, hub, fixtureSkill{id: "alpha-draft", state: "draft"})
	weak := filepath.Join(hub, "skills", "default", "beta-weak")
	writeSkillMetadataAt(t, weak, "schema_version: 1\nid: beta-weak\nstatus: active\nrouting:\n  triggers:\n    - beta\n  operations:\n    - convert\n  not_for:\n    - unrelated work\n  min_scope: single_step\n")
	writeProjectFile(t, weak, "SKILL.md", "---\nname: beta-weak\ndescription: Does beta things. Use when you need beta.\n---\n\n# beta-weak\n", 0o644)
	commitHubAll(t, hub, "seed")

	result, err := (SkillService{}).LintRouting(context.Background(), hub, SkillLintInput{})
	if err != nil || result.Error != nil {
		t.Fatalf("lint = %+v, %v", result.Error, err)
	}
	var order []string
	for _, entry := range result.Skills {
		order = append(order, entry.State+":"+entry.ID)
	}
	if len(order) != 2 || order[0] != "active:beta-weak" || order[1] != "draft:alpha-draft" {
		t.Fatalf("order = %v, want the weak active skill first, then the draft, and no clean skill", order)
	}
	codes := map[string]bool{}
	for _, finding := range result.Skills[0].Findings {
		codes[finding.Code] = true
	}
	if !codes["missing_examples"] || !codes["generic_trigger"] || !codes["unsupported_operation"] {
		t.Fatalf("beta-weak findings = %+v", result.Skills[0].Findings)
	}
	if result.Skills[0].SkillMD == "" || len(result.Skills[0].Routing.Triggers) != 1 {
		t.Fatalf("entry lacks what an agent drafts from: %+v", result.Skills[0])
	}

	all, err := (SkillService{}).LintRouting(context.Background(), hub, SkillLintInput{All: true})
	if err != nil || len(all.Skills) != 3 || all.Skills[2].ID != "alpha-draft" {
		t.Fatalf("--all = %+v, %v", all.Skills, err)
	}
	one, err := (SkillService{}).LintRouting(context.Background(), hub, SkillLintInput{SkillID: "zeta-done"})
	if err != nil || len(one.Skills) != 1 || len(one.Skills[0].Findings) != 0 {
		t.Fatalf("named clean skill = %+v, %v", one.Skills, err)
	}
	if missing, _ := (SkillService{}).LintRouting(context.Background(), hub, SkillLintInput{SkillID: "nope"}); missing.Error == nil {
		t.Fatalf("an unknown skill must be refused")
	}
}

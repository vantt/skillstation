package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsThatHideAClientSkillAreReported(t *testing.T) {
	t.Parallel()
	hidden := map[string]string{
		"whole tool":      `{"permissions":{"deny":["Skill"]}}`,
		"exact":           `{"permissions":{"deny":["Skill(skillstation-send)"]}}`,
		"with arguments":  `{"permissions":{"deny":["Skill(skillstation-send *)"]}}`,
		"parameter form":  `{"permissions":{"deny":["Skill(skill:skillstation-send)"]}}`,
		"glob":            `{"permissions":{"deny":["Skill(skillstation-*)"]}}`,
		"override off":    `{"skillOverrides":{"skillstation-send":"off"}}`,
		"override hidden": `{"skillOverrides":{"skillstation-send":"user-invocable-only"}}`,
	}
	for name, settings := range hidden {
		if hidingSetting([]byte(settings), "skillstation-send") == "" {
			t.Errorf("%s: %s hides the skill but was not reported", name, settings)
		}
	}
	visible := []string{
		`{"permissions":{"deny":["Skill(deploy)","Bash(rm *)"]}}`,
		`{"permissions":{"allow":["Skill"]}}`,
		`{"skillOverrides":{"skillstation-send":"name-only"}}`,
		`not json`,
	}
	for _, settings := range visible {
		if why := hidingSetting([]byte(settings), "skillstation-send"); why != "" {
			t.Errorf("%s reported as hiding the skill: %s", settings, why)
		}
	}
}

func TestHiddenSkillFindingNamesTheFileAndEditsNothing(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	settings := filepath.Join(project, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"permissions":{"deny":["Skill(skillstation-send)"]}}`
	if err := os.WriteFile(settings, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	findings := hiddenSkillFindings(project, []string{filepath.Join(project, "missing.json"), settings})
	if len(findings) != 1 || findings[0].Code != "client_skill_hidden" || findings[0].Fix != nil ||
		!strings.Contains(findings[0].Message, settings) || !strings.Contains(findings[0].Message, "skillstation-send") {
		t.Fatalf("findings: %+v", findings)
	}
	if after, _ := os.ReadFile(settings); string(after) != body {
		t.Fatalf("the settings file was changed: %s", after)
	}
}

package app

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vantt/skillstation/internal/hostintegration"
	"github.com/vantt/skillstation/internal/systemskills"
)

// claudeSettings is the part of a Claude Code settings file that can keep an
// agent from loading a skill.
type claudeSettings struct {
	Permissions struct {
		Deny []string `json:"deny"`
	} `json:"permissions"`
	SkillOverrides map[string]string `json:"skillOverrides"`
}

// claudeUserDir is where Claude Code keeps the user's settings.
func claudeUserDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// claudeManagedDir is where an organization's managed settings live.
func claudeManagedDir() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "windows":
		return `C:\Program Files\ClaudeCode`
	default:
		return "/etc/claude-code"
	}
}

// claudeSettingsFiles lists the settings files that apply to a connection,
// from the one that wins down: managed (with its drop-ins), the project's
// local and shared files (project scope only), then the user's.
func claudeSettingsFiles(root string, scope hostintegration.Scope, userDir, managedDir string) []string {
	var files []string
	if managedDir != "" {
		files = append(files, filepath.Join(managedDir, "managed-settings.json"))
		dropIns, _ := filepath.Glob(filepath.Join(managedDir, "managed-settings.d", "*.json"))
		files = append(files, dropIns...)
	}
	if scope == hostintegration.ScopeProject {
		files = append(files, filepath.Join(root, ".claude", "settings.local.json"), filepath.Join(root, ".claude", "settings.json"))
	}
	if userDir != "" {
		files = append(files, filepath.Join(userDir, "settings.json"))
	}
	return files
}

// denyRuleHides reports whether a permission rule keeps an agent from using
// the skill: `Skill` denies the whole tool, `Skill(name)`, `Skill(name *)` and
// `Skill(skill:name)` deny one skill, and the name may use `*`.
func denyRuleHides(rule, id string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "Skill" {
		return true
	}
	inner, ok := strings.CutPrefix(rule, "Skill(")
	if !ok || !strings.HasSuffix(inner, ")") {
		return false
	}
	inner = strings.TrimSuffix(inner, ")")
	inner = strings.TrimPrefix(inner, "skill:")
	inner = strings.TrimSuffix(inner, " *")
	matched, err := path.Match(inner, id)
	return err == nil && matched
}

// hidingSetting returns what in a settings file keeps the skill from agents,
// or "" when nothing does. A file that is missing or not JSON hides nothing.
func hidingSetting(raw []byte, id string) string {
	var settings claudeSettings
	if json.Unmarshal(raw, &settings) != nil {
		return ""
	}
	for _, rule := range settings.Permissions.Deny {
		if denyRuleHides(rule, id) {
			return `the deny rule "` + rule + `"`
		}
	}
	if mode := settings.SkillOverrides[id]; mode == "off" || mode == "user-invocable-only" {
		return `skillOverrides "` + mode + `"`
	}
	return ""
}

// hiddenSkillFindings reports the skills connect wrote that Claude Code
// settings keep an agent from loading. The files are there and current, so
// the other checks pass, yet asking the agent to send a skill does nothing.
// Settings belong to the person or the organization, so this only says which
// file does it and never changes one.
func hiddenSkillFindings(root string, files []string) []Finding {
	if IsHubRoot(root) {
		return nil
	}
	var findings []Finding
	for _, skill := range systemskills.SystemSkills() {
		if !skill.ClientSide {
			continue
		}
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			if why := hidingSetting(raw, skill.ID); why != "" {
				findings = append(findings, Finding{Code: "client_skill_hidden", Label: "Claude Code skill settings", Severity: SeverityProblem,
					Message: file + " has " + why + ", so Claude Code will not load " + skill.ID + " here even though its files are current. Remove that rule from the file if you want agents to send and fix skills; Skillstation does not edit Claude settings.",
					Manual:  "edit " + file})
				break
			}
		}
	}
	return findings
}

// claudeSkillSettingFindings is hiddenSkillFindings over the settings files
// of this machine.
func claudeSkillSettingFindings(root string, scope hostintegration.Scope) []Finding {
	return hiddenSkillFindings(root, claudeSettingsFiles(root, scope, claudeUserDir(), claudeManagedDir()))
}

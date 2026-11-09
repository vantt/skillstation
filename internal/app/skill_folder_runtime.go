package app

import (
	"fmt"
	"sort"

	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/skillruntime"
)

// declareRuntimeInFolderFix is the fix for a missing runtime block in a folder
// the author holds: the runtime is theirs to declare, and skill.yaml travels
// with the skill when it is sent.
const declareRuntimeInFolderFix = "Declare a `runtime:` block (requires.bins, requires.env, setup.command, setup.check) in the folder's skill.yaml; it is sent with the skill"

// folderRuntimeIssues runs the runtime hints over a skill folder before it is
// sent. meta is the skill.yaml (or the merged skill metadata) the runtime block
// is read from. A block that does not parse is its own finding, and counts as
// declared so the author is not also told it is missing.
func folderRuntimeIssues(skillID string, tree map[string]treeFile, meta []byte) []runtimeHintIssue {
	var issues []runtimeHintIssue
	_, hasSpec, err := parseRuntimeSpec(meta)
	if err != nil {
		hasSpec = true
		issues = append(issues, runtimeHintIssue{
			Code:    "invalid_runtime_block",
			Message: fmt.Sprintf("%s: the runtime block in skill.yaml is invalid: %v", skillID, err),
			Fix:     "Correct the `runtime:` block in the folder's skill.yaml (requires.bins, requires.env, requires.platforms, setup.command, setup.check)",
		})
	}
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	files := make([]skillruntime.HintFile, 0, len(paths))
	for _, path := range paths {
		content := tree[path].Data
		if len(content) > hintReadLimit {
			content = content[:hintReadLimit]
		}
		files = append(files, skillruntime.HintFile{Path: path, Content: content})
	}
	hints := skillruntime.AnalyzeHints(files, hasSpec)
	return append(issues, runtimeHintIssues(skillID, hints, declareRuntimeInFolderFix)...)
}

// folderSettings is the skill.yaml of a folder, empty when it has none.
func folderSettings(tree map[string]treeFile) []byte {
	return tree[skillmeta.SettingsFile].Data
}

func folderRuntimeLintFindings(skillID string, tree map[string]treeFile) []SkillLintFinding {
	findings := []SkillLintFinding{}
	for _, issue := range folderRuntimeIssues(skillID, tree, folderSettings(tree)) {
		findings = append(findings, SkillLintFinding{Code: issue.Code, Message: issue.Message, Fix: issue.Fix})
	}
	return findings
}

func folderRuntimeWarnings(skillID string, tree map[string]treeFile, meta []byte) []Warning {
	return runtimeIssueWarnings(folderRuntimeIssues(skillID, tree, meta))
}

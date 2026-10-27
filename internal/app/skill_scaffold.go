package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
)

// SkillScaffoldInput holds the answers of the create form, or the flags that
// stand in for it.
type SkillScaffoldInput struct {
	SkillID    string `json:"skill_id"`
	ProjectDir string `json:"project_dir"`
	// Name is the title of the skill; it defaults to the id.
	Name string `json:"name,omitempty"`
	// Description says what the skill does.
	Description string   `json:"description"`
	Triggers    []string `json:"triggers,omitempty"`
	NotFor      []string `json:"not_for,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	// Hub is the hub on this machine the project uses, if known; the hub then
	// records the new folder as an open draft.
	Hub string `json:"hub,omitempty"`
}

// SkillScaffoldResult reports the folder a create wrote in the project.
type SkillScaffoldResult struct {
	Result
	SkillID string `json:"skill_id"`
	// Path is the SKILL.md file, relative to the project root.
	Path string `json:"path"`
	// Folder is the absolute folder that holds the skill's files.
	Folder      string `json:"folder"`
	ProjectRoot string `json:"project_root"`
	// ClaudePointer is the SKILL.md, relative to the project root, that points
	// Claude Code at Folder. It holds no skill content.
	ClaudePointer string `json:"claude_pointer"`
	// GitignoreAdded lists the lines added to the project's .gitignore, as a
	// checkout adds them.
	GitignoreAdded []string `json:"gitignore_added,omitempty"`
}

// ScaffoldProjectSkill writes .agents/skills/<id>/ in the project: SKILL.md,
// and skill.yaml with the routing the form gave. Claude Code reads the folder
// through a pointer in .claude/skills/<id>/. Nothing changes on the hub except
// its record of open drafts. The folder carries no checkout marker, so its
// first commit is a new skill. An existing folder is refused, never
// overwritten.
func ScaffoldProjectSkill(ctx context.Context, input SkillScaffoldInput) (SkillScaffoldResult, error) {
	id := strings.TrimSpace(input.SkillID)
	if !skillTargetIDPattern.MatchString(id) {
		return scaffoldError(NewInvalidRequestError(fmt.Sprintf("invalid skill id %q", id), "Use lowercase letters, digits and hyphens, starting with a letter or digit (for example commit-style).")), nil
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return scaffoldError(NewInvalidRequestError("a description of what the skill does is required", "Pass --description \"<what the skill does>\".")), nil
	}
	projectRoot := projectRootOf(ctx, input.ProjectDir)
	shared, claude := projectSkillFolders(projectRoot, id)
	if refusal := newProjectFolderRefusal(shared, claude, id); refusal != nil {
		return scaffoldError(refusal), nil
	}
	if err := os.MkdirAll(shared.Dir, 0o755); err != nil {
		return SkillScaffoldResult{}, err
	}
	skillMD := []byte(scaffoldSkillMD(id, input))
	if err := os.WriteFile(filepath.Join(shared.Dir, "SKILL.md"), skillMD, 0o644); err != nil {
		return SkillScaffoldResult{}, err
	}
	settings, err := scaffoldSettings(input)
	if err != nil {
		return SkillScaffoldResult{}, err
	}
	if err := os.WriteFile(filepath.Join(shared.Dir, skillmeta.SettingsFile), settings, 0o644); err != nil {
		return SkillScaffoldResult{}, err
	}
	// Like a checkout, the folder is the person's work in progress and the
	// hub is where the skill lives, so the project's Git leaves it out.
	placement, err := placeClaudePointer(projectRoot, id, skillMD)
	if err != nil {
		return SkillScaffoldResult{}, err
	}
	if hub := strings.TrimSpace(input.Hub); hub != "" && IsHubRoot(hub) {
		if err := recordDraft(ctx, hub, DraftRecord{SkillID: id, ProjectRoot: projectRoot, Folder: shared.Rel, Pointer: placement.Pointer, StartedAt: time.Now().UTC()}); err != nil {
			return SkillScaffoldResult{}, err
		}
	}
	relative := shared.Rel + "/SKILL.md"
	return SkillScaffoldResult{
		Result:         NewResult(StatusOK, fmt.Sprintf("Created %s/ with SKILL.md and skill.yaml (nothing changed on the hub).", shared.Rel)),
		SkillID:        id,
		Path:           relative,
		Folder:         shared.Dir,
		ProjectRoot:    projectRoot,
		ClaudePointer:  placement.Pointer,
		GitignoreAdded: placement.GitignoreAdded,
	}, nil
}

func scaffoldError(err *Error) SkillScaffoldResult {
	return SkillScaffoldResult{Result: ErrorResult(err)}
}

// scaffoldSkillMD is the starter SKILL.md. The description says what the skill
// does and, for each trigger, a sentence that reads as English whatever form
// the person wrote it in (see useSentence), so an agent that reads the folder
// itself knows when to use it. The routing the hub uses is in skill.yaml. The
// scaffold marker keeps an untouched starter from being activated.
func scaffoldSkillMD(id string, input SkillScaffoldInput) string {
	title := strings.TrimSpace(input.Name)
	if title == "" {
		title = id
	}
	description := strings.TrimSpace(input.Description)
	if triggers := cleanPhrases(input.Triggers); len(triggers) > 0 {
		sentences := make([]string, 0, len(triggers))
		for _, trigger := range triggers {
			sentences = append(sentences, useSentence(trigger))
		}
		description = strings.TrimRight(description, ". ") + ". " + strings.Join(sentences, " ")
	}
	var out strings.Builder
	fmt.Fprintf(&out, "---\nname: %s\ndescription: %s\n---\n\n# %s\n\n", id, yamlScalar(description), title)
	fmt.Fprintf(&out, "## Instructions\n\n%s\nReplace this paragraph and the comment above it with the steps the agent should follow.\n", skill.ScaffoldMarker)
	return out.String()
}

// Words a trigger can start with that already read as a condition ("after a
// deploy", "when the build fails") or as its subject ("the user asks ...").
var (
	conditionWords = map[string]bool{"when": true, "whenever": true, "after": true, "before": true, "if": true, "once": true, "while": true, "during": true, "for": true, "on": true, "in": true, "at": true, "as": true}
	subjectWords   = map[string]bool{"the": true, "a": true, "an": true, "you": true, "your": true, "someone": true, "somebody": true, "user": true, "users": true, "i": true, "we": true, "they": true, "it": true, "there": true, "my": true, "our": true}
)

// useSentence turns one answer to "When should an agent use it?" into a
// sentence: "after a deploy" gives "Use after a deploy.", "the user asks for
// a review" gives "Use when the user asks for a review.", and a task such as
// "write a commit message" gives "Use it to write a commit message.".
func useSentence(trigger string) string {
	trigger = strings.TrimRight(strings.TrimSpace(trigger), ".")
	first := strings.ToLower(strings.Fields(trigger)[0])
	switch {
	case conditionWords[first]:
		return "Use " + trigger + "."
	case subjectWords[first]:
		return "Use when " + trigger + "."
	}
	return "Use it to " + trigger + "."
}

// scaffoldSettings is the starter skill.yaml: the routing the form gave, with
// every list present so the person sees where to add more.
func scaffoldSettings(input SkillScaffoldInput) ([]byte, error) {
	list := func(items []string) []string {
		if cleaned := cleanPhrases(items); len(cleaned) > 0 {
			return cleaned
		}
		return []string{}
	}
	document := map[string]any{
		"schema_version": 1,
		"routing": map[string]any{
			"triggers":         list(input.Triggers),
			"not_for":          list(input.NotFor),
			"examples":         list(input.Examples),
			"counter_examples": []string{},
		},
	}
	merged, err := yaml.Marshal(document)
	if err != nil {
		return nil, err
	}
	settings, _, err := skillmeta.Split(merged)
	return settings, err
}

// yamlScalar quotes a frontmatter value only when plain YAML would misread it.
func yamlScalar(text string) string {
	if strings.ContainsAny(text, ":#\"'\n") || strings.HasPrefix(text, "-") || strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{") {
		return fmt.Sprintf("%q", text)
	}
	return text
}

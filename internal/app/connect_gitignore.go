package app

import (
	"context"
	"path/filepath"

	"github.com/vantt/skillstation/internal/hostintegration"
	"github.com/vantt/skillstation/internal/systemskills"
)

const systemSkillGitignoreComment = "# skillstation generated skills"

// GitignoreItemPrefix starts the ID of a result item that names a line connect
// added to the project's .gitignore.
const GitignoreItemPrefix = "gitignore:"

// keepSystemSkillsOutOfGit ignores, in the Git work tree that holds a project,
// the folders connect generates for the skills a project's agents use. Each
// machine writes them from its own binary, so a copy committed to the project
// would freeze one version for everyone who clones it. It reports the lines it
// added. A user-scope connection, the hub folder and a folder outside Git are
// left alone.
func keepSystemSkillsOutOfGit(ctx context.Context, root string, scope hostintegration.Scope, isHub bool) ([]Item, error) {
	if scope != hostintegration.ScopeProject || isHub {
		return nil, nil
	}
	top, ok := gitTopLevel(ctx, root)
	if !ok {
		return nil, nil
	}
	prefix := ""
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if relative, err := filepath.Rel(top, root); err == nil && relative != "." {
		prefix = filepath.ToSlash(relative) + "/"
	}
	var items []Item
	for _, skill := range systemskills.SystemSkills() {
		if !skill.ClientSide {
			continue
		}
		for _, folder := range []string{".agents/skills/" + skill.ID + "/", ".claude/skills/" + skill.ID + "/"} {
			added, err := ensureGitignoredUnder(top, prefix+folder, systemSkillGitignoreComment)
			if err != nil {
				return items, err
			}
			if added {
				items = append(items, Item{ID: GitignoreItemPrefix + prefix + folder, Summary: ".gitignore: " + prefix + folder, Impact: "Added."})
			}
		}
	}
	return items, nil
}

// appendGitignoreItems adds the .gitignore lines connect wrote to result.
func appendGitignoreItems(ctx context.Context, result *Result, root string, scope hostintegration.Scope, isHub bool) error {
	items, err := keepSystemSkillsOutOfGit(ctx, root, scope, isHub)
	result.Items = append(result.Items, items...)
	return err
}

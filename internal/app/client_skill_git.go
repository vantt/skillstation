package app

import (
	"context"
	"strings"

	"github.com/vantt/skillstation/internal/hostintegration"
	"github.com/vantt/skillstation/internal/systemskills"
)

// clientSkillFolders are the folders, relative to a project and with forward
// slashes, where connect generates the skills a project's agents use.
func clientSkillFolders() []string {
	var folders []string
	for _, skill := range systemskills.SystemSkills() {
		if skill.ClientSide {
			folders = append(folders, ".agents/skills/"+skill.ID+"/", ".claude/skills/"+skill.ID+"/")
		}
	}
	return folders
}

// skillGitFindings checks how Git sees the skill files connect generated in a
// project. Each machine writes them from its own binary, so they belong
// outside Git: a copy that is tracked freezes one version for everyone who
// clones the project, and a copy that is neither tracked nor ignored is one
// careless `git add .` away from being committed. The hub folder has its own
// check, and a user-scope connection or a folder outside Git has nothing to
// report.
func skillGitFindings(ctx context.Context, root string, scope hostintegration.Scope) []Finding {
	if scope != hostintegration.ScopeProject || IsHubRoot(root) {
		return nil
	}
	if _, ok := gitTopLevel(ctx, root); !ok {
		return nil
	}
	folders := clientSkillFolders()
	var findings []Finding
	tracked, err := gitPaths(ctx, root, append([]string{"ls-files", "-z", "--"}, folders...)...)
	if err != nil {
		return nil
	}
	if owned := ownedGenerated(root, tracked); len(owned) > 0 {
		findings = append(findings, Finding{Code: "client_skill_tracked", Label: "Skill files in Git", Severity: SeverityNotice,
			Message: "Git tracks " + strings.Join(owned, ", ") + ", which each machine writes from its own Skillstation version; a tracked copy goes stale for everyone else. Stop tracking them with the command below, then commit.",
			Manual:  untrackCommand(root, owned)})
	}
	loose, err := gitPaths(ctx, root, append([]string{"ls-files", "-z", "--others", "--exclude-standard", "--"}, folders...)...)
	if err != nil {
		return findings
	}
	if owned := ownedGenerated(root, loose); len(owned) > 0 {
		fix := &FixAction{Summary: "add the generated skill folders to .gitignore"}
		fix.Apply = func(ctx context.Context) (Changed, error) {
			items, err := keepSystemSkillsOutOfGit(ctx, root, hostintegration.ScopeProject, false)
			var written Changed
			for _, item := range items {
				written = append(written, strings.TrimPrefix(item.ID, GitignoreItemPrefix))
			}
			return written, err
		}
		findings = append(findings, Finding{Code: "client_skill_not_ignored", Label: "Skill files in Git", Severity: SeverityNotice,
			Message: strings.Join(owned, ", ") + " are neither tracked nor ignored, so a `git add .` would commit a copy that goes stale; Skillstation writes them on each machine.",
			Fix:     fix, Manual: "skillstation connect"})
	}
	return findings
}

// ownedGenerated keeps the paths whose bytes Skillstation wrote, so a file a
// person put in the same folder is never reported or touched.
func ownedGenerated(root string, paths []string) []string {
	var owned []string
	for _, path := range paths {
		if raw, ok := readGeneratedFile(root, path); ok && hostintegration.SystemSkillFileOwned(path, raw) {
			owned = append(owned, path)
		}
	}
	return sortedUnique(owned)
}

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vantt/skillstation/internal/hostintegration"
)

// A project keeps the skill it edits in the shared Agent Skills folder, which
// Codex, Gemini CLI, Antigravity, pi and omp read. Claude Code reads only
// .claude/skills, so it gets a pointer SKILL.md there that names the shared
// folder. Releases before this one put the checkout itself in .claude/skills,
// and a person may keep a copy of their own there too, so every command that
// looks up a project skill by id looks in both places and tells them apart.
const (
	projectSkillsDir = hostintegration.SharedSkillsDirRelativePath
	claudeSkillsDir  = ".claude/skills"
)

// projectFolderKind says what one of the two places holds for a skill id.
type projectFolderKind int

const (
	_ projectFolderKind = iota // zero value: the place holds nothing
	// folderPointer is the SKILL.md Skillstation wrote for Claude Code.
	folderPointer
	// folderCheckout is a copy with a checkout marker.
	folderCheckout
	// folderCopy is a skill folder without a marker: created in the project or
	// copied there by hand.
	folderCopy
	// folderOther is anything else at that path: a file, a link, a folder
	// without SKILL.md, or a marker this version cannot read.
	folderOther
)

// projectFolder is what one of the two places holds for a skill id.
type projectFolder struct {
	// Rel is the folder relative to the project root, with slashes.
	Rel    string
	Dir    string
	Kind   projectFolderKind
	Marker CheckoutMarker
}

// real reports whether the folder holds skill files a commit could read.
func (folder projectFolder) real() bool {
	return folder.Kind == folderCheckout || folder.Kind == folderCopy
}

// describe names what the folder holds, for messages that list both places.
func (folder projectFolder) describe() string {
	switch folder.Kind {
	case folderPointer:
		return folder.Rel + "/ is the pointer Skillstation wrote for Claude Code"
	case folderCheckout:
		return fmt.Sprintf("%s/ is a checkout from %s", folder.Rel, folder.Marker.Hub)
	case folderCopy:
		return folder.Rel + "/ is a copy that was not checked out from a hub"
	case folderOther:
		return folder.Rel + " is in the way and is not a skill folder Skillstation can read"
	}
	return folder.Rel + "/ does not exist"
}

// projectSkillFolders reports what the shared folder and the Claude Code
// folder of the project hold for id, in that order.
func projectSkillFolders(projectRoot, id string) (shared, claude projectFolder) {
	return inspectProjectFolder(projectRoot, projectSkillsDir, id), inspectProjectFolder(projectRoot, claudeSkillsDir, id)
}

func inspectProjectFolder(projectRoot, parent, id string) projectFolder {
	rel := parent + "/" + id
	folder := projectFolder{Rel: rel, Dir: filepath.Join(projectRoot, filepath.FromSlash(rel))}
	info, err := os.Lstat(folder.Dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return folder
	case err != nil || !info.IsDir():
		folder.Kind = folderOther
		return folder
	}
	marker, found, err := ReadCheckoutMarker(folder.Dir)
	switch {
	case err != nil:
		folder.Kind = folderOther
	case found:
		folder.Kind, folder.Marker = folderCheckout, marker
	case isPointerFolder(folder.Dir, id):
		folder.Kind = folderPointer
	default:
		if _, err := os.Stat(filepath.Join(folder.Dir, "SKILL.md")); err != nil {
			folder.Kind = folderOther
		} else {
			folder.Kind = folderCopy
		}
	}
	return folder
}

// isPointerFolder reports whether dir holds nothing but a pointer SKILL.md
// that Skillstation wrote for id.
func isPointerFolder(dir, id string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "SKILL.md" || !entries[0].Type().IsRegular() {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	return err == nil && hostintegration.IsSkillPointer(id, raw)
}

// newProjectFolderRefusal explains why a checkout or create cannot write id:
// one of the two places already holds something other than a pointer.
func newProjectFolderRefusal(shared, claude projectFolder, id string) *Error {
	// A bare id commits the folder only when it is the one copy; otherwise the
	// fix names the folder by its path.
	commitArg := func(folder projectFolder) string {
		if shared.real() && claude.real() {
			return "./" + folder.Rel
		}
		return id
	}
	for _, folder := range []projectFolder{shared, claude} {
		switch folder.Kind {
		case folderCheckout:
			return NewSkillConflictError(
				fmt.Sprintf("%s, so %s is already checked out", folder.describe(), id),
				fmt.Sprintf("Edit it in %s/, then run `skillstation skill commit %s`.", folder.Rel, commitArg(folder)),
			)
		case folderCopy:
			return NewSkillConflictError(
				fmt.Sprintf("%s, so a new copy would leave two versions of %s in this project", folder.describe(), id),
				fmt.Sprintf("Commit that folder with `skillstation skill commit %s`, or move it out of the project and run the command again.", commitArg(folder)),
			)
		case folderOther:
			return NewSkillConflictError(
				folder.describe(),
				fmt.Sprintf("Move %s out of the project and run the command again.", folder.Rel),
			)
		}
	}
	if shared.Kind == folderPointer {
		return NewSkillConflictError(
			fmt.Sprintf("%s/ holds only a pointer, which belongs in %s/", shared.Rel, claudeSkillsDir),
			fmt.Sprintf("Delete %s/ and run the command again.", shared.Rel),
		)
	}
	return nil
}

// projectSkillPlacement is where a checkout or create put a skill: the folder
// that holds its files and the pointer Claude Code reads.
type projectSkillPlacement struct {
	// Pointer is the pointer SKILL.md relative to the project root.
	Pointer string
	// GitignoreAdded lists the lines added to the project's .gitignore.
	GitignoreAdded []string
}

// placeClaudePointer writes the pointer SKILL.md in .claude/skills/<id>/ and
// keeps both folders out of the project's Git. Only an absent folder or an
// earlier pointer is written: the caller refused everything else.
func placeClaudePointer(projectRoot, id string, skillMD []byte) (projectSkillPlacement, error) {
	pointer, err := hostintegration.RenderSkillPointer(id, skillMD)
	if err != nil {
		return projectSkillPlacement{}, err
	}
	dir := filepath.Join(projectRoot, filepath.FromSlash(claudeSkillsDir), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return projectSkillPlacement{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), pointer, 0o644); err != nil {
		return projectSkillPlacement{}, err
	}
	placement := projectSkillPlacement{Pointer: claudeSkillsDir + "/" + id + "/SKILL.md"}
	for _, parent := range []string{projectSkillsDir, claudeSkillsDir} {
		pattern := "/" + parent + "/" + id + "/"
		added, err := ensureGitignored(projectRoot, pattern)
		if err != nil {
			return projectSkillPlacement{}, err
		}
		if added {
			placement.GitignoreAdded = append(placement.GitignoreAdded, pattern)
		}
	}
	return placement, nil
}

// resolveProjectSkillByID finds the one folder a bare id names for commit and
// pull. Two folders with skill files are refused rather than guessed between.
func resolveProjectSkillByID(projectRoot, id string) (string, *Error) {
	shared, claude := projectSkillFolders(projectRoot, id)
	switch {
	case shared.real() && claude.real():
		return "", NewSkillConflictError(
			fmt.Sprintf("this project has two copies of %s: %s; %s", id, shared.describe(), claude.describe()),
			fmt.Sprintf("Name the one you mean by its path, such as `./%s`, and move the other out of the project.", shared.Rel),
		)
	case shared.real():
		return shared.Dir, nil
	case claude.real():
		return claude.Dir, nil
	case claude.Kind == folderPointer:
		return "", NewInvalidRequestError(
			fmt.Sprintf("%s; the skill files it points to in %s/ are gone", claude.describe(), shared.Rel),
			fmt.Sprintf("Check the skill out again with `skillstation skill checkout %s`, or delete %s/.", id, claude.Rel),
		)
	}
	var problems []string
	for _, folder := range []projectFolder{shared, claude} {
		if folder.Kind == folderOther {
			problems = append(problems, folder.describe())
		}
	}
	detail := fmt.Sprintf("there is no %s/ or %s/ folder in this project", shared.Rel, claude.Rel)
	if len(problems) > 0 {
		detail = strings.Join(problems, "; ")
	}
	return "", NewInvalidRequestError(detail,
		fmt.Sprintf("Copy it here with `skillstation skill checkout %s`, start a new one with `skillstation skill create %s`, or name another folder by its path, such as `./%s`.", id, id, id),
	)
}

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/workspace"
)

const checkoutGitignoreComment = "# skillstation checkout"

// SkillCheckoutInput names the skill and the directory the command ran in.
type SkillCheckoutInput struct {
	SkillID    string `json:"skill_id"`
	ProjectDir string `json:"project_dir"`
}

// SkillCheckoutResult reports what a checkout copied into the project.
type SkillCheckoutResult struct {
	Result
	SkillID string `json:"skill_id"`
	State   string `json:"state"`
	// Path is the folder that holds the skill's files, relative to the project
	// root. It is the only copy to edit.
	Path string `json:"path"`
	// Folder is Path as an absolute path.
	Folder      string `json:"folder"`
	ProjectRoot string `json:"project_root"`
	// ClaudePointer is the SKILL.md, relative to the project root, that points
	// Claude Code at Path. It holds no skill content.
	ClaudePointer string `json:"claude_pointer"`
	FileCount     int    `json:"file_count"`
	BaseVersion   string `json:"base_version"`
	BaseCommit    string `json:"base_commit,omitempty"`
	// GitignoreAdded lists the lines the checkout added to the project's .gitignore.
	GitignoreAdded []string `json:"gitignore_added,omitempty"`
	// DraftRecorded says the hub on this machine knows the checkout: agents
	// in this project get it through skill_resolve and skill_get too.
	DraftRecorded bool `json:"checkout_recorded"`
}

// SkillCheckoutService copies a hub skill into a project to edit and try.
type SkillCheckoutService struct {
	Clock Clock
}

// Checkout copies every content file of a hub skill into
// <project>/.agents/skills/<id>/, records where it came from, and points Claude
// Code at it from <project>/.claude/skills/<id>/SKILL.md. The hub records the
// draft, so its agents in this project get the draft while every other
// project keeps the current version; nothing is committed on the hub.
func (service SkillCheckoutService) Checkout(ctx context.Context, path string, input SkillCheckoutInput) (SkillCheckoutResult, error) {
	if service.Clock == nil {
		service.Clock = SystemClock{}
	}
	root, err := workspace.Discover(path)
	if err != nil {
		return SkillCheckoutResult{}, err
	}
	id := strings.TrimSpace(input.SkillID)
	if !skillmeta.ValidSkillID(id) {
		return checkoutError(NewInvalidRequestError(fmt.Sprintf("invalid skill id %q", id), "Use the lowercase id shown by `skillstation skill list`.")), nil
	}
	_, relDir, metaBytes, err := locateSkillDir(root, id)
	if err != nil {
		return SkillCheckoutResult{}, err
	}
	projectRoot := projectRootOf(ctx, input.ProjectDir)
	shared, claude := projectSkillFolders(projectRoot, id)
	if refusal := newProjectFolderRefusal(shared, claude, id); refusal != nil {
		return checkoutError(refusal), nil
	}
	tree, err := readHubSkillTree(root, relDir)
	if err != nil {
		return SkillCheckoutResult{}, err
	}
	baseCommit := ""
	if dirty, dirtyErr := workspace.DirtyPaths(ctx, root, []string{relDir}); dirtyErr == nil && len(dirty) == 0 {
		baseCommit = workspace.HeadCommit(ctx, root)
	}
	marker := CheckoutMarker{
		SkillID:      id,
		Hub:          root,
		BaseVersion:  skillBaseVersion(tree),
		BaseCommit:   baseCommit,
		CheckedOutAt: service.Clock.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	state := parseSkillReviewMeta(metaBytes).Status
	result, err := finishCheckout(projectRoot, shared, tree, marker, fmt.Sprintf("%s, %s", state, pluralize(len(tree), "file")), "")
	if err != nil || result.Error != nil {
		return result, err
	}
	result.State = state
	result.BaseCommit = marker.BaseCommit
	return result, nil
}

// finishCheckout writes the checked-out tree into the shared folder and the
// pointer for Claude Code, and reports where the files are.
func finishCheckout(projectRoot string, shared projectFolder, tree map[string]treeFile, marker CheckoutMarker, detail, from string) (SkillCheckoutResult, error) {
	if err := writeCheckoutFolder(shared.Dir, tree, marker); err != nil {
		return SkillCheckoutResult{}, err
	}
	var skillMD []byte
	if file, ok := tree["SKILL.md"]; ok {
		skillMD = file.Data
	}
	placement, err := placeClaudePointer(projectRoot, marker.SkillID, skillMD)
	if err != nil {
		return SkillCheckoutResult{}, err
	}
	// A hub on this machine records the draft; one on another machine only
	// knows the folder from the marker.
	recorded := from == "" && IsHubRoot(marker.Hub)
	if recorded {
		if err := recordDraft(context.Background(), marker.Hub, DraftRecord{SkillID: marker.SkillID, ProjectRoot: projectRoot, Folder: shared.Rel, Pointer: placement.Pointer, BaseVersion: marker.BaseVersion, StartedAt: time.Now().UTC()}); err != nil {
			return SkillCheckoutResult{}, err
		}
	}
	summary := fmt.Sprintf("Copied %s (%s) to %s/", marker.SkillID, detail, shared.Rel)
	if from != "" {
		summary = fmt.Sprintf("Copied %s (%s) from %s to %s/", marker.SkillID, detail, from, shared.Rel)
	}
	return SkillCheckoutResult{
		Result:         NewResult(StatusOK, summary),
		SkillID:        marker.SkillID,
		Path:           shared.Rel,
		Folder:         shared.Dir,
		ProjectRoot:    projectRoot,
		ClaudePointer:  placement.Pointer,
		FileCount:      len(tree),
		BaseVersion:    marker.BaseVersion,
		GitignoreAdded: placement.GitignoreAdded,
		DraftRecorded:  recorded,
	}, nil
}

func checkoutError(err *Error) SkillCheckoutResult {
	return SkillCheckoutResult{Result: ErrorResult(err)}
}

// writeCheckoutFolder writes the tree and the marker into a temporary folder
// beside target, then moves it into place, so a failed checkout leaves nothing
// that looks like a finished one.
func writeCheckoutFolder(target string, tree map[string]treeFile, marker CheckoutMarker) (err error) {
	parent := filepath.Dir(target)
	if err = os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".checkout-")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(staging)
		}
	}()
	for rel, file := range tree {
		if !safeTreePath(rel) {
			return fmt.Errorf("skill file path %q leaves the skill folder", rel)
		}
		full := filepath.Join(staging, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if file.Executable {
			mode = 0o755
		}
		if err = os.WriteFile(full, file.Data, mode); err != nil {
			return err
		}
		if err = os.Chmod(full, mode); err != nil {
			return err
		}
	}
	if err = WriteCheckoutMarker(staging, marker); err != nil {
		return err
	}
	if err = os.Chmod(staging, 0o755); err != nil {
		return err
	}
	return os.Rename(staging, target)
}

// projectRootOf is the Git top level of dir, or dir itself outside Git.
func projectRootOf(ctx context.Context, dir string) string {
	if top, ok := gitTopLevel(ctx, dir); ok {
		return top
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// gitTopLevel is the top level of the Git work tree that holds dir.
func gitTopLevel(ctx context.Context, dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	output, err := offlineGitCommand(ctx, abs, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	top := strings.TrimSpace(string(output))
	return filepath.FromSlash(top), top != ""
}

// ensureGitignored appends pattern to the project's .gitignore under a
// "# skillstation checkout" comment unless a line already covers it. It reports
// whether it wrote anything.
func ensureGitignored(projectRoot, pattern string) (bool, error) {
	path := filepath.Join(projectRoot, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	text := string(existing)
	covers := coveringIgnoreLines(pattern)
	hasComment := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if covers[line] {
			return false, nil
		}
		if line == checkoutGitignoreComment {
			hasComment = true
		}
	}
	var addition strings.Builder
	if text != "" && !strings.HasSuffix(text, "\n") {
		addition.WriteString("\n")
	}
	if !hasComment {
		if text != "" {
			addition.WriteString("\n")
		}
		addition.WriteString(checkoutGitignoreComment + "\n")
	}
	addition.WriteString(pattern + "\n")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	_, writeErr := file.WriteString(addition.String())
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	return writeErr == nil, writeErr
}

// coveringIgnoreLines lists the .gitignore lines that already ignore the
// folder a pattern names, from the exact line up to its top folder, such as
// .agents or .claude.
func coveringIgnoreLines(pattern string) map[string]bool {
	folder := strings.Trim(pattern, "/")
	covers := map[string]bool{pattern: true, folder: true, "/" + folder: true, folder + "/": true}
	for dir := folder; dir != "." && dir != ""; dir = filepath.ToSlash(filepath.Dir(dir)) {
		if dir != folder {
			covers[dir], covers["/"+dir], covers[dir+"/"], covers["/"+dir+"/"] = true, true, true, true
		}
	}
	return covers
}

func pluralize(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

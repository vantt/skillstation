package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vantt/skillstation/internal/canonical"
	"github.com/vantt/skillstation/internal/fsutil"
	"github.com/vantt/skillstation/internal/version"
)

// issuesFilePath is where a hub keeps the last canonical validation between
// processes, beside the catalog generations in the runtime folder that Git
// ignores. Like the stamp file it only saves work: it is read when its key
// matches the tree and the binary, and never trusted otherwise.
const issuesFilePath = "runtime/catalog/canonical-issues.json"

// issuesFileVersion changes whenever the meaning of a field changes; a file of
// another version is ignored and rewritten.
const issuesFileVersion = 1

// maxIssuesFileBytes bounds what is read; a larger file is ignored.
const maxIssuesFileBytes = 8 << 20

type issuesFile struct {
	Version int           `json:"version"`
	Key     string        `json:"key"`
	Issues  []storedIssue `json:"issues"`
}

type storedIssue struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
	Code    string `json:"code,omitempty"`
}

// storedKey ties a stored validation to the tree (validationKey) and to the
// binary that produced it, because a newer binary may judge the same files
// differently. The executable's size and modification time stand in for the
// version of a local build, which carries none.
func storedKey(treeKey string) string {
	info := version.Current()
	identity := fmt.Sprintf("%s|%s|%s", info.Version, info.Commit, info.Dirty)
	if executable, err := os.Executable(); err == nil {
		if stat, err := os.Stat(executable); err == nil {
			identity += fmt.Sprintf("|%d|%d", stat.Size(), stat.ModTime().UnixNano())
		}
	}
	return treeKey + "|" + identity
}

// readStoredIssues returns the validation another process stored for key, or
// false when there is none that matches.
func readStoredIssues(root, key string) ([]canonical.Issue, bool) {
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil, false
	}
	defer rootHandle.Close()
	info, err := rootHandle.Lstat(issuesFilePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxIssuesFileBytes {
		return nil, false
	}
	contents, err := rootHandle.ReadFile(issuesFilePath)
	if err != nil {
		return nil, false
	}
	var stored issuesFile
	if err := json.Unmarshal(contents, &stored); err != nil || stored.Version != issuesFileVersion || stored.Key != key {
		return nil, false
	}
	issues := make([]canonical.Issue, 0, len(stored.Issues))
	for _, item := range stored.Issues {
		issues = append(issues, canonical.Issue{Path: item.Path, Line: item.Line, Message: item.Message, Fix: item.Fix, Code: item.Code})
	}
	return issues, true
}

// writeStoredIssues records the validation for key. It does nothing unless the
// hub already has a catalog folder, so it never creates one, and a failed write
// is ignored: the file is only an optimisation.
func writeStoredIssues(root, key string, issues []canonical.Issue) {
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return
	}
	defer rootHandle.Close()
	for _, directory := range []string{"runtime", "runtime/catalog"} {
		info, err := rootHandle.Lstat(directory)
		if err != nil || !info.IsDir() {
			return
		}
	}
	stored := issuesFile{Version: issuesFileVersion, Key: key, Issues: make([]storedIssue, 0, len(issues))}
	for _, issue := range issues {
		stored.Issues = append(stored.Issues, storedIssue{Path: issue.Path, Line: issue.Line, Message: issue.Message, Fix: issue.Fix, Code: issue.Code})
	}
	contents, err := json.Marshal(stored)
	if err != nil {
		return
	}
	_ = fsutil.WriteFileAtomic(filepath.Join(root, filepath.FromSlash(issuesFilePath)), contents, 0o600)
}

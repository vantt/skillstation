package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vantt/skillstation/internal/canonical"
)

// maxIssueMemoRoots bounds the process-wide memo; a server normally reads one workspace.
const maxIssueMemoRoots = 16

// issueMemo remembers the canonical validation of the last tree seen per
// workspace. A full validation reads and parses every canonical file and asks
// Git for unmerged paths, so a page that shows one skill must not pay for it on
// every view of an unchanged hub.
var issueMemo = struct {
	sync.Mutex
	roots map[string]issueMemoEntry
}{roots: map[string]issueMemoEntry{}}

type issueMemoEntry struct {
	key    string
	issues []canonical.Issue
	// stored is true when the hub's issues file holds this result.
	stored bool
}

// CanonicalIssues is canonical.Validate for the hub at root, reused while every
// canonical file and the Git index are unchanged, so a command that checks the
// hub twice, as status does, reads and parses its files once.
func CanonicalIssues(root string) ([]canonical.Issue, error) {
	return canonicalIssues(root, canonical.Snapshot{}, false)
}

// CanonicalIssuesRemembered is CanonicalIssues for a command that may leave its
// result in the hub's runtime folder, so the next process that finds the
// canonical files and the binary unchanged skips the validation. A read-only
// command that must not write to the hub calls CanonicalIssues, which still
// reads a result stored earlier.
func CanonicalIssuesRemembered(root string) ([]canonical.Issue, error) {
	return canonicalIssues(root, canonical.Snapshot{}, true)
}

// SkillCanonicalIssues returns the canonical validation issues inside one skill
// folder (skillRelDir, slash-separated below root) as "path: message" lines.
// Validation covers the whole hub, because a skill can be invalid through
// another file (a duplicate id, a broken reference); its result is reused
// while every canonical file and the Git index are unchanged.
func SkillCanonicalIssues(root, skillRelDir string) ([]string, error) {
	return skillCanonicalIssues(root, skillRelDir, canonical.Snapshot{})
}

// skillCanonicalIssues is SkillCanonicalIssues for a caller that has just
// scanned the tree; an empty snapshot is scanned here.
func skillCanonicalIssues(root, skillRelDir string, scanned canonical.Snapshot) ([]string, error) {
	issues, err := canonicalIssues(root, scanned, false)
	if err != nil {
		return nil, err
	}
	prefix := skillRelDir + "/"
	var lines []string
	for _, issue := range issues {
		if strings.HasPrefix(issue.Path, prefix) {
			lines = append(lines, issue.Path+": "+issue.Message)
		}
	}
	return lines, nil
}

// canonicalIssues is canonical.Validate, reused for an unchanged tree: from the
// memo of this process or from the result an earlier process stored in the hub.
// The result is kept only when the tree had the same key before and after the
// validation, so a change made while validating is never recorded under the
// earlier key. It is stored in the hub only when remember is set.
func canonicalIssues(root string, scanned canonical.Snapshot, remember bool) ([]canonical.Issue, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve canonical workspace: %w", err)
	}
	before, cacheable := validationKey(root, scanned)
	if cacheable {
		issueMemo.Lock()
		entry, ok := issueMemo.roots[abs]
		issueMemo.Unlock()
		if ok && entry.key == before {
			// An earlier read in this process, such as the health sections of
			// status, may have validated without storing the result.
			if remember && !entry.stored {
				writeStoredIssues(root, storedKey(before), entry.issues)
				rememberIssues(abs, before, entry.issues, true)
			}
			return entry.issues, nil
		}
		if issues, found := readStoredIssues(root, storedKey(before)); found {
			rememberIssues(abs, before, issues, true)
			return issues, nil
		}
	}
	issues, err := canonical.Validate(root)
	if err != nil {
		return nil, err
	}
	if after, ok := validationKey(root, canonical.Snapshot{}); cacheable && ok && after == before {
		if remember {
			writeStoredIssues(root, storedKey(before), issues)
		}
		rememberIssues(abs, before, issues, remember)
	}
	return issues, nil
}

// validationKey identifies everything canonical validation reads: the digest
// of every canonical file and the stamp of the Git index, whose unmerged
// entries are reported too. It reports false when the tree cannot be keyed
// safely, such as a Git directory kept outside the workspace, whose index is
// not watched here; the caller then validates without the memo. A snapshot
// without files is not a scan, and the tree is scanned here.
func validationKey(root string, snapshot canonical.Snapshot) (string, bool) {
	index, ok := gitIndexStamp(root)
	if !ok {
		return "", false
	}
	if snapshot.Files == nil {
		scanned, err := canonical.ScanCached(root)
		if err != nil {
			return "", false
		}
		snapshot = scanned
	}
	hash := sha256.New()
	hash.Write([]byte(index))
	for _, file := range snapshot.Files {
		hash.Write([]byte("\n" + file.Path + "\x00" + file.Digest))
	}
	return hex.EncodeToString(hash.Sum(nil)), true
}

func gitIndexStamp(root string) (string, bool) {
	gitDir, err := os.Lstat(filepath.Join(root, ".git"))
	if errors.Is(err, fs.ErrNotExist) {
		return "no-git", true
	}
	if err != nil || !gitDir.IsDir() {
		return "", false
	}
	info, err := os.Stat(filepath.Join(root, ".git", "index"))
	if errors.Is(err, fs.ErrNotExist) {
		return "no-index", true
	}
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("index:%d:%d", info.Size(), info.ModTime().UnixNano()), true
}

func rememberIssues(abs, key string, issues []canonical.Issue, stored bool) {
	issueMemo.Lock()
	defer issueMemo.Unlock()
	if _, known := issueMemo.roots[abs]; !known && len(issueMemo.roots) >= maxIssueMemoRoots {
		clear(issueMemo.roots)
	}
	issueMemo.roots[abs] = issueMemoEntry{key: key, issues: issues, stored: stored}
}

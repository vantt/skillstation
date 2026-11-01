package canonical

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vantt/skillstation/internal/workspace"
)

func newCachedScanWorkspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}
	write(t, root, "skills/collection/alpha/skill.meta.yaml", "schema_version: 1\nid: alpha\nname: Alpha\n")
	write(t, root, "skills/collection/alpha/SKILL.md", "# Alpha\n\nVersion A.\n")
	write(t, root, "skills/collection/beta/SKILL.md", "# Beta\n")
	write(t, root, "sources/catalog/source.yaml", "id: SRC-1\n")
	ageTree(t, root, time.Minute)
	return root
}

// ageTree moves every canonical file's modification time into the past, so the
// cache may trust its stamps as for a hub that was not edited recently.
func ageTree(t *testing.T, root string, age time.Duration) {
	t.Helper()
	files, err := workspace.RelativeFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-age).Truncate(time.Second)
	for _, relative := range files {
		if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(relative)), past, past); err != nil {
			t.Fatal(err)
		}
	}
}

func requireCachedScanMatches(t *testing.T, cache *snapshotCache, root string) Snapshot {
	t.Helper()
	cached, err := cache.scan(root, nil)
	if err != nil {
		t.Fatalf("cached scan: %v", err)
	}
	full, err := Scan(root)
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}
	if !reflect.DeepEqual(cached, full) {
		t.Fatalf("cached scan differs from full scan:\ncached %+v\nfull   %+v", cached, full)
	}
	return cached
}

func TestScanCachedDetectsEveryKindOfChange(t *testing.T) {
	changes := []struct {
		name   string
		mutate func(t *testing.T, root string)
	}{
		{"edit keeping size and restoring the modification time", func(t *testing.T, root string) {
			path := filepath.Join(root, "skills/collection/alpha/SKILL.md")
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("# Alpha\n\nVersion B.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
		}},
		{"edit keeping size within the same second", func(t *testing.T, root string) {
			write(t, root, "skills/collection/alpha/SKILL.md", "# Alpha\n\nVersion C.\n")
		}},
		{"replace through rename keeping size and modification time", func(t *testing.T, root string) {
			path := filepath.Join(root, "skills/collection/alpha/SKILL.md")
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			temporary := filepath.Join(root, "skills/collection/alpha/SKILL.md.tmp")
			if err := os.WriteFile(temporary, []byte("# Alpha\n\nVersion D.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(temporary, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(temporary, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"add a file", func(t *testing.T, root string) {
			write(t, root, "skills/collection/gamma/SKILL.md", "# Gamma\n")
		}},
		{"delete a file", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "skills/collection/beta/SKILL.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename a file", func(t *testing.T, root string) {
			if err := os.Rename(filepath.Join(root, "skills/collection/beta/SKILL.md"), filepath.Join(root, "skills/collection/beta/README.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"rename a directory", func(t *testing.T, root string) {
			if err := os.Rename(filepath.Join(root, "skills/collection/beta"), filepath.Join(root, "skills/collection/delta")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, change := range changes {
		t.Run(change.name, func(t *testing.T) {
			root := newCachedScanWorkspace(t)
			cache := &snapshotCache{now: time.Now}
			before := requireCachedScanMatches(t, cache, root)
			// The second scan is served entirely from trusted cached digests.
			if again := requireCachedScanMatches(t, cache, root); again.ProjectionInputDigest != before.ProjectionInputDigest {
				t.Fatal("unchanged tree produced a different snapshot")
			}
			change.mutate(t, root)
			after := requireCachedScanMatches(t, cache, root)
			if after.ProjectionInputDigest == before.ProjectionInputDigest {
				t.Fatal("cached scan did not observe the change")
			}
			if after.CatalogSnapshot == before.CatalogSnapshot {
				t.Fatal("catalog snapshot did not observe a change under skills/")
			}
		})
	}
}

func TestScanCachedRevertedEditMatchesOriginalSnapshot(t *testing.T) {
	root := newCachedScanWorkspace(t)
	cache := &snapshotCache{now: time.Now}
	original := requireCachedScanMatches(t, cache, root)
	write(t, root, "skills/collection/alpha/SKILL.md", "# Alpha\n\nVersion B, longer.\n")
	if edited := requireCachedScanMatches(t, cache, root); edited.ProjectionInputDigest == original.ProjectionInputDigest {
		t.Fatal("edit was not observed")
	}
	write(t, root, "skills/collection/alpha/SKILL.md", "# Alpha\n\nVersion A.\n")
	if reverted := requireCachedScanMatches(t, cache, root); reverted.ProjectionInputDigest != original.ProjectionInputDigest {
		t.Fatal("reverted edit did not restore the original snapshot")
	}
}

// TestScanCachedTrustsOnlyStampsOutsideTheRacyWindow plants a wrong digest under
// the file's current stamp. Only an entry hashed clearly after the file's last
// modification may be reused; a recent modification forces a fresh read.
func TestScanCachedTrustsOnlyStampsOutsideTheRacyWindow(t *testing.T) {
	root := newCachedScanWorkspace(t)
	const relative = "skills/collection/alpha/SKILL.md"
	plant := func(t *testing.T, cache *snapshotCache, hashedAfterModification time.Duration) {
		t.Helper()
		key, err := filepath.Abs(root)
		if err != nil {
			t.Fatal(err)
		}
		tree := cache.load(key)
		entries := make(map[string]stampedDigest, len(tree.entries))
		for path, entry := range tree.entries {
			entries[path] = entry
		}
		entry := entries[relative]
		entry.digest = FileDigest{Path: relative, Digest: "sha256:planted"}
		entry.hashedAt = entry.stamp.modTime + int64(hashedAfterModification)
		entries[relative] = entry
		cache.store(key, &cachedTree{entries: entries, snapshot: tree.snapshot})
	}
	digestOf := func(snapshot Snapshot) string {
		for _, file := range snapshot.Files {
			if file.Path == relative {
				return file.Digest
			}
		}
		t.Fatalf("%s missing from snapshot", relative)
		return ""
	}

	t.Run("recent modification is read again", func(t *testing.T) {
		cache := &snapshotCache{now: time.Now}
		requireCachedScanMatches(t, cache, root)
		plant(t, cache, racyWindow-time.Millisecond)
		requireCachedScanMatches(t, cache, root)
	})
	t.Run("modification at the window boundary is read again", func(t *testing.T) {
		cache := &snapshotCache{now: time.Now}
		requireCachedScanMatches(t, cache, root)
		plant(t, cache, racyWindow)
		requireCachedScanMatches(t, cache, root)
	})
	t.Run("old modification reuses the cached digest without reading", func(t *testing.T) {
		cache := &snapshotCache{now: time.Now}
		requireCachedScanMatches(t, cache, root)
		plant(t, cache, racyWindow+time.Millisecond)
		snapshot, err := cache.scan(root, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := digestOf(snapshot); got != "sha256:planted" {
			t.Fatalf("trusted entry was not reused: digest %s", got)
		}
	})
}

func TestScanCachedRehashesFilesModifiedJustBeforeTheScan(t *testing.T) {
	root := newCachedScanWorkspace(t)
	write(t, root, "skills/collection/alpha/SKILL.md", "# Alpha\n\nVersion R.\n")
	cache := &snapshotCache{now: time.Now}
	requireCachedScanMatches(t, cache, root)
	key, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := cache.load(key).entries["skills/collection/alpha/SKILL.md"]
	if entry.trusted() {
		t.Fatal("a file written just before the scan must not be trusted on its stamp alone")
	}
	if !cache.load(key).entries["skills/collection/beta/SKILL.md"].trusted() {
		t.Fatal("an old file should be trusted on its stamp")
	}
}

func TestScanCachedRejectsObservedConcurrentModification(t *testing.T) {
	root := newCachedScanWorkspace(t)
	cache := &snapshotCache{now: time.Now}
	_, err := cache.scan(root, func() {
		write(t, root, "sources/catalog/source.yaml", "id: SRC-1\nname: changed\n")
	})
	if err == nil || !strings.Contains(err.Error(), "changed during scan") {
		t.Fatalf("cached scan accepted observed concurrent modification: %v", err)
	}
	requireCachedScanMatches(t, cache, root)
}

func TestScanCachedRejectsSymlinkLikeScan(t *testing.T) {
	root := newCachedScanWorkspace(t)
	cache := &snapshotCache{now: time.Now}
	requireCachedScanMatches(t, cache, root)
	if err := os.Symlink(filepath.Join(root, "sources/catalog/source.yaml"), filepath.Join(root, "skills/collection/beta/linked.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Scan(root); err == nil {
		t.Fatal("full scan accepted a symlink")
	}
	if _, err := cache.scan(root, nil); err == nil {
		t.Fatal("cached scan accepted a symlink")
	}
}

func TestScanCachedIsSafeForConcurrentCallers(t *testing.T) {
	root := newCachedScanWorkspace(t)
	want, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	cache := &snapshotCache{now: time.Now}
	var group sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		group.Go(func() {
			for range 20 {
				got, err := cache.scan(root, nil)
				if err != nil {
					errs <- err
					return
				}
				if got.ProjectionInputDigest != want.ProjectionInputDigest || got.CatalogSnapshot != want.CatalogSnapshot {
					errs <- os.ErrInvalid
					return
				}
				got.Files[0].Digest = "caller-owned" // must not leak into the cache
			}
		})
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	requireCachedScanMatches(t, cache, root)
}

// TestScanCachedStampFileCarriesTrustedDigestsToTheNextProcess drives two caches
// over one hub as two processes would: the first records its trusted digests in
// the stamp file, the second starts from that file. The file only saves reads;
// whatever it holds, the second scan returns what a full scan returns, except for
// the planted digest of an entry it may trust, which proves the file was not read.
func TestScanCachedStampFileCarriesTrustedDigestsToTheNextProcess(t *testing.T) {
	const relative = "skills/collection/alpha/SKILL.md"
	planted := "sha256:" + strings.Repeat("0", 64)
	newHub := func(t *testing.T) string {
		t.Helper()
		root := newCachedScanWorkspace(t)
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(path.Dir(stampFilePath))), 0o700); err != nil {
			t.Fatal(err)
		}
		requireCachedScanMatches(t, &snapshotCache{now: time.Now, persist: true}, root)
		return root
	}
	stampPath := func(root string) string { return filepath.Join(root, filepath.FromSlash(stampFilePath)) }
	// editStampFile rewrites the stamp file, passing edit the entry of relative.
	editStampFile := func(t *testing.T, root string, edit func(stored *stampFile, entry *stampFileEntry)) {
		t.Helper()
		contents, err := os.ReadFile(stampPath(root))
		if err != nil {
			t.Fatalf("the first scan left no stamp file: %v", err)
		}
		var stored stampFile
		if err := json.Unmarshal(contents, &stored); err != nil {
			t.Fatal(err)
		}
		index := slices.IndexFunc(stored.Entries, func(entry stampFileEntry) bool { return entry.Path == relative })
		if index < 0 {
			t.Fatalf("the stamp file holds no trusted entry for %s", relative)
		}
		edit(&stored, &stored.Entries[index])
		if contents, err = json.Marshal(stored); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stampPath(root), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	nextProcess := func() *snapshotCache { return &snapshotCache{now: time.Now, persist: true} }

	t.Run("an unchanged file is not read again", func(t *testing.T) {
		root := newHub(t)
		editStampFile(t, root, func(_ *stampFile, entry *stampFileEntry) { entry.Digest = planted })
		snapshot, err := nextProcess().scan(root, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range snapshot.Files {
			if file.Path == relative && file.Digest != planted {
				t.Fatalf("the trusted digest of the stamp file was not reused: %s", file.Digest)
			}
		}
	})
	t.Run("a same-size edit within the racy window is read again", func(t *testing.T) {
		root := newHub(t)
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# Alpha\n\nVersion B.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
		requireCachedScanMatches(t, nextProcess(), root)
		// A file system whose clock did not move for the edit leaves the stamp
		// as recorded. The digest was taken within the racy window, so the
		// stamp alone must not vouch for it.
		paths, stamps, err := stampTree(root)
		if err != nil {
			t.Fatal(err)
		}
		stamp := stamps[slices.Index(paths, relative)]
		editStampFile(t, root, func(_ *stampFile, entry *stampFileEntry) {
			entry.Size, entry.Mode, entry.ModTime, entry.ChangeTime = stamp.size, uint32(stamp.mode), stamp.modTime, stamp.changeTime
			entry.Device, entry.Inode = stamp.device, stamp.inode
			entry.Digest = planted
			entry.HashedAt = stamp.modTime + int64(racyWindow) - 1
		})
		requireCachedScanMatches(t, nextProcess(), root)
	})
	t.Run("a corrupt or foreign stamp file is ignored", func(t *testing.T) {
		edits := map[string]func(t *testing.T, root string){
			"not JSON": func(t *testing.T, root string) {
				if err := os.WriteFile(stampPath(root), []byte("{not json"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			"another version": func(t *testing.T, root string) {
				editStampFile(t, root, func(stored *stampFile, entry *stampFileEntry) {
					stored.Version++
					entry.Digest = planted
				})
			},
			"a malformed digest": func(t *testing.T, root string) {
				editStampFile(t, root, func(_ *stampFile, entry *stampFileEntry) { entry.Digest = "sha256:short" })
			},
		}
		for name, edit := range edits {
			t.Run(name, func(t *testing.T) {
				root := newHub(t)
				edit(t, root)
				requireCachedScanMatches(t, nextProcess(), root)
			})
		}
	})
}

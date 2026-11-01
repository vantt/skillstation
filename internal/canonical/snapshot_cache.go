package canonical

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vantt/skillstation/internal/digest"
	"github.com/vantt/skillstation/internal/fsutil"
	"github.com/vantt/skillstation/internal/workspace"
)

// racyWindow is how long before its hash a file must have last been modified for
// its stat stamp alone to prove the contents are unchanged. A write that lands in
// the same timestamp tick as the hashed version, on a filesystem with a coarse
// clock (one or two seconds on some), or with a modest clock skew between a
// network file server and this host, keeps the modification time unchanged. Such
// a "racy" file is rehashed on every scan until its modification time falls
// outside the window, the same rule Git applies to racily clean index entries.
const racyWindow = 5 * time.Second

// maxCachedRoots bounds the process-wide cache; a server normally scans one workspace.
const maxCachedRoots = 16

// fileStamp is the cheap identity of one canonical file. Any write, replacement,
// rename onto the path, or metadata restore changes at least one field, except a
// write that leaves the modification time unchanged, which the racy window covers.
type fileStamp struct {
	size       int64
	mode       fs.FileMode
	modTime    int64  // nanoseconds since the Unix epoch
	changeTime int64  // status change time in nanoseconds; 0 where the platform has none
	device     uint64 // 0 where the platform has none
	inode      uint64 // 0 where the platform has none
}

// stampedDigest records the content digest observed for a stamp and the wall
// clock reading taken before that stamp and those contents were read.
type stampedDigest struct {
	stamp    fileStamp
	digest   FileDigest
	hashedAt int64
}

// trusted reports whether an unchanged stamp proves the digest is still current:
// the file was last modified clearly before its contents were hashed, so any later
// write must have moved its modification or change time.
func (entry stampedDigest) trusted() bool {
	return entry.stamp.modTime < entry.hashedAt-int64(racyWindow)
}

// cachedTree is one workspace's digests and the snapshot they produced. It is
// replaced as a whole and never mutated, so readers need no lock once loaded.
type cachedTree struct {
	entries  map[string]stampedDigest
	snapshot Snapshot
	// persisted is the number of entries the stamp file holds for this tree as
	// far as this process knows, so an unchanged tree is not written again.
	persisted int
}

// reusable returns the cached digest for a path when its stamp proves it current.
func (tree *cachedTree) reusable(path string, stamp fileStamp) (stampedDigest, bool) {
	if tree == nil {
		return stampedDigest{}, false
	}
	entry, ok := tree.entries[path]
	if !ok || entry.stamp != stamp || !entry.trusted() {
		return stampedDigest{}, false
	}
	return entry, true
}

// snapshotCache remembers content digests per workspace so repeated scans read
// only files whose stamp changed or is too recent to trust.
type snapshotCache struct {
	mu    sync.Mutex
	roots map[string]*cachedTree
	now   func() time.Time
	// persist starts from the trusted digests in the hub's stamp file, which
	// a recording scan keeps, instead of reading every file again.
	persist bool
}

var processSnapshotCache = &snapshotCache{now: time.Now, persist: true}

// ScanCached returns exactly the Snapshot that Scan returns for the same tree. It
// lists the canonical files on every call, then reuses a digest computed by an
// earlier call, in this process or recorded in the hub's stamp file, when the
// file's size, mode, modification time, change time and file identity are all
// unchanged and the file was not modified within racyWindow of being hashed.
// Every other file is read and hashed twice
// around a second listing, as Scan does. With no recent changes the cost depends
// on the number of files, not on their contents.
//
// A snapshot built only from reused digests describes the tree at the start of
// the call: each file kept its hashed contents from its last modification, before
// that instant, until it was stamped, after it.
//
// It also starts from the trusted digests that ScanCachedAndRecord kept in the
// hub's stamp file (stampFilePath), under the same rules, and never writes that
// file, so a read-only command or a dry run leaves the hub untouched. The file
// is only an optimisation: a missing, unreadable, corrupt or older-format file
// is ignored and every file is read.
func ScanCached(root string) (Snapshot, error) {
	return processSnapshotCache.scan(root, nil, false)
}

// ScanCachedAndRecord is ScanCached that also keeps the trusted digests in the
// hub's stamp file for the next process. Only the catalog build, which writes
// the catalog's runtime folder anyway, uses it. A failure to write the file is
// not an error.
func ScanCachedAndRecord(root string) (Snapshot, error) {
	return processSnapshotCache.scan(root, nil, true)
}

func (cache *snapshotCache) scan(root string, beforeRecheck func(), record bool) (Snapshot, error) {
	key, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve canonical workspace: %w", err)
	}
	startedAt := cache.now().UnixNano()
	paths, stamps, err := stampTree(root)
	if err != nil {
		return Snapshot{}, err
	}
	previous := cache.load(key)
	if previous == nil && cache.persist {
		previous = readStampFile(root)
	}
	files := make([]FileDigest, len(paths))
	var pending []int
	for index, path := range paths {
		if entry, ok := previous.reusable(path, stamps[index]); ok {
			files[index] = entry.digest
			continue
		}
		pending = append(pending, index)
	}
	// With nothing pending every listed path is a cached one, so equal counts mean
	// the path set is unchanged too and the cached snapshot is current.
	if previous != nil && len(pending) == 0 && len(previous.entries) == len(paths) {
		fromFile := previous.snapshot.CatalogSnapshot == ""
		snapshot := previous.snapshot
		if fromFile {
			// Read from the stamp file: derive the aggregates once and keep them.
			snapshot = NewSnapshot(files)
		} else {
			snapshot.Files = files
		}
		persisted := previous.persisted
		if cache.persist && record {
			// An earlier scan that did not record may have left the file behind.
			persisted = persistTree(root, previous, false)
		}
		if fromFile || persisted != previous.persisted {
			cached := snapshot
			cached.Files = nil
			cache.store(key, &cachedTree{entries: previous.entries, snapshot: cached, persisted: persisted})
		}
		return snapshot, nil
	}
	// A file was added, removed, renamed or must be read: observe the tree twice,
	// like Scan does, before trusting what was read.
	first, err := hashPending(root, paths, pending)
	if err != nil {
		return Snapshot{}, err
	}
	if beforeRecheck != nil {
		beforeRecheck()
	}
	recheckPaths, recheckStamps, err := stampTree(root)
	if err != nil {
		return Snapshot{}, err
	}
	if !sameStampedTree(paths, stamps, recheckPaths, recheckStamps) {
		return Snapshot{}, errChangedDuringScan
	}
	second, err := hashPending(root, paths, pending)
	if err != nil {
		return Snapshot{}, err
	}
	entries := make(map[string]stampedDigest, len(paths))
	for index, path := range paths {
		if entry, ok := previous.reusable(path, stamps[index]); ok {
			entries[path] = entry
		}
	}
	for position, index := range pending {
		if first[position] != second[position] {
			return Snapshot{}, errChangedDuringScan
		}
		files[index] = first[position]
		entries[paths[index]] = stampedDigest{stamp: stamps[index], digest: first[position], hashedAt: startedAt}
	}
	snapshot := NewSnapshot(files)
	cached := snapshot
	cached.Files = nil // callers own the returned slice; the cache keeps only the digests
	// The tree changed since the stamp file was read or written, so a scan that
	// does not record leaves its state unknown and the next recording scan
	// writes it.
	tree := &cachedTree{entries: entries, snapshot: cached, persisted: -1}
	if cache.persist && record {
		if previous != nil {
			tree.persisted = previous.persisted
		}
		tree.persisted = persistTree(root, tree, newlyTrusted(entries, paths, pending))
	}
	cache.store(key, tree)
	return snapshot, nil
}

// newlyTrusted reports whether a file read in this scan may be reused on its
// stamp by a later one. A file read within racyWindow of its last change is not
// worth keeping: no later scan may trust that digest.
func newlyTrusted(entries map[string]stampedDigest, paths []string, pending []int) bool {
	for _, index := range pending {
		if entries[paths[index]].trusted() {
			return true
		}
	}
	return false
}

func (cache *snapshotCache) load(key string) *cachedTree {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.roots[key]
}

func (cache *snapshotCache) store(key string, tree *cachedTree) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.roots == nil {
		cache.roots = make(map[string]*cachedTree)
	}
	if _, exists := cache.roots[key]; !exists && len(cache.roots) >= maxCachedRoots {
		clear(cache.roots)
	}
	cache.roots[key] = tree
}

// stampTree lists canonical files with the same rules and limits as Scan and
// stamps each one without reading its contents. The stamps come from the
// lstat taken while each directory was listed, one stat per file.
func stampTree(root string) ([]string, []fileStamp, error) {
	paths, infos, err := workspace.StatFiles(root)
	if err != nil {
		return nil, nil, err
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, fmt.Errorf("open canonical workspace: %w", err)
	}
	defer rootHandle.Close()
	stamps := make([]fileStamp, len(paths))
	for index, relative := range paths {
		info := infos[index]
		stamp := fileStamp{size: info.Size(), mode: info.Mode(), modTime: info.ModTime().UnixNano()}
		if err := addPlatformStamp(&stamp, rootHandle, relative, info); err != nil {
			return nil, nil, fmt.Errorf("inspect canonical input %s: %w", relative, err)
		}
		stamps[index] = stamp
	}
	return paths, stamps, nil
}

func hashPending(root string, paths []string, pending []int) ([]FileDigest, error) {
	if len(pending) == 0 {
		return nil, nil
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open canonical workspace: %w", err)
	}
	defer rootHandle.Close()
	digests := make([]FileDigest, len(pending))
	for position, index := range pending {
		contents, err := workspace.ReadCanonicalFileIn(rootHandle, paths[index])
		if err != nil {
			return nil, err
		}
		digests[position] = NewFileDigest(paths[index], contents)
	}
	return digests, nil
}

func sameStampedTree(paths []string, stamps []fileStamp, otherPaths []string, otherStamps []fileStamp) bool {
	if len(paths) != len(otherPaths) {
		return false
	}
	for index := range paths {
		if paths[index] != otherPaths[index] || stamps[index] != otherStamps[index] {
			return false
		}
	}
	return true
}

// stampFilePath is where a hub keeps the trusted digests between processes:
// beside the catalog generations, below the runtime folder that Git ignores and
// that is rebuilt from the canonical files. The file can only make a catalog
// count as current, so it sits where only whoever can replace the catalog
// itself may write, not in the cache folder agents are allowed to write.
const stampFilePath = "runtime/catalog/canonical-stamps.json"

// stampFileVersion changes whenever the meaning of a field changes; a file of
// another version is ignored and rewritten.
const stampFileVersion = 1

// maxStampFileBytes bounds what is read from the stamp file; a larger file is
// ignored. A hub at the aggregate size limit fits several times over.
const maxStampFileBytes = 64 << 20

type stampFile struct {
	Version int              `json:"version"`
	Entries []stampFileEntry `json:"entries"`
}

type stampFileEntry struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Mode       uint32 `json:"mode"`
	ModTime    int64  `json:"mtime_ns"`
	ChangeTime int64  `json:"ctime_ns,omitempty"`
	Device     uint64 `json:"dev,omitempty"`
	Inode      uint64 `json:"ino,omitempty"`
	Digest     string `json:"digest"`
	Catalog    string `json:"catalog_digest,omitempty"`
	HashedAt   int64  `json:"hashed_at_ns"`
}

// readStampFile loads the trusted digests another process recorded for the hub
// at root, or nil when there are none it can use. Nothing found in the file is
// trusted beyond what ScanCached checks for an entry kept in memory.
func readStampFile(root string) *cachedTree {
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil
	}
	defer rootHandle.Close()
	if !realDirectories(rootHandle, path.Dir(stampFilePath)) {
		return nil
	}
	contents, err := readRegularFile(rootHandle, stampFilePath, maxStampFileBytes)
	if err != nil {
		return nil
	}
	var stored stampFile
	if err := json.Unmarshal(contents, &stored); err != nil || stored.Version != stampFileVersion {
		return nil
	}
	entries := make(map[string]stampedDigest, len(stored.Entries))
	for _, item := range stored.Entries {
		_, duplicate := entries[item.Path]
		if duplicate || !IsCanonicalPath(item.Path) || !digest.Valid(item.Digest) || (item.Catalog != "" && !digest.Valid(item.Catalog)) {
			return nil
		}
		entries[item.Path] = stampedDigest{
			stamp:    fileStamp{size: item.Size, mode: fs.FileMode(item.Mode), modTime: item.ModTime, changeTime: item.ChangeTime, device: item.Device, inode: item.Inode},
			digest:   FileDigest{Path: item.Path, Digest: item.Digest, catalog: item.Catalog},
			hashedAt: item.HashedAt,
		}
	}
	return &cachedTree{entries: entries, persisted: len(entries)}
}

// persistTree writes the trusted entries of tree to the stamp file when they
// differ from what the file holds, and returns the number the file now holds as
// far as this process knows. A failed write is not retried until the tree
// changes again: the file is only an optimisation.
func persistTree(root string, tree *cachedTree, newlyTrusted bool) int {
	count := 0
	for _, entry := range tree.entries {
		if entry.trusted() {
			count++
		}
	}
	// Trusted entries are only ever added by a read in this scan, so an equal
	// count without one means the file already holds exactly these entries.
	unchanged := tree.persisted == count || (tree.persisted < 0 && count == 0)
	if !newlyTrusted && unchanged {
		return tree.persisted
	}
	stored := stampFile{Version: stampFileVersion, Entries: make([]stampFileEntry, 0, count)}
	for _, entry := range tree.entries {
		if !entry.trusted() {
			continue
		}
		stored.Entries = append(stored.Entries, stampFileEntry{
			Path: entry.digest.Path, Size: entry.stamp.size, Mode: uint32(entry.stamp.mode),
			ModTime: entry.stamp.modTime, ChangeTime: entry.stamp.changeTime, Device: entry.stamp.device, Inode: entry.stamp.inode,
			Digest: entry.digest.Digest, Catalog: entry.digest.catalog, HashedAt: entry.hashedAt,
		})
	}
	sort.Slice(stored.Entries, func(i, j int) bool { return stored.Entries[i].Path < stored.Entries[j].Path })
	contents, err := json.Marshal(stored)
	if err != nil {
		return count
	}
	if !stampDirectoryExists(root) {
		return count
	}
	_ = fsutil.WriteFileAtomic(filepath.Join(root, filepath.FromSlash(stampFilePath)), contents, 0o600)
	return count
}

// stampDirectoryExists reports whether the folder of the stamp file is a real
// folder below root. It is never created here: a folder without a catalog gets
// no stamp file.
func stampDirectoryExists(root string) bool {
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return false
	}
	defer rootHandle.Close()
	return realDirectories(rootHandle, path.Dir(stampFilePath))
}

// realDirectories reports whether every component of the slash path relative is
// a directory below the root and none is a symbolic link.
func realDirectories(rootHandle *os.Root, relative string) bool {
	current := ""
	for _, component := range strings.Split(relative, "/") {
		current = path.Join(current, component)
		info, err := rootHandle.Lstat(current)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return false
		}
	}
	return true
}

// readRegularFile reads a regular file below the root of at most limit bytes.
func readRegularFile(rootHandle *os.Root, relative string, limit int64) ([]byte, error) {
	info, err := rootHandle.Lstat(relative)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("%s is not a regular file within the size limit", relative)
	}
	file, err := rootHandle.Open(relative)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("%s grew beyond the size limit", relative)
	}
	return contents, nil
}

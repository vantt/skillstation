package skillruntime

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// placeAttempts bounds the retries of the final rename and of the check of a
// copy another process has just placed.
const placeAttempts = 5

// PlaceTree moves the complete staging directory to target, which content
// addressing makes the only valid place for it. valid reports whether target
// already holds a complete copy; ensureParent recreates the parent of target
// when it vanished. It reports whether staging was placed. On every other
// outcome staging is removed.
//
// The rename fails when a non-empty target exists (POSIX may replace an empty
// one, which is never a valid copy), and the existing copy is checked instead,
// with a short retry: a copy that another process has just placed or is
// reading can fail to open for a moment (a sharing violation on Windows), and
// treating that as damage would delete a valid copy. An invalid leftover, for
// example one edited by hand, is moved into trashDir and removed. A failure
// with no target is retried briefly: another process may have removed the
// parent meanwhile, or, on Windows, a virus scanner may still hold a new file
// open.
func PlaceTree(staging, target, trashDir string, ensureParent func() error, valid func() bool) (bool, error) {
	var renameErr error
	for attempt := range placeAttempts {
		renameErr = os.Rename(staging, target)
		if renameErr == nil {
			return true, nil
		}
		if _, statErr := os.Lstat(target); errors.Is(statErr, fs.ErrNotExist) {
			if err := ensureParent(); err != nil {
				RemoveTree(staging)
				return false, err
			}
			time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
			continue
		}
		if Settled(valid) {
			// Another process placed the same content first.
			RemoveTree(staging)
			return false, nil
		}
		trash := filepath.Join(trashDir, "trash-"+RandomSuffix())
		if err := os.Rename(target, trash); err != nil && !errors.Is(err, fs.ErrNotExist) {
			RemoveTree(staging)
			return false, fmt.Errorf("move an invalid copy of %s aside: %w", target, err)
		}
		RemoveTree(trash)
	}
	RemoveTree(staging)
	return false, fmt.Errorf("place %s: concurrent replacement did not settle: %w", target, renameErr)
}

// Settled is check with a short retry, for a tree another process may have
// just placed or may be reading.
func Settled(check func() bool) bool {
	for attempt := 0; ; attempt++ {
		if check() {
			return true
		}
		if attempt == placeAttempts {
			return false
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
}

// RemoveTree deletes a cached, staged or state tree. Entries are made writable
// first, because cached files are read-only and Windows refuses to delete
// read-only files, and some trees (module caches) hold read-only directories.
// Symlinks are removed, never followed.
func RemoveTree(directory string) {
	_ = filepath.WalkDir(directory, func(current string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if entry.Type().IsRegular() {
			_ = os.Chmod(current, 0o600)
		} else if entry.IsDir() {
			_ = os.Chmod(current, 0o700)
		}
		return nil
	})
	_ = os.RemoveAll(directory)
}

// RandomSuffix returns a random hex name component for staging and trash
// entries.
func RandomSuffix() string {
	var value [12]byte
	// crypto/rand.Read never returns an error on supported platforms.
	_, _ = rand.Read(value[:])
	return hex.EncodeToString(value[:])
}

package app

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// runtimeTempMaxAge is the age past which an entry of runtime/tmp is taken as
// left behind by a process that ended without cleaning up, such as a crash.
// Every user of runtime/tmp removes its folder before its request returns.
const runtimeTempMaxAge = time.Hour

// newRuntimeTemp makes a private folder named prefix plus a random suffix in
// the hub's runtime/tmp and returns it with a function that removes it. It
// refuses a runtime/tmp that is a symlink or not a folder, and removes the
// entries older than runtimeTempMaxAge first.
func newRuntimeTemp(root, prefix string) (string, func(), error) {
	parent := filepath.Join(root, "runtime", "tmp")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", nil, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return "", nil, err
	}
	info, err := handle.Lstat(filepath.Join("runtime", "tmp"))
	_ = handle.Close()
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", nil, errors.New("unsafe runtime/tmp directory")
	}
	removeStaleRuntimeTemp(parent)
	dir, err := os.MkdirTemp(parent, prefix+"*")
	if err != nil {
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

// removeStaleRuntimeTemp removes the entries of runtime/tmp older than
// runtimeTempMaxAge. Failures are ignored: a leftover is only clutter.
func removeStaleRuntimeTemp(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-runtimeTempMaxAge)
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(parent, entry.Name()))
		}
	}
}

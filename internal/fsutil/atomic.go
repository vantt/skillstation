// Package fsutil holds small file system helpers shared by every layer.
package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// WriteFileAtomic replaces path with data so that a reader sees either the old
// or the new contents, never a part. It writes a temporary file in the same
// folder, gives it perm, syncs it, renames it over path and then syncs the
// folder so the rename survives a crash. The folder must exist. On Windows
// perm only controls the read-only flag.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) (resultErr error) {
	directory, name := filepath.Split(filepath.Clean(path))
	if directory == "" {
		directory = "."
	}
	temporary, err := os.CreateTemp(directory, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if resultErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	err = temporary.Chmod(perm)
	if err != nil && runtime.GOOS == "windows" {
		err = nil
	}
	if err == nil {
		_, err = temporary.Write(data)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := Replace(temporaryPath, path); err != nil {
		return err
	}
	return SyncDir(directory)
}

// SyncDir makes the entries of a folder durable, such as a file just renamed
// into it. Windows has no folder sync, so it does nothing there.
func SyncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && !errors.Is(err, fs.ErrInvalid) {
		return err
	}
	return nil
}

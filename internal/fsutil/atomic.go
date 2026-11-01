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
	directory, temporaryPath, err := writeTemporary(path, data, perm)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := Replace(temporaryPath, path); err != nil {
		return err
	}
	return SyncDir(directory)
}

// WriteNewFileAtomic creates path with data like WriteFileAtomic, but never
// replaces an existing entry: when path exists, even one created after the
// caller looked, it returns an error matching fs.ErrExist and leaves path as
// it was. The complete file is published under its name in one step through
// a hard link, so the file system must support hard links.
func WriteNewFileAtomic(path string, data []byte, perm fs.FileMode) error {
	directory, temporaryPath, err := writeTemporary(path, data, perm)
	if err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	return SyncDir(directory)
}

// writeTemporary writes data with perm to a synced temporary file next to path
// and returns the folder and the temporary file. It removes the temporary file
// itself when it fails.
func writeTemporary(path string, data []byte, perm fs.FileMode) (directory, temporaryPath string, err error) {
	directory, name := filepath.Split(filepath.Clean(path))
	if directory == "" {
		directory = "."
	}
	temporary, err := os.CreateTemp(directory, "."+name+".tmp-*")
	if err != nil {
		return "", "", err
	}
	temporaryPath = temporary.Name()
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
		_ = os.Remove(temporaryPath)
		return "", "", err
	}
	return directory, temporaryPath, nil
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

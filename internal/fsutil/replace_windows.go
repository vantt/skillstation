//go:build windows

package fsutil

import "golang.org/x/sys/windows"

// Replace renames source over target in one step, replacing target if it
// exists, and returns only once the move is written through to disk.
func Replace(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

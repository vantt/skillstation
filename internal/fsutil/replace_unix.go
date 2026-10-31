//go:build !windows

package fsutil

import "os"

// Replace renames source over target in one step, replacing target if it exists.
func Replace(source, target string) error { return os.Rename(source, target) }

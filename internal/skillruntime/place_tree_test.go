package skillruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, directory, body string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTree(t *testing.T, directory string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// PlaceTree keeps a valid copy another process placed first, replaces an
// invalid leftover, and recreates a parent that vanished; staging never stays
// behind.
func TestPlaceTreeOutcomes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		existing   string // body of a copy already at target, "" for none
		dropParent bool
		valid      func(target string) bool
		wantPlaced bool
		wantBody   string
	}{
		"empty target": {wantPlaced: true, wantBody: "new"},
		"valid copy placed first": {
			existing: "same", valid: func(string) bool { return true }, wantBody: "same",
		},
		"invalid leftover": {
			existing: "edited by hand", wantPlaced: true, wantBody: "new",
			valid: func(target string) bool {
				data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
				return err == nil && string(data) == "new"
			},
		},
		"parent removed meanwhile": {dropParent: true, wantPlaced: true, wantBody: "new"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			trash := filepath.Join(root, "trash")
			parent := filepath.Join(root, "skills")
			target := filepath.Join(parent, "demo")
			staging := filepath.Join(trash, "staging")
			writeTree(t, staging, "new")
			if test.existing != "" {
				writeTree(t, target, test.existing)
			} else if !test.dropParent {
				if err := os.MkdirAll(parent, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			valid := test.valid
			if valid == nil {
				valid = func(string) bool { return false }
			}
			placed, err := PlaceTree(staging, target, trash, func() error { return os.MkdirAll(parent, 0o700) }, func() bool { return valid(target) })
			if err != nil || placed != test.wantPlaced {
				t.Fatalf("placed = %v, err = %v, want placed %v", placed, err, test.wantPlaced)
			}
			if got := readTree(t, target); got != test.wantBody {
				t.Fatalf("target holds %q, want %q", got, test.wantBody)
			}
			if leftovers, _ := os.ReadDir(trash); len(leftovers) != 0 {
				t.Fatalf("staging or trash left behind: %v", leftovers)
			}
		})
	}
}

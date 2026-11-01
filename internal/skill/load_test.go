package skill

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoadRefusesWhatItWouldReadThrough pins the one lookup of a skill by id:
// every entry it would read through a symbolic link is refused, a stray file
// beside the collections is not, and an id filed twice is an error.
func TestLoadRefusesWhatItWouldReadThrough(t *testing.T) {
	t.Parallel()
	writeSkill := func(t *testing.T, root, collection, id string) {
		t.Helper()
		dir := filepath.Join(root, "skills", collection, id)
		mustWrite(t, filepath.Join(dir, ".meta", "hub.yaml"), "status: draft\n")
		mustWrite(t, filepath.Join(dir, "skill.yaml"), "name: Demo\n")
		mustWrite(t, filepath.Join(dir, "SKILL.md"), "# Demo\n")
	}
	link := func(t *testing.T, target, name string) {
		t.Helper()
		if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if err := os.Symlink(target, name); err != nil {
			t.Skipf("symbolic links are not available here: %v", err)
		}
	}
	cases := []struct {
		name    string
		arrange func(t *testing.T, root, outside string)
		wantErr error
		wantIDs []string
	}{
		{
			name: "a skill in a collection, with a stray file beside the collections",
			arrange: func(t *testing.T, root, _ string) {
				mustWrite(t, filepath.Join(root, "skills", "README.md"), "notes\n")
			},
			wantIDs: []string{"demo"},
		},
		{
			name:    "no skills folder",
			arrange: func(t *testing.T, root, _ string) { mustRemoveAll(t, filepath.Join(root, "skills")) },
			wantErr: ErrNotFound,
		},
		{
			name: "skill folder is a symbolic link",
			arrange: func(t *testing.T, root, outside string) {
				dir := filepath.Join(root, "skills", "default", "demo")
				mustRename(t, dir, filepath.Join(outside, "demo"))
				link(t, filepath.Join(outside, "demo"), dir)
			},
			wantErr: ErrInvalidSkillTree,
			wantIDs: []string{"demo"},
		},
		{
			name: "hub metadata is a symbolic link",
			arrange: func(t *testing.T, root, outside string) {
				mustWrite(t, filepath.Join(outside, "hub.yaml"), "status: active\n")
				link(t, filepath.Join(outside, "hub.yaml"), filepath.Join(root, "skills", "default", "demo", ".meta", "hub.yaml"))
			},
			wantErr: ErrInvalidSkillTree,
			wantIDs: []string{"demo"},
		},
		{
			name: "SKILL.md is a symbolic link",
			arrange: func(t *testing.T, root, outside string) {
				mustWrite(t, filepath.Join(outside, "SKILL.md"), "# Elsewhere\n")
				link(t, filepath.Join(outside, "SKILL.md"), filepath.Join(root, "skills", "default", "demo", "SKILL.md"))
			},
			wantErr: ErrInvalidSkillTree,
			wantIDs: []string{"demo"},
		},
		{
			name:    "one id in two collections",
			arrange: func(t *testing.T, root, _ string) { writeSkill(t, root, "extra", "demo") },
			wantErr: ErrInvalidSkillTree,
			wantIDs: []string{"demo"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, outside := t.TempDir(), t.TempDir()
			writeSkill(t, root, "default", "demo")
			tc.arrange(t, root, outside)

			loaded, err := Load(root, "demo")
			if err == nil {
				_, err = ReadEntrypoint(root, loaded.Dir)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Load and ReadEntrypoint error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && (loaded.Collection != "default" || loaded.Dir != "skills/default/demo" ||
				loaded.MetadataPath != "skills/default/demo/.meta/hub.yaml" || len(loaded.Metadata) == 0) {
				t.Fatalf("Load = %+v", loaded)
			}
			ids, err := IDs(root)
			if err != nil || !reflect.DeepEqual(ids, tc.wantIDs) {
				t.Fatalf("IDs = %v, %v; want %v", ids, err, tc.wantIDs)
			}
		})
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRemoveAll(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

func mustRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

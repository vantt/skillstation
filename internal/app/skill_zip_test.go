package app

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	sourcepkg "github.com/vantt/skillstation/internal/source"
)

type zipFile struct {
	name    string
	content string
	mode    os.FileMode
	flags   uint16
}

func buildZip(t *testing.T, files ...zipFile) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		if file.mode != 0 {
			header.SetMode(file.mode)
		}
		header.Flags |= file.flags
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestExtractSkillZipRefusesUnsafeEntries(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("a", int(sourcepkg.DefaultMaxFileSize)+1)
	chunk := strings.Repeat("b", int(sourcepkg.DefaultMaxFileSize))
	cases := []struct {
		name  string
		files []zipFile
		why   string
	}{
		{"backslash path", []zipFile{{name: `dir\SKILL.md`}}, "backslash"},
		{"drive letter", []zipFile{{name: "C:/SKILL.md"}}, "absolute path"},
		{"parent segment", []zipFile{{name: "a/../../SKILL.md"}}, "leaves the archive folder"},
		{"symbolic link", []zipFile{{name: "link", content: "/etc/passwd", mode: os.ModeSymlink | 0o777}}, "symbolic link"},
		{"encrypted entry", []zipFile{{name: "SKILL.md", flags: 0x1}}, "encrypted"},
		{"same file in two cases", []zipFile{{name: "SKILL.md"}, {name: "skill.md"}}, "same file"},
		{"file over the size limit", []zipFile{{name: "big.md", content: big}}, "larger than"},
		{"total over the size limit", []zipFile{
			{name: "a.md", content: chunk}, {name: "b.md", content: chunk}, {name: "c.md", content: chunk},
			{name: "d.md", content: chunk}, {name: "e.md", content: "x"},
		}, "more than 8 MiB"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := extractSkillZip(buildZip(t, test.files...), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), test.why) {
				t.Fatalf("error = %v, want one mentioning %q", err, test.why)
			}
		})
	}
}

// Files an import never copies are skipped, not refused, and an executable bit survives.
func TestExtractSkillZipSkipsImportExclusionsAndKeepsExecutableBit(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	archive := buildZip(t,
		zipFile{name: "skill/SKILL.md", content: "# skill\n"},
		zipFile{name: "skill/scripts/run.sh", content: "#!/bin/sh\n", mode: 0o755},
		zipFile{name: "skill/.env", content: "SECRET=1\n"},
		zipFile{name: "skill/node_modules/x/index.js", content: "x"},
		zipFile{name: "__MACOSX/skill/._SKILL.md", content: "junk"},
	)
	if err := extractSkillZip(archive, dest); err != nil {
		t.Fatal(err)
	}
	for _, skipped := range []string{"skill/.env", "skill/node_modules", "__MACOSX"} {
		if _, err := os.Stat(filepath.Join(dest, skipped)); err == nil {
			t.Fatalf("%s was extracted", skipped)
		}
	}
	info, err := os.Stat(filepath.Join(dest, "skill", "scripts", "run.sh"))
	if err != nil {
		t.Fatalf("run.sh was not extracted: %v", err)
	}
	// Windows has no executable bit to keep.
	if runtime.GOOS != "windows" && info.Mode()&0o100 == 0 {
		t.Fatalf("run.sh mode = %v; want it executable", info.Mode())
	}
}

// A temporary folder left in runtime/tmp by a process that never cleaned up is
// removed by the next request, and a symlinked runtime/tmp is refused.
func TestNewRuntimeTempRemovesLeftoversAndRefusesASymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := filepath.Join(root, "runtime", "tmp")
	stale, fresh := filepath.Join(parent, "skill-zip-stale"), filepath.Join(parent, "upstream-merge-fresh")
	for _, dir := range []string{stale, fresh} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * runtimeTempMaxAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	dir, cleanup, err := newRuntimeTemp(root, "skill-zip-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("a leftover older than %s stayed: %v", runtimeTempMaxAge, err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("a folder still in use was removed: %v", err)
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("cleanup left %s: %v", dir, err)
	}

	linked := t.TempDir()
	if err := os.MkdirAll(filepath.Join(linked, "runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(linked, "runtime", "tmp")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	if _, _, err := newRuntimeTemp(linked, "skill-zip-"); err == nil {
		t.Fatal("a symlinked runtime/tmp was accepted")
	}
}

package app

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if err != nil || info.Mode()&0o100 == 0 {
		t.Fatalf("run.sh = %v, %v; want it extracted and executable", info, err)
	}
}

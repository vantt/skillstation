package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/workspace"
)

func TestCatalogInputRejectsOversizedCanonicalFile(t *testing.T) {
	t.Parallel()
	root := newWorkspace(t)
	path := filepath.Join(root, "skills", "oversized", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(workspace.MaxCanonicalFileBytesV1 + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readInput(root); err == nil || !strings.Contains(err.Error(), "skills/oversized/SKILL.md exceeds V1 per-file limit") {
		t.Fatalf("catalog input error = %v", err)
	}
}

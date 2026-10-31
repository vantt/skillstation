// Package archtest holds the import rules between the layers of the module.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/vantt/skillstation/"

// A package may import only packages below it. A rule lists the import prefixes
// the packages under a directory must not use; the exceptions are existing
// imports that may be removed but never added.
type rule struct {
	from       string
	forbidden  []string
	exceptions map[string]string
	why        string
}

var rules = []rule{
	{
		from:      "internal/delivery",
		forbidden: []string{"internal/canonical", "internal/catalog", "internal/mutation"},
		why:       "delivery surfaces reach the hub through internal/app, the single door",
		exceptions: map[string]string{
			"internal/delivery/cli -> internal/mutation":      "the maintain commands run recovery directly",
			"internal/delivery/mcpserver -> internal/catalog": "the snapshot and bundle endpoints read the catalog",
		},
	},
	{
		from:      "internal/canonical",
		forbidden: []string{"internal/app", "internal/delivery"},
		why:       "the canonical store knows nothing about the services above it",
	},
	{
		from:      "internal/mutation",
		forbidden: []string{"internal/app", "internal/delivery", "internal/catalog"},
		why:       "mutation writes the canonical repository and is below the catalog",
	},
	{
		from:      "internal/catalog",
		forbidden: []string{"internal/app", "internal/delivery"},
		why:       "the catalog is a projection of the canonical repository and serves the services above it",
	},
	{
		from:      "internal/skillmeta",
		forbidden: []string{"internal/app", "internal/delivery", "internal/canonical", "internal/catalog", "internal/mutation"},
		why:       "skillmeta is the lowest vocabulary package",
	},
	{
		from:      "internal/app",
		forbidden: []string{"internal/delivery"},
		why:       "services never depend on a delivery surface",
	},
}

func TestPackagesImportOnlyDownward(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	used := map[string]bool{}
	for _, r := range rules {
		for _, edge := range importEdges(t, root, r.from) {
			for _, bad := range r.forbidden {
				if edge.to != bad && !strings.HasPrefix(edge.to, bad+"/") {
					continue
				}
				key := edge.fromPackage + " -> " + edge.to
				if _, ok := r.exceptions[key]; ok {
					used[key] = true
					continue
				}
				t.Errorf("%s imports %s (%s): %s", edge.fromPackage, edge.to, edge.file, r.why)
			}
		}
		for key := range r.exceptions {
			if !used[key] {
				t.Errorf("exception %q is no longer needed; remove it from the rule", key)
			}
		}
	}
}

type importEdge struct{ fromPackage, to, file string }

// importEdges lists the module-internal imports of the non-test files under dir.
func importEdges(t *testing.T, root, dir string) []importEdge {
	t.Helper()
	var edges []importEdge
	fset := token.NewFileSet()
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, filepath.Dir(path))
		for _, spec := range file.Imports {
			target := strings.Trim(spec.Path.Value, `"`)
			if strings.HasPrefix(target, modulePath) {
				edges = append(edges, importEdge{filepath.ToSlash(relative), strings.TrimPrefix(target, modulePath), filepath.ToSlash(path)})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].fromPackage+edges[i].to < edges[j].fromPackage+edges[j].to })
	return edges
}

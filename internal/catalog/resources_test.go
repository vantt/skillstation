package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A generation published under an older derived schema is rebuilt on open, and
// the rebuilt one looks up a skill's resources by index instead of a scan.
func TestOlderDerivedGenerationIsRebuiltWithIndexedResourceLookup(t *testing.T) {
	t.Parallel()
	root := newWorkspace(t)
	writeSkillMetadata(t, root, "skills/core/review", "schema_version: 1\nid: review\nname: Review\nstatus: active\ndescription: Review code.\nrouting:\n  triggers: [review code]\n  not_for: [write prose]\n  min_scope: multi_step\n")
	writeCanonical(t, root, "skills/core/review/SKILL.md", "# Review\nUse evidence.\n")
	writeCanonical(t, root, "skills/core/review/scripts/run.sh", "echo ok\n")
	old := build(t, root, BuildOptions{})

	pointerPath := filepath.Join(root, "runtime", "catalog", "current.json")
	contents, err := os.ReadFile(pointerPath)
	if err != nil {
		t.Fatal(err)
	}
	var pointer map[string]any
	if err := json.Unmarshal(contents, &pointer); err != nil {
		t.Fatal(err)
	}
	pointer["derived_schema_version"] = float64(DerivedSchemaVersion - 1)
	if contents, err = json.Marshal(pointer); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pointerPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	handle, err := OpenWithFallback(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if handle.Pointer.Generation == old.Pointer.Generation || handle.Pointer.DerivedSchemaVersion != DerivedSchemaVersion {
		t.Fatalf("open served pointer %+v, want a rebuilt generation", handle.Pointer)
	}
	resources, err := SkillResources(context.Background(), handle.DB, "review")
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, resource := range resources {
		paths = append(paths, resource.Path)
	}
	if want := "skills/core/review/SKILL.md,skills/core/review/scripts/run.sh"; strings.Join(paths, ",") != want {
		t.Fatalf("resources = %v, want %s", paths, want)
	}
	rows, err := handle.DB.Query("EXPLAIN QUERY PLAN "+skillResourcesQuery, "review")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plan := []string{}
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(plan, "; "); !strings.Contains(text, "resources_skill") {
		t.Fatalf("resource lookup plan = %q, want the resources_skill index", text)
	}
}

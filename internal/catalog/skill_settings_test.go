package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/skillruntime"
)

// servedContentDigest is the digest a review approves, computed from what the
// catalog serves: the skill's resource rows and the runtime block of its
// manifest.
func servedContentDigest(t *testing.T, root, id string) string {
	t.Helper()
	build(t, root, BuildOptions{})
	handle, err := OpenCurrent(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	rows, err := handle.DB.Query(`SELECT path,digest FROM resources WHERE skill_id=? ORDER BY path`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var files []skillruntime.ResourceDigest
	for rows.Next() {
		var file skillruntime.ResourceDigest
		if err := rows.Scan(&file.Path, &file.Digest); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(file.Path, "/skill.yaml") || strings.Contains(file.Path, "/.meta/") {
			t.Fatalf("metadata file %s is served as a resource", file.Path)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("the skill serves no resources")
	}
	var contentJSON string
	if err := handle.DB.QueryRow(`SELECT content_json FROM canonical_entities WHERE id=?`, id).Scan(&contentJSON); err != nil {
		t.Fatal(err)
	}
	spec, hasSpec, err := skillruntime.ParseSpec([]byte(contentJSON))
	if err != nil {
		t.Fatal(err)
	}
	return skillruntime.ContentDigest(files, spec, hasSpec)
}

func TestSkillYAMLRoutingKeepsTheContentDigestAndRuntimeChangesIt(t *testing.T) {
	t.Parallel()
	root := newWorkspace(t)
	const dir = "skills/core/digest"
	writeCanonical(t, root, dir+"/SKILL.md", "---\nname: digest\ndescription: Digest fixture.\n---\n\n# Digest\n")
	writeCanonical(t, root, dir+"/scripts/run.sh", "#!/bin/sh\necho run\n")
	writeCanonical(t, root, dir+"/.meta/hub.yaml", "schema_version: 1\nstatus: active\n")
	settings := func(trigger, bins string) string {
		return "schema_version: 1\nruntime:\n    requires:\n        bins: [" + bins + "]\nrouting:\n    triggers: [" + trigger + "]\n    not_for: [unrelated]\n    min_scope: single_step\n"
	}
	writeCanonical(t, root, dir+"/skill.yaml", settings("compute a digest", "git"))
	approved := servedContentDigest(t, root, "digest")

	writeCanonical(t, root, dir+"/skill.yaml", settings("hash the folder", "git"))
	if got := servedContentDigest(t, root, "digest"); got != approved {
		t.Fatalf("a routing edit in skill.yaml changed the content digest: %s -> %s", approved, got)
	}

	writeCanonical(t, root, dir+"/skill.yaml", settings("hash the folder", "git, curl"))
	if got := servedContentDigest(t, root, "digest"); got == approved {
		t.Fatal("a runtime edit in skill.yaml kept the content digest")
	}
}

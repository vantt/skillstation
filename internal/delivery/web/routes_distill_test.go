package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/vantt/skillstation/internal/distill"
	"github.com/vantt/skillstation/internal/workspace"
)

func TestSkillDistillGetRoute(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := workspace.Apply(root); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(root, "skills", "default", "test-audit")
	if err := os.MkdirAll(filepath.Join(skillDir, ".meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Audit\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSkillMeta(t, root, "skills/default/test-audit", "schema_version: 1\nid: test-audit\nstatus: active\n")

	// Read real test-audit distill.yaml from testdata
	testdataBytes, err := os.ReadFile(filepath.Join("..", "..", "migration", "testdata", "test-audit-distill.yaml"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, ".meta", "distill.yaml"), testdataBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, root)

	// 1. GET /api/v1/skills/test-audit/distill -> 200 OK
	rec := get(t, srv, "/api/v1/skills/test-audit/distill")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /distill status = %d: %s", rec.Code, rec.Body.String())
	}
	var gotDoc distill.Document
	if err := json.Unmarshal(rec.Body.Bytes(), &gotDoc); err != nil {
		t.Fatal(err)
	}
	if len(gotDoc.Lessons) != 42 {
		t.Fatalf("expected 42 lessons, got %d", len(gotDoc.Lessons))
	}
	for i, l := range gotDoc.Lessons {
		if l.Key == "" {
			t.Fatalf("lesson[%d] missing key", i)
		}
		if l.Layer == "" {
			t.Fatalf("lesson[%d] (%s) missing layer", i, l.Key)
		}
		if l.Score.Why == "" {
			t.Fatalf("lesson[%d] (%s) missing score", i, l.Key)
		}
		if l.Decision.State == "" {
			t.Fatalf("lesson[%d] (%s) missing decision.state", i, l.Key)
		}
	}

	// 2. A missing skill, or a skill with no distilled knowledge, is not found.
	writeSkillMeta(t, root, "skills/default/plain-skill", "schema_version: 1\nid: plain-skill\nstatus: draft\n")
	for _, path := range []string{"/api/v1/skills/nonexistent/distill", "/api/v1/skills/plain-skill/distill"} {
		rec := get(t, srv, path)
		if rec.Code != http.StatusNotFound || extractErrorCode(t, rec) != "not_found" {
			t.Fatalf("GET %s = %d %s, want 404 not_found", path, rec.Code, rec.Body.String())
		}
	}

	// An id is never a file pattern or a path: neither a glob that matches the
	// one distilled skill nor an escape to a distill.yaml outside the skill
	// folders reaches a file.
	escaped := filepath.Join(root, "escaped", ".meta")
	if err := os.MkdirAll(escaped, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(escaped, "distill.yaml"), testdataBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"%2A", "test-aud%3F%3F", "..%2F..%2Fescaped"} {
		rec := get(t, srv, "/api/v1/skills/"+id+"/distill")
		if rec.Code == http.StatusOK || extractErrorCode(t, rec) == "" {
			t.Fatalf("GET distill of id %q = %d %s, want a refusal", id, rec.Code, rec.Body.String())
		}
	}

	// 3. POST /api/v1/skills/test-audit/distill -> 405 Method Not Allowed
	recPost := postJSON(t, srv, "/api/v1/skills/test-audit/distill", map[string]string{"foo": "bar"})
	if recPost.Code != http.StatusMethodNotAllowed && recPost.Code != http.StatusNotFound {
		t.Fatalf("expected 404 or 405 for POST /distill, got %d", recPost.Code)
	}
}

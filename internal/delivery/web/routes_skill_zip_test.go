package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func uploadSkillZip(t *testing.T, srv *Server, query string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, skillZipUploadPath+query, bytes.NewReader(body))
	req.Host = "127.0.0.1:7421"
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Origin", "http://127.0.0.1:7421")
	req.Header.Set("Content-Type", "application/zip")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

const uploadedSkill = "---\nname: zip-demo\ndescription: Demonstrates a skill that arrives as a zip archive.\nlicense: Apache-2.0\n---\n\n# Zip demo\n\nDo the demo.\n"

// A zip goes through the same preview and confirmation as a GitHub add: the
// proposal names the skill in the archive, confirming creates it as a draft, and
// no temporary folder is left behind.
func TestSkillZipUploadPreviewsAndConfirms(t *testing.T) {
	root := newWebWorkspace(t)
	srv := newTestServer(t, root)
	archive := zipOf(t, map[string]string{
		"zip-demo/SKILL.md":            uploadedSkill,
		"zip-demo/references/guide.md": "# Guide\n",
		"zip-demo/.env":                "SECRET=1\n",
	})

	rec := uploadSkillZip(t, srv, "?filename=zip-demo.zip", archive)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "skill-zip-") || strings.Contains(rec.Body.String(), "SECRET=1") {
		t.Fatalf("proposal leaks the temporary folder or a skipped secret: %s", rec.Body.String())
	}
	var proposal struct {
		SkillID string `json:"skill_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &proposal); err != nil || proposal.SkillID != "zip-demo" {
		t.Fatalf("proposal skill = %q (%v): %s", proposal.SkillID, err, rec.Body.String())
	}

	pins := extractProposalPins(t, rec)
	confirmed := postJSON(t, srv, "/api/v1/skills/add/confirm", confirmationRequest{
		ProposalID: pins.ProposalID, ProposalDigest: pins.ProposalDigest, BaseVersion: pins.BaseVersion,
	})
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirm = %d: %s", confirmed.Code, confirmed.Body.String())
	}
	detail := get(t, srv, "/api/v1/skills/zip-demo")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"draft"`) {
		t.Fatalf("uploaded skill is not a draft: %d %s", detail.Code, detail.Body.String())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(root, "runtime", "tmp", "skill-zip-*")); len(leftovers) != 0 {
		t.Fatalf("temporary folders left behind: %v", leftovers)
	}
}

// An unsafe or invalid archive is refused with a reason and writes nothing.
func TestSkillZipUploadRefusesUnsafeArchives(t *testing.T) {
	root := newWebWorkspace(t)
	srv := newTestServer(t, root)
	for name, test := range map[string]struct {
		body []byte
		why  string
	}{
		"not a zip":      {[]byte("plain text"), "not a readable zip"},
		"path traversal": {zipOf(t, map[string]string{"../evil/SKILL.md": uploadedSkill}), "leaves the archive folder"},
		"absolute path":  {zipOf(t, map[string]string{"/etc/SKILL.md": uploadedSkill}), "absolute path"},
		"no skill":       {zipOf(t, map[string]string{"notes/readme.md": "# Notes\n"}), ""},
	} {
		t.Run(name, func(t *testing.T) {
			before := hashWorkspaceFiles(t, root)
			rec := uploadSkillZip(t, srv, "?filename=x.zip", test.body)
			if rec.Code == http.StatusOK && !strings.Contains(rec.Body.String(), `"error"`) {
				t.Fatalf("archive was accepted: %s", rec.Body.String())
			}
			if test.why != "" && !strings.Contains(rec.Body.String(), test.why) {
				t.Fatalf("want reason %q, got: %s", test.why, rec.Body.String())
			}
			if hashWorkspaceFiles(t, root) != before {
				t.Fatal("a refused archive changed the hub")
			}
		})
	}
	// The route takes only a zip, whatever the body.
	req := httptest.NewRequest(http.MethodPost, skillZipUploadPath, bytes.NewReader([]byte("{}")))
	req.Host = "127.0.0.1:7421"
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Origin", "http://127.0.0.1:7421")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("json body = %d, want 415", rec.Code)
	}
}

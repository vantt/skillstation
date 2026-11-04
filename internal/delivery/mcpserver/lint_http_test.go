package mcpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/app"
)

func postLint(t *testing.T, hub *httpTestHub, token, contentType string, body []byte) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hub.baseURL+HTTPLintPath, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", contentType)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reply, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, reply
}

// The routing check takes the two routing files of a skill the hub has not seen,
// answers with the lint the hub would run, and changes nothing.
func TestLintEndpointChecksRoutingAndChangesNothing(t *testing.T) {
	root := newBundleHubFixture(t)
	hub := startHTTPTestHub(t, root)
	before := skillFolderNames(t, root)
	skill := func(id, md string) []byte {
		body, err := json.Marshal(app.RemoteLintRequest{SkillID: id, SkillMD: md})
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	valid := skill("team-release", "---\nname: team-release\ndescription: Cut a team release.\n---\n\n# Team release\n")

	if response, _ := postLint(t, hub, "", "application/json", valid); response.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", response.StatusCode)
	}

	response, body := postLint(t, hub, hub.token, "application/json", valid)
	var reply app.RemoteLintReply
	if response.StatusCode != http.StatusOK || json.Unmarshal(body, &reply) != nil || reply.SkillID != "team-release" || reply.Description != "Cut a team release." || reply.Findings == nil {
		t.Fatalf("a valid skill: status = %d\n%s", response.StatusCode, body)
	}

	for name, attempt := range map[string]struct {
		contentType string
		body        []byte
		status      int
		code        string
	}{
		"not json media":       {"text/plain", valid, http.StatusUnsupportedMediaType, "invalid_request"},
		"malformed body":       {"application/json", []byte("{"), http.StatusBadRequest, "invalid_request"},
		"an id that is no id":  {"application/json", skill("Not An Id", "---\nname: x\ndescription: y\n---\n"), http.StatusBadRequest, "invalid_request"},
		"an empty SKILL.md":    {"application/json", skill("team-release", ""), http.StatusBadRequest, "invalid_request"},
		"unclosed frontmatter": {"application/json", skill("team-release", "---\nname: x\ndescription: y\n"), http.StatusBadRequest, "invalid_request"},
		"too large":            {"application/json", skill("team-release", strings.Repeat("a", app.MaxRemoteLintBytes)), http.StatusRequestEntityTooLarge, "resource_limits_exceeded"},
	} {
		response, body := postLint(t, hub, hub.token, attempt.contentType, attempt.body)
		if response.StatusCode != attempt.status || errorCode(t, body) != attempt.code {
			t.Errorf("%s: status = %d, code = %q, want %d %q\n%s", name, response.StatusCode, errorCode(t, body), attempt.status, attempt.code, body)
		}
	}

	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, hub.baseURL+HTTPLintPath, nil)
	request.Header.Set("Authorization", "Bearer "+hub.token)
	get, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = get.Body.Close()
	if get.StatusCode != http.StatusMethodNotAllowed || get.Header.Get("Allow") != http.MethodPost {
		t.Errorf("GET: %d, Allow %q", get.StatusCode, get.Header.Get("Allow"))
	}

	if queued, _ := app.ListSubmissions(hub.root); len(queued) != 0 {
		t.Fatalf("a routing check queued %+v", queued)
	}
	if after := skillFolderNames(t, root); after != before {
		t.Fatalf("a routing check changed the hub's skills: %q -> %q", before, after)
	}
}

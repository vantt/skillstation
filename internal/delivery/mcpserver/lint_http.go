package mcpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vantt/skillstation/internal/app"
)

// HTTPLintPath checks the routing of a skill another machine has not sent yet:
// the body is its SKILL.md and skill.yaml as JSON, the reply is what the hub's
// routing lint finds against its active skills. It only reads; nothing is
// queued or written.
const HTTPLintPath = "/mcp/lint"

// lintHTTPHandler serves HTTPLintPath behind the listener's gate, which has
// already checked the Host, the token and the rate limit.
func (adapter *Server) lintHTTPHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			writeBundleError(writer, http.StatusMethodNotAllowed, bundleError{Code: "method_not_allowed", Message: "Use POST."})
			return
		}
		if request.Header.Get("Content-Type") != "application/json" {
			writeBundleError(writer, http.StatusUnsupportedMediaType, bundleError{Code: "invalid_request", Message: "Send the skill as application/json."})
			return
		}
		var lint app.RemoteLintRequest
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, app.MaxRemoteLintBytes)).Decode(&lint); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeBundleError(writer, http.StatusRequestEntityTooLarge, bundleError{Code: "resource_limits_exceeded", Message: "SKILL.md and skill.yaml together are larger than a routing check takes."})
				return
			}
			writeBundleError(writer, http.StatusBadRequest, bundleError{Code: "invalid_request", Message: "The request is not the JSON of a skill_id, skill_md and skill_yaml."})
			return
		}
		reply, err := app.LintProposedSkill(request.Context(), adapter.workspace, lint)
		if err != nil {
			if errors.Is(err, app.ErrRemoteLintInvalid) {
				writeBundleError(writer, http.StatusBadRequest, bundleError{Code: "invalid_request", Message: err.Error()})
				return
			}
			correlation := correlationID()
			adapter.logger.Error("routing lint internal failure", "correlation_id", correlation, "error", err)
			writeBundleError(writer, http.StatusInternalServerError, bundleError{Code: "internal_error", Message: "The routing check failed.", CorrelationID: correlation})
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(reply)
	})
}

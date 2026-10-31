package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vantt/skillstation/internal/app"
)

func init() {
	registerRoutes((*Server).registerDraftRoutes)
}

// registerDraftRoutes lists the checkouts projects on this machine hold and
// lets a person discard one: a preview that names what would be lost, then a
// discard pinned to the folder as previewed.
func (s *Server) registerDraftRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/checkouts", s.handleDrafts)
	mux.HandleFunc("POST /api/v1/checkouts/discard/preview", s.handleDraftDiscardPreview)
	mux.HandleFunc("POST /api/v1/checkouts/discard", s.handleDraftDiscard)
}

type draftRequest struct {
	SkillID     string `json:"skill_id"`
	ProjectRoot string `json:"project_root"`
	Digest      string `json:"digest,omitempty"`
}

func (request draftRequest) valid(needDigest bool) *app.Error {
	if strings.TrimSpace(request.SkillID) == "" || strings.TrimSpace(request.ProjectRoot) == "" || (needDigest && strings.TrimSpace(request.Digest) == "") {
		return app.NewInvalidRequestError("skill_id and project_root are required, and a discard needs the digest its preview gave.", "Open the checkout on the Checkouts page and discard it from there.")
	}
	return nil
}

func writeDraftError(w http.ResponseWriter, err error) {
	if errors.Is(err, app.ErrDraftNotFound) {
		writeAppError(w, app.NewNotFoundError(
			"The hub has no record of that checkout any more.",
			"It may have been committed or discarded; reload the Checkouts page.",
		))
		return
	}
	writeError(w, err)
}

func (s *Server) handleDrafts(w http.ResponseWriter, r *http.Request) {
	list, err := (app.DraftService{}).List(r.Context(), s.workspace)
	if err != nil {
		writeDraftError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleDraftDiscardPreview(w http.ResponseWriter, r *http.Request) {
	var request draftRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAppError(w, err)
		return
	}
	if invalid := request.valid(false); invalid != nil {
		writeAppError(w, invalid)
		return
	}
	preview, err := (app.DraftService{}).PreviewDiscard(r.Context(), s.workspace, request.SkillID, request.ProjectRoot)
	if err != nil {
		writeDraftError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleDraftDiscard(w http.ResponseWriter, r *http.Request) {
	var request draftRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAppError(w, err)
		return
	}
	if invalid := request.valid(true); invalid != nil {
		writeAppError(w, invalid)
		return
	}
	result, err := (app.DraftService{}).ConfirmDiscard(r.Context(), s.workspace, request.SkillID, request.ProjectRoot, request.Digest)
	if err != nil {
		writeDraftError(w, err)
		return
	}
	if result.Error != nil {
		writeAppError(w, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

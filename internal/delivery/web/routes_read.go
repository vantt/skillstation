package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/version"
)

func init() {
	registerRoutes((*Server).registerReadRoutes)
}

func (s *Server) registerReadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/session", s.handleSession)
	mux.HandleFunc("GET /api/v1/home", s.handleHome)
	mux.HandleFunc("GET /api/v1/skills", s.handleSkills)
	mux.HandleFunc("GET /api/v1/skills/{id}", s.handleSkillDetail)
	mux.HandleFunc("GET /api/v1/skills/{id}/review", s.handleSkillReview)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	sum := sha256.Sum256([]byte(s.workspace))
	session := map[string]any{
		"api_version":          1,
		"skillstation_version": version.Version,
		"workspace_id":         "sha256:" + hex.EncodeToString(sum[:]),
		"workspace_name":       filepath.Base(s.workspace),
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	home, err := s.curation.GetCurationHome(r.Context(), s.workspace)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, shellCommandsForWeb(home))
}

// shellCommandsForWeb returns a copy of the home result whose commands are
// shell commands a person can paste, not the MCP tool names agents receive.
func shellCommandsForWeb(home app.CurationHome) app.CurationHome {
	home.Actions = append([]app.ActionItem(nil), home.Actions...)
	for i := range home.Actions {
		if home.Actions[i].Command != "" {
			home.Actions[i].Command = app.ShellCommandFor(home.Actions[i].Command, home.Actions[i].ID)
		}
	}
	home.SuggestedActions = append([]app.Action(nil), home.SuggestedActions...)
	for i := range home.SuggestedActions {
		if home.SuggestedActions[i].Command != "" {
			home.SuggestedActions[i].Command = app.ShellCommandFor(home.SuggestedActions[i].Command, "")
		}
	}
	return home
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	state := params.Get("state")
	if state != "" {
		valid := false
		for _, candidate := range app.SkillStates {
			if state == candidate {
				valid = true
				break
			}
		}
		if !valid {
			writeAppError(w, app.NewInvalidRequestError("Unknown lifecycle state.", "Use draft, active, deprecated or archived."))
			return
		}
	}

	list, err := s.skills.ListSkillCatalog(r.Context(), s.workspace, app.SkillListQuery{
		State:  state,
		Origin: params.Get("origin"),
		Topic:  params.Get("topic"),
		Label:  params.Get("label"),
		Search: params.Get("q"),
		Sort:   params.Get("sort"),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleSkillDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	detail, err := s.skills.GetSkillDetail(r.Context(), s.workspace, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleSkillReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	review, err := s.skills.ReviewSkill(r.Context(), s.workspace, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

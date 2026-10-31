package web

import (
	"net/http"
)

func init() {
	registerRoutes((*Server).registerDistillRoutes)
}

func (s *Server) registerDistillRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/skills/{id}/distill", s.handleGetSkillDistill)
}

func (s *Server) handleGetSkillDistill(w http.ResponseWriter, r *http.Request) {
	doc, err := s.skills.GetSkillDistill(r.Context(), s.workspace, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

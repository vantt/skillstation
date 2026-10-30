package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/skill"
)

func init() {
	registerRoutes((*Server).registerUsageRoutes)
}

func (s *Server) registerUsageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/skills/{id}/usage", s.handleSkillUsage)
	mux.HandleFunc("GET /api/v1/hub-calls", s.handleHubCalls)
	mux.HandleFunc("PUT /api/v1/hub-calls/settings", s.handleHubCallSettings)
}

func (s *Server) handleSkillUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sinceParam := r.URL.Query().Get("since")
	now := s.opts.Now().UTC()
	until := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var since time.Time

	switch sinceParam {
	case "", "30d":
		since = until.AddDate(0, 0, -30)
	case "7d":
		since = until.AddDate(0, 0, -7)
	case "90d":
		since = until.AddDate(0, 0, -90)
	case "180d":
		since = until.AddDate(0, 0, -180)
	default:
		writeAppError(w, app.NewInvalidRequestError("Invalid since parameter.", "Use 7d, 30d, 90d, or 180d."))
		return
	}

	usageService := app.UsageService{
		Skills: s.skills,
	}
	report, err := usageService.Funnel(r.Context(), s.workspace, app.FunnelQuery{
		Since:   since,
		Until:   until,
		SkillID: id,
	})
	if err != nil {
		writeError(w, err, errors.Is(err, skill.ErrNotFound))
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleHubCalls(w http.ResponseWriter, r *http.Request) {
	log, err := app.HubCallService{Now: s.opts.Now}.Log(r.Context(), s.workspace)
	if err != nil {
		writeError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

type hubCallSettingsRequest struct {
	CaptureTaskText bool `json:"capture_task_text"`
}

func (s *Server) handleHubCallSettings(w http.ResponseWriter, r *http.Request) {
	var request hubCallSettingsRequest
	if appErr := decodeJSON(r, &request); appErr != nil {
		writeAppError(w, appErr)
		return
	}
	settings := app.HubCallSettings{CaptureTaskText: request.CaptureTaskText}
	if err := app.SaveHubCallSettings(s.workspace, settings); err != nil {
		writeError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

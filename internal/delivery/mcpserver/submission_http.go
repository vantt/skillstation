package mcpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/skillruntime"
)

// HTTPSubmissionsPath receives a skill another machine sends for review. The
// body is a bundle archive; the reply names the queue entry. A submission only
// waits in a queue on the hub: nothing changes and nothing is served until a
// person on the hub machine approves it.
const HTTPSubmissionsPath = "/mcp/submissions"

// submissionUploadTimeout is how long a submission may take to arrive. The
// server has no body timeout of its own, so a client that stalls would hold its
// upload, and the disk it uses, for good.
var submissionUploadTimeout = 2 * time.Minute

type submissionReply struct {
	SubmissionID string `json:"submission_id"`
	Status       string `json:"status"`
	// NoteKept tells the sender the hub stored its note. A hub from before notes
	// never sets it, which is how the sender learns the note was dropped.
	NoteKept bool `json:"note_kept,omitempty"`
}

// submissionHTTPHandler serves HTTPSubmissionsPath behind the listener's gate,
// which has already checked the Host, the token and the rate limit.
func (adapter *Server) submissionHTTPHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			adapter.serveSubmissionStatus(writer, request)
			return
		}
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", "GET, POST")
			writeBundleError(writer, http.StatusMethodNotAllowed, bundleError{Code: "method_not_allowed", Message: "Use POST to send a skill and GET to ask about one."})
			return
		}
		if request.Header.Get("Content-Type") != skillruntime.BundleMediaType {
			writeBundleError(writer, http.StatusUnsupportedMediaType, bundleError{Code: "invalid_request", Message: "Send the skill as " + skillruntime.BundleMediaType + "."})
			return
		}
		if err := http.NewResponseController(writer).SetReadDeadline(time.Now().Add(submissionUploadTimeout)); err != nil {
			adapter.logger.Warn("submission upload deadline could not be set", "error", err)
		}
		body := http.MaxBytesReader(writer, request.Body, skillruntime.BundleMaxArchiveBytes)
		submission, err := app.AddSubmission(adapter.workspace, body, time.Now())
		if err != nil {
			status, reply := adapter.submissionHTTPError(err)
			writeBundleError(writer, status, reply)
			return
		}
		adapter.logger.Info("skill submission queued", "submission_id", submission.ID, "skill_id", submission.SkillID)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(writer).Encode(submissionReply{SubmissionID: submission.ID, Status: "waiting", NoteKept: submission.Note != ""})
	})
}

func (adapter *Server) submissionHTTPError(err error) (int, bundleError) {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, os.ErrDeadlineExceeded):
		return http.StatusRequestTimeout, bundleError{Code: "request_timeout", Message: "The submission took too long to arrive. Send it again."}
	case errors.Is(err, app.ErrSubmissionBusy):
		return http.StatusServiceUnavailable, bundleError{Code: "submission_busy", Message: err.Error() + ". Try again in a moment."}
	case errors.Is(err, skillruntime.ErrBundleLimit), errors.As(err, &tooLarge):
		return http.StatusRequestEntityTooLarge, bundleError{
			Code:    "resource_limits_exceeded",
			Message: "The skill is larger than a skill may be: at most 512 files and 16 MiB in total.",
		}
	case errors.Is(err, app.ErrSubmissionQueueFull):
		return http.StatusTooManyRequests, bundleError{Code: "submission_queue_full", Message: "The hub's review queue is full. " + err.Error() + ". Ask the person who runs the hub to review or discard waiting submissions."}
	case errors.Is(err, skillruntime.ErrBundleInvalid), errors.Is(err, skillruntime.ErrBundleDigestMismatch), errors.Is(err, app.ErrSubmissionInvalid):
		return http.StatusBadRequest, bundleError{Code: "invalid_request", Message: err.Error()}
	}
	correlation := correlationID()
	adapter.logger.Error("skill submission internal failure", "correlation_id", correlation, "error", err)
	return http.StatusInternalServerError, bundleError{Code: "internal_error", Message: "The submission could not be queued.", CorrelationID: correlation}
}

// serveSubmissionStatus answers GET HTTPSubmissionsPath?id=SUB-…: whether the
// submission still waits for review or how it left the queue. A sender can ask
// about its own submissions with the hub token it already holds; the reply
// names no file and no reviewer.
func (adapter *Server) serveSubmissionStatus(writer http.ResponseWriter, request *http.Request) {
	status, err := app.ReadSubmissionStatus(adapter.workspace, request.URL.Query().Get("id"))
	if err != nil {
		if errors.Is(err, app.ErrSubmissionNotFound) {
			writeBundleError(writer, http.StatusNotFound, bundleError{Code: "not_found", Message: "The hub has no record of that submission: the id is wrong, or its record is older than the hub keeps."})
			return
		}
		correlation := correlationID()
		adapter.logger.Error("submission status internal failure", "correlation_id", correlation, "error", err)
		writeBundleError(writer, http.StatusInternalServerError, bundleError{Code: "internal_error", Message: "The submission status could not be read.", CorrelationID: correlation})
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(status)
}

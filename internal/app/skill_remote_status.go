package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// RemoteSubmissionService asks a hub on another machine what became of a skill
// this machine sent it. It only reads: nothing is queued, written or approved.
type RemoteSubmissionService struct {
	Client *http.Client
}

// RemoteSubmissionResult is what the hub says about one submission.
type RemoteSubmissionResult struct {
	Result
	SubmissionID string `json:"submission_id"`
	SkillID      string `json:"skill_id,omitempty"`
	Hub          string `json:"hub"`
	// Status is "waiting", "applied", "discarded" or "replaced".
	Status      string `json:"status,omitempty"`
	SubmittedAt string `json:"submitted_at,omitempty"`
	DecidedAt   string `json:"decided_at,omitempty"`
	ReplacedBy  string `json:"replaced_by,omitempty"`
}

// Status asks the hub. The token goes only to a hub this machine's own
// settings name; for any other the result says how to trust it, and no request
// is made.
func (service RemoteSubmissionService) Status(ctx context.Context, hub RemoteHub, id string) (RemoteSubmissionResult, error) {
	failed := func(err *Error) (RemoteSubmissionResult, error) {
		return RemoteSubmissionResult{Result: ErrorResult(err), SubmissionID: id, Hub: hub.URL}, nil
	}
	if !submissionIDPattern.MatchString(id) {
		return failed(NewInvalidRequestError(fmt.Sprintf("%q is not a submission id", id), "Use the SUB-… id the send printed."))
	}
	if !hub.MaySendToken {
		return failed(NewInvalidRequestError(
			fmt.Sprintf("only this project's files name the hub at %s, so asking it does not send your hub token", hub.URL),
			"Trust that hub with `"+hub.TrustCommand()+"`, then ask again."))
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return failed(problem)
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return failed(NewInvalidRequestError(token.Problem.Detail, token.Problem.Fix))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, hubURL+remoteSubmissionsPath+"?id="+url.QueryEscape(id), nil)
	if err != nil {
		return RemoteSubmissionResult{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token.Value)
	response, err := remoteHTTPClient(service.Client).Do(request)
	if err != nil {
		return failed(NewInvalidRequestError(fmt.Sprintf("nothing answered at %s: %s", hubURL, probeFailureReason(err)), "Check the connection with `skillstation doctor`."))
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return failed(NewInvalidRequestError(
			fmt.Sprintf("the hub at %s has no record of %s", hubURL, id),
			"Check the id. A record is kept for 90 days after the submission leaves the queue, so an older one is no longer known."))
	case http.StatusMethodNotAllowed:
		return failed(NewInvalidRequestError(
			fmt.Sprintf("the hub at %s runs a Skillstation from before submission status", hubURL),
			"Ask the person who runs the hub, or look at the Waiting for review page there; updating the hub makes this command work."))
	default:
		return failed(remoteRefusal(hubURL, response.StatusCode, body, "ask about"))
	}
	var reply SubmissionStatus
	if json.Unmarshal(body, &reply) != nil || reply.ID != id || reply.Status == "" {
		return failed(NewInvalidRequestError(fmt.Sprintf("the hub at %s answered, but not as a skillstation that reports submissions", hubURL), "Check that the hub runs the same skillstation version as this machine."))
	}
	return RemoteSubmissionResult{
		Result:       NewResult(StatusOK, SubmissionStatusSummary(reply)),
		SubmissionID: id, SkillID: reply.SkillID, Hub: hubURL, Status: reply.Status,
		SubmittedAt: reply.SubmittedAt, DecidedAt: reply.DecidedAt, ReplacedBy: reply.ReplacedBy,
	}, nil
}

// SubmissionStatusSummary says in a sentence what the status of a submission
// means to the person who sent it; the hub machine's owner reads the same
// sentence.
func SubmissionStatusSummary(status SubmissionStatus) string {
	switch status.Status {
	case SubmissionWaiting:
		return fmt.Sprintf("%s (%s) is waiting for a person on the hub to review it.", status.ID, status.SkillID)
	case SubmissionApplied:
		return fmt.Sprintf("%s (%s) was reviewed and applied on the hub on %s. A new skill may still be a draft until the person who runs the hub activates it.", status.ID, status.SkillID, dayOf(status.DecidedAt))
	case SubmissionDiscarded:
		return fmt.Sprintf("%s (%s) was discarded by the person who reviews it, on %s. Nothing changed on the hub, and your folder is as you left it.", status.ID, status.SkillID, dayOf(status.DecidedAt))
	case SubmissionReplaced:
		return fmt.Sprintf("%s (%s) was replaced on %s by %s, a newer submission of the same skill from this machine. Ask about that one.", status.ID, status.SkillID, dayOf(status.DecidedAt), status.ReplacedBy)
	}
	return fmt.Sprintf("%s (%s): %s.", status.ID, status.SkillID, status.Status)
}

func dayOf(timestamp string) string {
	if len(timestamp) >= 10 {
		return timestamp[:10]
	}
	return timestamp
}

// CheckedOutInProject reports whether the project of projectDir holds a
// checkout of the skill, which `skill pull` can bring up to date. A folder the
// person wrote and sent is not one.
func CheckedOutInProject(ctx context.Context, projectDir, id string) bool {
	shared, claude := projectSkillFolders(projectRootOf(ctx, projectDir), id)
	for _, folder := range []projectFolder{shared, claude} {
		if _, found, err := ReadCheckoutMarker(folder.Dir); err == nil && found {
			return true
		}
	}
	return false
}

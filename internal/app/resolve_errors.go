package app

import (
	"errors"

	resolverpkg "github.com/vantt/skillstation/internal/resolver"
)

// priorNeedsCurrentCatalog refuses a request with a prior while the catalog is
// not current. A prior is checked against the catalog it was issued on, so
// the hub does not rebuild for it; a request without prior does.
func priorNeedsCurrentCatalog() *Error {
	return &Error{Code: ErrorIndexStale, Retryable: true, Render: ErrorRender{
		Error: "The hub's skill index is out of date, so the prior cannot be checked.",
		Why:   "A request with prior needs the current index, and a request with prior never rebuilds it.",
		Fix:   "Resolve again without prior; that rebuilds the index. If the error comes back, tell the user to run `skillstation doctor --fix` in the hub.",
	}}
}

// classifyResolveRequestError renders a refusal of a skill_resolve request by
// its kind. MCP shows only the ERROR line and the FIX of an error, so a
// refusal whose cause the caller must read names that cause in ERROR.
func classifyResolveRequestError(err error) *Error {
	var refused *resolverpkg.RequestError
	if !errors.As(err, &refused) {
		return nil
	}
	reason := refused.Reason
	switch {
	case errors.Is(refused, resolverpkg.ErrUnsupportedSchema):
		return &Error{Code: ErrorUnsupportedSchema, Render: ErrorRender{
			Error: "The resolve request uses an unsupported schema version.",
			Why:   "This hub speaks version 1 of the skill_resolve request: " + reason + ".",
			Fix:   `Send schema_version "1" with the fields that version defines.`,
		}}
	case errors.Is(refused, resolverpkg.ErrIncompleteClarification):
		return &Error{Code: ErrorInvalidRequest, Render: ErrorRender{
			Error: "The clarification answer cannot be accepted: " + reason + ".",
			Why:   "A clarification prior answers the question of a needs_context answer, so it names that question and the chosen answer.",
			Fix:   "Send the same request with prior.question_id set to question.id and prior.answer set to one of question.choices, or leave prior out.",
		}}
	case errors.Is(refused, resolverpkg.ErrUnknownPrior):
		return &Error{Code: ErrorUnknownResolution, Retryable: true, Render: ErrorRender{
			Error: "The prior resolution is unavailable.",
			Why:   reason + ".",
			Fix:   "Resolve again without prior.",
		}}
	case errors.Is(refused, resolverpkg.ErrStalePrior):
		return &Error{Code: ErrorStaleContext, Retryable: true, Render: ErrorRender{
			Error: "The prior resolution no longer matches the current request.",
			Why:   reason + ".",
			Fix:   "Resolve again without prior, with the current task.",
		}}
	case errors.Is(refused, resolverpkg.ErrRetryExhausted):
		return &Error{Code: ErrorResolutionRetryExhausted, Render: ErrorRender{
			Error: "This task cannot be re-resolved again: " + reason + ".",
			Why:   "A task gets at most two re-resolutions after its answers are rejected.",
			Fix:   "Continue without a hub skill. A corrected scope or constraint is the same task; only a different task description starts again.",
		}}
	case errors.Is(refused, resolverpkg.ErrClarificationExhausted):
		return &Error{Code: ErrorClarificationBudgetExhausted, Render: ErrorRender{
			Error: "The hub cannot ask another question for this task: " + reason + ".",
			Why:   "A task gets at most one clarification round, asked on its first answer.",
			Fix:   "Narrow the task description, or continue without a hub skill.",
		}}
	default:
		return &Error{Code: ErrorInvalidRequest, Render: ErrorRender{
			Error: "The resolve request cannot be accepted: " + reason + ".",
			Why:   "A value breaks a rule of the skill_resolve request.",
			Fix:   "Correct the field the error names and send the request again.",
		}}
	}
}

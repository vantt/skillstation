package resolver

import (
	"errors"
	"fmt"
)

// The kinds of RequestError. Callers match them with errors.Is.
var (
	// ErrInvalidRequest marks a value that breaks a rule of the request
	// contract.
	ErrInvalidRequest = errors.New("invalid resolve request")
	// ErrIncompleteClarification marks a clarification prior without the
	// question it answers or without the answer.
	ErrIncompleteClarification = errors.New("incomplete clarification prior")
	// ErrUnsupportedSchema marks a request for a schema version this resolver
	// does not speak.
	ErrUnsupportedSchema = errors.New("unsupported schema_version")
	// ErrUnknownPrior marks a prior that names a resolution this resolver did
	// not issue for the request, or no longer remembers.
	ErrUnknownPrior = errors.New("unknown prior resolution")
	// ErrStalePrior marks a prior whose revision or question no longer matches
	// the resolution it names.
	ErrStalePrior = errors.New("stale prior resolution")
	// ErrRetryExhausted marks a rejection beyond the re-resolution budget of
	// one stable task.
	ErrRetryExhausted = errors.New("re-resolution budget exhausted")
	// ErrClarificationExhausted marks a clarification answer on a chain that
	// already used its clarification round.
	ErrClarificationExhausted = errors.New("clarification budget exhausted")
)

// RequestError is a refusal of a request as sent. Reason is a short, safe
// sentence naming the broken rule; it never repeats request values.
type RequestError struct {
	Kind   error
	Reason string
}

func (e *RequestError) Error() string { return e.Reason }

func (e *RequestError) Unwrap() error { return e.Kind }

func requestError(kind error, format string, args ...any) error {
	return &RequestError{Kind: kind, Reason: fmt.Sprintf(format, args...)}
}

func invalidRequest(format string, args ...any) error {
	return requestError(ErrInvalidRequest, format, args...)
}

// CheckSchemaVersion refuses a request schema version other than the one
// this resolver speaks. Adapters call it before any other validation, so a
// request written for another version gets unsupported_schema rather than a
// complaint about a field that version defines.
func CheckSchemaVersion(version string) error {
	if version != SchemaVersion {
		return requestError(ErrUnsupportedSchema, "schema_version must be %q", SchemaVersion)
	}
	return nil
}

package resolver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	maxRequestIDBytes   = 128
	maxDescriptionBytes = 4096
	maxConstraintBytes  = 512
	maxConstraints      = 16
	maxFacts            = 32
	maxFactBytes        = 256
	maxCapabilities     = 32
	maxActiveProcedures = 8
)

var operations = setOf("", "explore", "design", "implement", "review", "debug", "test", "refactor", "migrate", "document", "operate", "research", "other")

// SupportedOperations are the operations a request may name, in the order the
// request schema lists them.
var SupportedOperations = []string{"explore", "design", "implement", "review", "debug", "test", "refactor", "migrate", "document", "operate", "research", "other"}

// IsSupportedOperation reports whether a request may name operation.
func IsSupportedOperation(operation string) bool {
	return operation != "" && operations[strings.ToLower(strings.TrimSpace(operation))]
}

var scopes = setOf("", "single_step", "multi_step", "project")
var factBases = setOf("user", "tool", "agent-host", "agent-inference")
var activationModes = setOf("allow-primary", "supplement-only", "coverage-check")
var procedureRoles = setOf("primary", "supporting")
var stateBases = setOf("host-native", "adapter-tracked", "user-selected", "agent-reported")

func NormalizeRequest(request Request) (Request, error) {
	if err := validateRequestUTF8(request); err != nil {
		return Request{}, err
	}
	if request.SchemaVersion != SchemaVersion {
		return Request{}, fmt.Errorf("unsupported schema_version %q", request.SchemaVersion)
	}
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.RequestID == "" || len(request.RequestID) > maxRequestIDBytes {
		return Request{}, fmt.Errorf("request_id must contain 1..%d bytes", maxRequestIDBytes)
	}
	request.Task.Description = normalizeText(request.Task.Description)
	if request.Task.Description == "" || len(request.Task.Description) > maxDescriptionBytes {
		return Request{}, fmt.Errorf("task.description must contain 1..%d bytes", maxDescriptionBytes)
	}
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	if !operations[request.Operation] {
		return Request{}, fmt.Errorf("operation is unsupported")
	}
	request.Task.Scope = strings.ToLower(strings.TrimSpace(request.Task.Scope))
	if !scopes[request.Task.Scope] {
		return Request{}, fmt.Errorf("task.scope is unsupported")
	}
	if err := normalizeTask(&request.Task); err != nil {
		return Request{}, err
	}
	if err := normalizeRequestContext(&request.Context); err != nil {
		return Request{}, err
	}
	if err := normalizeActivationContext(request.ActivationContext); err != nil {
		return Request{}, err
	}
	if err := normalizePrior(request.Prior); err != nil {
		return Request{}, err
	}
	return request, nil
}

// normalizeTask sorts and deduplicates the task constraints and exclusions.
func normalizeTask(task *Task) error {
	if len(task.Constraints) > maxConstraints {
		return fmt.Errorf("task.constraints exceeds %d entries", maxConstraints)
	}
	var constraints []string
	seen := map[string]bool{}
	for _, value := range task.Constraints {
		value = normalizeText(value)
		if value == "" || len(value) > maxConstraintBytes {
			return fmt.Errorf("task.constraints entries must contain 1..%d bytes", maxConstraintBytes)
		}
		if !seen[value] {
			constraints = append(constraints, value)
			seen[value] = true
		}
	}
	sort.Strings(constraints)
	task.Constraints = constraints
	normalizedSkillIDs, normalizeErr := normalizeIdentifiers(task.Exclusions.SkillIDs, maxConstraints)
	if normalizeErr != nil {
		return fmt.Errorf("task.exclusions.skill_ids: %w", normalizeErr)
	}
	task.Exclusions.SkillIDs = normalizedSkillIDs
	if len(task.Exclusions.Terms) > maxConstraints {
		return fmt.Errorf("task.exclusions.terms exceeds %d entries", maxConstraints)
	}
	task.Exclusions.Terms = normalizeSortedText(task.Exclusions.Terms)
	return nil
}

// normalizeRequestContext normalizes the active artifact, facts, execution
// capabilities and signal of a request.
func normalizeRequestContext(requestContext *RequestContext) error {
	if artifact := requestContext.ActiveArtifact; artifact != nil {
		artifact.Kind = normalizeIdentifier(artifact.Kind)
		artifact.Language = normalizeIdentifier(artifact.Language)
		artifact.PathHint = strings.TrimSpace(norm.NFKC.String(artifact.PathHint))
		if artifact.PathHint != "" {
			normalizedPath := strings.ReplaceAll(artifact.PathHint, "\\", "/")
			cleaned := path.Clean(normalizedPath)
			windowsVolume := len(normalizedPath) >= 2 && ((normalizedPath[0] >= 'A' && normalizedPath[0] <= 'Z') || (normalizedPath[0] >= 'a' && normalizedPath[0] <= 'z')) && normalizedPath[1] == ':'
			if len(artifact.PathHint) > 512 || strings.ContainsRune(artifact.PathHint, '\x00') || path.IsAbs(normalizedPath) || windowsVolume || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
				return fmt.Errorf("context.active_artifact.path_hint must be a safe relative path hint")
			}
			artifact.PathHint = cleaned
		}
	}
	if len(requestContext.Facts) > maxFacts {
		return fmt.Errorf("context.facts exceeds %d entries", maxFacts)
	}
	for index := range requestContext.Facts {
		fact := &requestContext.Facts[index]
		fact.Key = normalizeIdentifier(fact.Key)
		fact.Value = normalizeText(fact.Value)
		fact.Basis = strings.ToLower(strings.TrimSpace(fact.Basis))
		fact.Scope = normalizeIdentifier(fact.Scope)
		if fact.Key == "" || fact.Value == "" || len(fact.Key) > maxFactBytes || len(fact.Value) > maxFactBytes {
			return fmt.Errorf("context.facts entries require bounded key and value")
		}
		if !factBases[fact.Basis] {
			return fmt.Errorf("context.facts[%d].basis is unsupported", index)
		}
	}
	sort.Slice(requestContext.Facts, func(i, j int) bool {
		a, b := requestContext.Facts[i], requestContext.Facts[j]
		return factSortKey(a) < factSortKey(b)
	})
	requestContext.Facts = deduplicateFacts(requestContext.Facts)
	var err error
	requestContext.Execution.Capabilities, err = normalizeIdentifiers(requestContext.Execution.Capabilities, maxCapabilities)
	if err != nil {
		return fmt.Errorf("context.execution.capabilities: %w", err)
	}
	requestContext.Execution.UnavailableCapabilities, err = normalizeIdentifiers(requestContext.Execution.UnavailableCapabilities, maxCapabilities)
	if err != nil {
		return fmt.Errorf("context.execution.unavailable_capabilities: %w", err)
	}
	available := sliceSet(requestContext.Execution.Capabilities)
	for _, value := range requestContext.Execution.UnavailableCapabilities {
		if available[value] {
			return fmt.Errorf("capability %q cannot be both available and unavailable", value)
		}
	}
	if signal := requestContext.Execution.Signal; signal != nil {
		signal.Kind = normalizeIdentifier(signal.Kind)
		signal.Summary = normalizeText(signal.Summary)
		if signal.Kind == "" || signal.Summary == "" || len(signal.Summary) > 512 {
			return fmt.Errorf("context.execution.signal requires bounded kind and summary")
		}
	}
	return nil
}

// normalizeActivationContext checks the activation mode and the procedures
// the host reports as active; a missing context is valid.
func normalizeActivationContext(activation *ActivationContext) error {
	if activation == nil {
		return nil
	}
	activation.Mode = strings.ToLower(strings.TrimSpace(activation.Mode))
	if !activationModes[activation.Mode] {
		return fmt.Errorf("activation_context.mode is unsupported")
	}
	if len(activation.ActiveProcedures) > maxActiveProcedures {
		return fmt.Errorf("activation_context.active_procedures exceeds %d entries", maxActiveProcedures)
	}
	for index := range activation.ActiveProcedures {
		procedure := &activation.ActiveProcedures[index]
		procedure.ID = strings.TrimSpace(procedure.ID)
		procedure.Role = strings.ToLower(strings.TrimSpace(procedure.Role))
		procedure.Scope = normalizeText(procedure.Scope)
		procedure.Summary = normalizeText(procedure.Summary)
		procedure.StateBasis = strings.ToLower(strings.TrimSpace(procedure.StateBasis))
		procedure.LockedBy = normalizeText(procedure.LockedBy)
		if procedure.Identity != nil {
			procedure.Identity.Digest = strings.TrimSpace(procedure.Identity.Digest)
			procedure.Identity.Origin = normalizeText(procedure.Identity.Origin)
		}
		if procedure.ID == "" || len(procedure.ID) > 256 || procedure.Scope == "" || !procedureRoles[procedure.Role] || !stateBases[procedure.StateBasis] {
			return fmt.Errorf("activation_context.active_procedures[%d] is invalid", index)
		}
	}
	sort.Slice(activation.ActiveProcedures, func(i, j int) bool {
		a, b := activation.ActiveProcedures[i], activation.ActiveProcedures[j]
		return a.ID+"\x00"+a.Role+"\x00"+a.Scope+"\x00"+a.StateBasis < b.ID+"\x00"+b.Role+"\x00"+b.Scope+"\x00"+b.StateBasis
	})
	for index := 1; index < len(activation.ActiveProcedures); index++ {
		if activation.ActiveProcedures[index-1].ID == activation.ActiveProcedures[index].ID {
			return fmt.Errorf("activation_context.active_procedures contains duplicate id %q", activation.ActiveProcedures[index].ID)
		}
	}
	return nil
}

// normalizePrior checks the earlier resolution a request refers to; a missing
// prior is valid.
func normalizePrior(prior *Prior) error {
	if prior == nil {
		return nil
	}
	prior.ResolutionID = strings.TrimSpace(prior.ResolutionID)
	prior.Kind = strings.ToLower(strings.TrimSpace(prior.Kind))
	prior.QuestionID = normalizeIdentifier(prior.QuestionID)
	prior.Answer = normalizeText(prior.Answer)
	prior.Basis = normalizeIdentifier(prior.Basis)
	prior.ReasonCode = normalizeIdentifier(prior.ReasonCode)
	if prior.ResolutionID == "" || prior.ContextRevision < 1 || (prior.Kind != "clarification" && prior.Kind != "rejected") {
		return fmt.Errorf("prior is invalid")
	}
	if prior.Kind == "clarification" && (prior.QuestionID == "" || prior.Answer == "") {
		return fmt.Errorf("prior clarification requires question_id and answer")
	}
	return nil
}

func validateRequestUTF8(request Request) error {
	values := []string{request.SchemaVersion, request.RequestID, request.Task.Description, request.Task.Scope, request.Operation}
	values = append(values, request.Task.Constraints...)
	values = append(values, request.Task.Exclusions.SkillIDs...)
	values = append(values, request.Task.Exclusions.Terms...)
	if artifact := request.Context.ActiveArtifact; artifact != nil {
		values = append(values, artifact.Kind, artifact.PathHint, artifact.Language)
	}
	for _, fact := range request.Context.Facts {
		values = append(values, fact.Key, fact.Value, fact.Basis, fact.Scope)
	}
	values = append(values, request.Context.Execution.Capabilities...)
	values = append(values, request.Context.Execution.UnavailableCapabilities...)
	if signal := request.Context.Execution.Signal; signal != nil {
		values = append(values, signal.Kind, signal.Summary)
	}
	if activation := request.ActivationContext; activation != nil {
		values = append(values, activation.Mode)
		for _, procedure := range activation.ActiveProcedures {
			values = append(values, procedure.ID, procedure.Role, procedure.Scope, procedure.Summary, procedure.StateBasis, procedure.LockedBy)
			if procedure.Identity != nil {
				values = append(values, procedure.Identity.Digest, procedure.Identity.Origin)
			}
		}
	}
	if prior := request.Prior; prior != nil {
		values = append(values, prior.ResolutionID, prior.Kind, prior.QuestionID, prior.Answer, prior.Basis, prior.ReasonCode)
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return fmt.Errorf("request strings must be valid UTF-8")
		}
	}
	return nil
}

func normalizeText(value string) string {
	value = norm.NFKC.String(value)
	return strings.Join(strings.FieldsFunc(value, unicode.IsSpace), " ")
}

func normalizeIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFKC.String(value)))
}

func normalizeSortedText(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeText(value)
		if value != "" && !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	sort.Strings(result)
	return result
}

func factSortKey(fact Fact) string {
	return fact.Key + "\x00" + fact.Value + "\x00" + fact.Basis + "\x00" + fact.Scope + "\x00" + fact.ObservedAt.UTC().Format(time.RFC3339Nano)
}

func deduplicateFacts(values []Fact) []Fact {
	result := values[:0]
	last := ""
	for _, value := range values {
		key := factSortKey(value)
		if len(result) == 0 || key != last {
			result = append(result, value)
			last = key
		}
	}
	return result
}

func normalizeIdentifiers(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		return nil, fmt.Errorf("exceeds %d entries", limit)
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeIdentifier(value)
		if value == "" || len(value) > 128 || !utf8.ValidString(value) {
			return nil, fmt.Errorf("entries must be bounded non-empty UTF-8 strings")
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	sort.Strings(result)
	return result, nil
}

func fingerprint(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func setOf(values ...string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
func sliceSet(values []string) map[string]bool { return setOf(values...) }

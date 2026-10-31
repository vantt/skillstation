package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vantt/skillstation/internal/app"
	resolverpkg "github.com/vantt/skillstation/internal/resolver"
)

func (adapter *Server) registerResolverTools(tools *toolRegistry) {
	addTool(tools, &mcp.Tool{
		Name: "skill_resolve", Title: "Resolve a skill",
		Description: "Pick at most one skill for the current task from task.description (in English). Then load the recommended skill with skill_get, or with skills/get and resources/read if your host supports the MCP skills extension.",
		Annotations: annotations(true, false, true, false),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input resolveInput) (*mcp.CallToolResult, toolOutcome[resolveResult], error) {
		if err := validateResolverArguments(req.Params.Arguments); err != nil {
			return nil, toolOutcome[resolveResult]{}, err
		}
		request := resolverpkg.Request(input)
		if _, err := resolverpkg.NormalizeRequest(request); err != nil {
			return failure[resolveResult](err)
		}
		session := toolSession(req)
		caller := adapter.callerContext(session)
		ctx = app.WithProjectDir(app.WithCallerContext(ctx, caller), adapter.projectDir)

		response, err := adapter.resolver.Resolve(ctx, adapter.workspace, request)
		if err != nil {
			return failure[resolveResult](err)
		}
		if adapter.tracker != nil {
			priorVer := false
			if request.Prior != nil {
				priorVer = caller.VerifyPrior(request.Prior.ResolutionID)
			}
			adapter.tracker.noteResolution(session, request, response, priorVer, caller.Client)
		}
		trueValue := true
		return success(resolveResult{
			Resolution: response, ActivationContext: &trueValue, Feedback: &trueValue,
			SchemaMajor: SchemaVersion, ResourceVersionPinning: true,
		})
	}, withCommittedResolverSchemas())

	addTool(tools, &mcp.Tool{
		Name: "skill_feedback", Title: "Record skill feedback",
		Description: "Report how a skill worked out: resolution_id, a new event_id, outcome; selected_skill may name a skill it missed.",
		Annotations: annotations(false, false, true, false),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input feedbackInput) (*mcp.CallToolResult, toolOutcome[app.FeedbackResult], error) {
		if input.SchemaVersion != SchemaVersion || input.ResolutionID == "" || input.EventID == "" || input.Outcome == "" {
			return failure[app.FeedbackResult](fmt.Errorf("schema_version, resolution_id, event_id, and outcome are required"))
		}
		session := toolSession(req)
		caller := adapter.callerContext(session)
		ctx = app.WithCallerContext(ctx, caller)

		return appResult(adapter.feedback.Record(ctx, adapter.workspace, app.FeedbackInput{
			SchemaVersion: input.SchemaVersion, ResolutionID: input.ResolutionID, EventID: input.EventID,
			Outcome: input.Outcome, ReasonCode: input.ReasonCode, SelectedSkill: input.SelectedSkill,
			Utility: input.Utility, Basis: input.Basis,
		}))
	}, withPropertyEnums(map[string][]any{
		"outcome":     {"activated", "loaded", "used", "abandoned", "rejected", "completed", "failed"},
		"reason_code": {"user_rejected", "scope_mismatch", "capability_unavailable", "constraint_conflict", "workflow_completed", "workflow_failed", "abandoned", "host_report", "setup_failed"},
		"utility":     {"helpful", "harmful", "neutral"},
		"basis":       {"user", "evaluator", "controlled-benchmark"},
	}))
}

// resolverRequestTools are the tools whose arguments are a skill_resolve
// request.
var resolverRequestTools = map[string]bool{"skill_resolve": true, "routing_evaluate": true}

// refuseOtherSchemaVersions answers a resolver request for another schema
// version with unsupported_schema in the tool's error envelope. It runs before
// the SDK checks the arguments against the advertised schema, which would
// refuse that version, and any field only that version defines, as plain text.
func refuseOtherSchemaVersions(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if call, ok := req.(*mcp.CallToolRequest); ok && method == "tools/call" && call.Params != nil && resolverRequestTools[call.Params.Name] {
			if refused := unsupportedSchemaResult(call.Params.Arguments); refused != nil {
				return refused, nil
			}
		}
		return next(ctx, method, req)
	}
}

// unsupportedSchemaResult is the tool result for arguments that name another
// schema version, or nil for any other arguments.
func unsupportedSchemaResult(raw json.RawMessage) *mcp.CallToolResult {
	var arguments struct {
		SchemaVersion *string `json:"schema_version"`
	}
	if json.Unmarshal(raw, &arguments) != nil || arguments.SchemaVersion == nil {
		return nil
	}
	err := resolverpkg.CheckSchemaVersion(*arguments.SchemaVersion)
	if err == nil {
		return nil
	}
	result, outcome, _ := failure[resolveResult](err)
	encoded, err := json.Marshal(outcome)
	if err != nil {
		return nil
	}
	result.StructuredContent = json.RawMessage(encoded)
	result.Content = []mcp.Content{&mcp.TextContent{Text: string(encoded)}}
	return result
}

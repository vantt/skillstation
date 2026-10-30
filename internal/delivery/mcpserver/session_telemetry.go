package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/telemetry"
)

// toolSession returns the session of a tool call; a nil request has none.
func toolSession(req *mcp.CallToolRequest) *mcp.ServerSession {
	if req == nil {
		return nil
	}
	return req.Session
}

func (adapter *Server) clientForReq(req mcp.Request) telemetry.Client {
	var session *mcp.ServerSession
	if req != nil {
		session, _ = req.GetSession().(*mcp.ServerSession)
	}
	return adapter.sessionClient(session)
}

// sessionClient names the host of a session for telemetry: the normalized
// clientInfo from the initialize handshake ("other" when absent), labelled
// remote when the session came through the network listener.
func (adapter *Server) sessionClient(session *mcp.ServerSession) telemetry.Client {
	client := telemetry.Client{Name: "other"}
	if session != nil {
		if params := session.InitializeParams(); params != nil && params.ClientInfo != nil {
			client = NormalizeClient(params.ClientInfo.Name, params.ClientInfo.Version)
		}
	}
	if adapter.remote {
		client = telemetry.RemoteClient(client)
	}
	return client
}

func (adapter *Server) telemetryMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		started := time.Now()
		result, err := next(ctx, method, req)
		if logged, ok := hubCallMethods[method]; ok {
			// Logging stays off the request path: the caller gets its answer
			// without waiting for the catalog pins or the recorder queue.
			elapsed := time.Since(started)
			go adapter.recordHubCall(context.WithoutCancel(ctx), logged, req, result, err, elapsed)
		}
		if err == nil && method == "tools/list" {
			if size, ok := adapter.toolsListBytes(result); ok {
				app.RecordServerMetric(ctx, adapter.telemetry, adapter.workspace, app.ServerMetric{Name: "tools_list_bytes", Value: float64(size), Client: adapter.clientForReq(req)})
			}
		}
		return result, err
	}
}

// hubCallMethods maps the requests that are logged as hub calls to their
// event vocabulary. A client that uses the initialize handshake is logged by
// recordClientConnected instead; server/discover is its stateless counterpart.
var hubCallMethods = map[string]string{
	"server/discover": "connect",
	"tools/list":      "tools_list",
	"tools/call":      "tools_call",
	"skills/get":      "skills_get",
	"resources/read":  "resources_read",
}

// recordHubCall logs one served request: which client called, what it asked
// for and whether it succeeded. Arguments and results are never recorded.
func (adapter *Server) recordHubCall(ctx context.Context, method string, req mcp.Request, result mcp.Result, err error, elapsed time.Duration) {
	call := app.HubCall{Method: method, Duration: elapsed, Client: adapter.clientForReq(req)}
	switch params := req.(type) {
	case *mcp.CallToolRequest:
		if params.Params != nil {
			call.Tool = params.Params.Name
		}
	}
	if err != nil {
		call.Failed = true
		call.ErrorCode = string(app.ClassifyError(err).Code)
	} else if toolResult, ok := result.(*mcp.CallToolResult); ok && toolResult.IsError {
		call.Failed = true
		call.ErrorCode = toolResultErrorCode(toolResult)
	}
	app.RecordHubCall(ctx, adapter.telemetry, adapter.workspace, call)
}

// recordClientConnected logs the handshake of a client that finished
// initializing, which the receiving middleware never sees.
func (adapter *Server) recordClientConnected(ctx context.Context, req *mcp.InitializedRequest) {
	go app.RecordHubCall(ctx, adapter.telemetry, adapter.workspace, app.HubCall{Method: "connect", Client: adapter.clientForReq(req)})
}

// toolResultErrorCode reads the error code a failed tool call reports in its
// structured content.
func toolResultErrorCode(result *mcp.CallToolResult) string {
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return ""
	}
	var outcome struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(encoded, &outcome) != nil || outcome.Error == nil {
		return ""
	}
	return outcome.Error.Code
}

// toolsListBytes reports the encoded size of a tools/list result. The server
// returns the same registered tool objects until the tool set changes, so the
// result is encoded once per tool set and the size is reused for later calls.
func (adapter *Server) toolsListBytes(result mcp.Result) (int, bool) {
	var key strings.Builder
	if listed, ok := result.(*mcp.ListToolsResult); ok {
		for _, tool := range listed.Tools {
			fmt.Fprintf(&key, "%p;", tool)
		}
		key.WriteString(listed.NextCursor)
	}
	adapter.toolsListMu.Lock()
	defer adapter.toolsListMu.Unlock()
	if key.Len() > 0 && adapter.toolsListKey == key.String() {
		return adapter.toolsListSize, true
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return 0, false
	}
	adapter.toolsListKey, adapter.toolsListSize = key.String(), len(encoded)
	return len(encoded), true
}

func (adapter *Server) callerContext(session *mcp.ServerSession) app.CallerContext {
	sessionHash := ""
	client := adapter.sessionClient(session)
	var verifier func(string) bool
	var lookup func(string) (app.PriorResolution, bool)
	if adapter.tracker != nil && session != nil {
		sessionHash = adapter.tracker.sessionHash(session)
		verifier = func(resID string) bool {
			return adapter.tracker.hasResolution(session, resID)
		}
		lookup = func(resID string) (app.PriorResolution, bool) {
			noted, ok := adapter.tracker.resolutionData(session, resID)
			return app.PriorResolution{PrimaryID: noted.PrimaryID, TaskDescription: noted.TaskDescription}, ok
		}
	}
	return app.CallerContext{
		SessionHash:   sessionHash,
		Client:        client,
		PriorVerifier: verifier,
		PriorLookup:   lookup,
	}
}

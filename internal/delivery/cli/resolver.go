package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/delivery/cli/termui"
	resolverpkg "github.com/vantt/skillstation/internal/resolver"
	"github.com/vantt/skillstation/internal/telemetry"
)

const maxResolveRequestBytes = 64 << 10

func runResolve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	workspacePath, requestPath, jsonOutput, err := resolveFlags(args)
	if err != nil {
		var resErr *WorkspaceResolutionError
		if errors.As(err, &resErr) {
			return writeWorkspaceResolutionError(stdout, stderr, hasJSONFlag(args), resErr)
		}
		return writeInvalidRequest(stdout, stderr, hasJSONFlag(args), err.Error(), "Run `skillstation resolve --workspace <path> --request <request.json> --json`.")
	}
	request, err := readResolveRequest(requestPath)
	var refused *resolverpkg.RequestError
	if errors.As(err, &refused) {
		return writeStructuredError(stdout, stderr, jsonOutput, app.ClassifyError(err))
	}
	if err != nil {
		return writeInvalidRequest(stdout, stderr, jsonOutput, err.Error(), "Correct the versioned resolve request and retry.")
	}
	service := app.ResolverService{}
	// Telemetry is best-effort and disposable. Failure to open, record, flush, or
	// close it must never alter the resolver response or process exit status.
	if recorder, telemetryErr := (app.TelemetryService{}).Open(workspacePath); telemetryErr == nil {
		service.Telemetry = recorder
		defer closeResolutionTelemetry(recorder)
	}
	response, err := service.Resolve(ctx, workspacePath, request)
	if errors.Is(err, context.Canceled) {
		return writeWorkspaceResult(app.Result{}, err, stdout, stderr, jsonOutput)
	}
	if err != nil {
		if classified := classifyResolveError(err); classified != nil {
			return writeStructuredError(stdout, stderr, jsonOutput, classified)
		}
		return writeInvalidWorkspace(stdout, stderr, jsonOutput, err)
	}
	return writeResult(stdout, stderr, jsonOutput, response, func(p *termui.Printer) {
		switch response.Status {
		case resolverpkg.StatusResolved:
			p.Line(fmt.Sprintf("Resolved %s (%s confidence). Host approval is required before activation.", response.Primary.ID, response.Primary.Confidence))
		case resolverpkg.StatusNeedsContext:
			p.Line(response.Question.Text)
			if len(response.Question.Choices) > 0 {
				p.Line(fmt.Sprintf("Choices: %s", strings.Join(response.Question.Choices, " | ")))
			}
			field := response.Question.Field
			if field == "" {
				field = "scope"
			}
			sampleChoice := "single_step"
			if len(response.Question.Choices) > 0 {
				sampleChoice = response.Question.Choices[0]
			}
			p.Blank()
			if taskKey, inTask := strings.CutPrefix(field, "task."); inTask {
				p.Line(fmt.Sprintf("To answer, set %q in your request JSON, e.g.:\n  {\"task\": {\"description\": \"...\", %q: %q}}", field, taskKey, sampleChoice))
			} else {
				p.Line(fmt.Sprintf("To answer, set %q in your request JSON.", field))
			}
			if response.ResolutionID != "" {
				p.Line(fmt.Sprintf("or provide prior in your request:\n  {\"prior\": {\"resolution_id\": %q, \"answers\": {%q: %q}}}", response.ResolutionID, field, sampleChoice))
			}
		case resolverpkg.StatusAlreadyCovered:
			p.Line(fmt.Sprintf("The active procedure %s already covers this task.", response.CoveredBy))
		case resolverpkg.StatusNoSkill:
			p.Line(fmt.Sprintf("No skill recommended: %s.", response.NoSkill.ReasonCode))
		}
	})
}

// classifyResolveError reports a resolver failure with the code and message
// app.ClassifyError gives it, as the MCP adapter does, naming the CLI command
// that rebuilds the catalog. It returns nil for failures that are genuinely
// workspace problems.
func classifyResolveError(err error) *app.Error {
	classified := *app.ClassifyError(err)
	var refused *resolverpkg.RequestError
	switch {
	case errors.As(err, &refused):
	case classified.Code == app.ErrorSnapshotExpired, classified.Code == app.ErrorUnknownResolution, classified.Code == app.ErrorStaleContext:
	case classified.Code == app.ErrorIndexStale:
		classified.Render.Fix = "Run `skillstation maintain rebuild --workspace <path>`, then retry."
	default:
		return nil
	}
	if classified.Render.Why == "" {
		classified.Render.Why = err.Error()
	}
	return &classified
}

func writeStructuredError(stdout, stderr io.Writer, jsonOutput bool, failure *app.Error) int {
	result := app.ErrorResult(failure)
	if jsonOutput {
		if err := writeJSON(stdout, result); err != nil {
			p := termui.New(stderr)
			p.Line(err.Error())
			return 1
		}
		return 2
	}
	p := termui.New(stderr)
	p.Error(failure.Render.Error, failure.Render.Why, failure.Render.Fix)
	return 2
}

func closeResolutionTelemetry(recorder *telemetry.Recorder) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = recorder.Close(ctx)
}

func resolveFlags(args []string) (workspacePath, requestPath string, jsonOutput bool, err error) {
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--json":
			jsonOutput = true
		case "--workspace", "--request":
			if index+1 == len(args) || strings.HasPrefix(args[index+1], "-") {
				return "", "", jsonOutput, fmt.Errorf("%s requires a value", args[index])
			}
			if args[index] == "--workspace" {
				workspacePath = args[index+1]
			} else {
				requestPath = args[index+1]
			}
			index++
		default:
			return "", "", jsonOutput, fmt.Errorf("unknown argument %q", args[index])
		}
	}
	if requestPath == "" {
		return "", "", jsonOutput, errors.New("--request is required")
	}
	resolved, resErr := resolveWorkspace(workspacePath)
	if resErr != nil {
		return "", "", jsonOutput, resErr
	}
	return resolved, requestPath, jsonOutput, nil
}

func readResolveRequest(path string) (resolverpkg.Request, error) {
	var request resolverpkg.Request
	file, err := os.Open(path)
	if err != nil {
		return request, fmt.Errorf("read request: %w", err)
	}
	defer file.Close()
	limited := io.LimitReader(file, maxResolveRequestBytes+1)
	contents, err := io.ReadAll(limited)
	if err != nil {
		return request, fmt.Errorf("read request: %w", err)
	}
	if len(contents) > maxResolveRequestBytes {
		return request, fmt.Errorf("request exceeds %d bytes", maxResolveRequestBytes)
	}
	if !utf8.Valid(contents) {
		return request, fmt.Errorf("request must be valid UTF-8")
	}
	// The version is checked first: a request written for another version
	// may carry fields this one does not define.
	var versioned struct {
		SchemaVersion *string `json:"schema_version"`
	}
	if json.Unmarshal(contents, &versioned) == nil && versioned.SchemaVersion != nil {
		if err := resolverpkg.CheckSchemaVersion(*versioned.SchemaVersion); err != nil {
			return request, err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, fmt.Errorf("parse request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return request, fmt.Errorf("request contains trailing data")
	}
	if _, err := resolverpkg.NormalizeRequest(request); err != nil {
		return request, err
	}
	return request, nil
}

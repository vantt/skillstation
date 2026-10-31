package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vantt/skillstation/internal/fsutil"
	"github.com/vantt/skillstation/internal/telemetry"
	"github.com/vantt/skillstation/internal/workspace"
)

const (
	// HubCallRetentionDays mirrors telemetry.HubCallRetention for reports.
	HubCallRetentionDays = int(telemetry.HubCallRetention / (24 * time.Hour))
	hubCallRecentLimit   = 200
	hubCallUnknownTool   = "unknown"
	hubCallNoPin         = "none"
)

var hubCallToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@+\-]{0,255}$`)

// HubCall is one request served by the MCP hub. It holds only bounded tokens;
// arguments and results never reach the log.
type HubCall struct {
	Method    string
	Tool      string
	Failed    bool
	ErrorCode string
	Duration  time.Duration
	Client    telemetry.Client
	// SkillID is the skill a call asked for, when it names one.
	SkillID string
	// ResolutionID links a skill_resolve call to its resolution events.
	ResolutionID string
	// Operation and TaskText are what a skill_resolve call asked for. TaskText
	// is stored only while the user has turned task capture on.
	Operation string
	TaskText  string
}

// RecordHubCall asynchronously records one hub.call event.
func RecordHubCall(ctx context.Context, sink TelemetrySink, workspacePath string, call HubCall) {
	if sink == nil {
		return
	}
	status := telemetry.HubCallStatusOK
	if call.Failed {
		status = telemetry.HubCallStatusError
	}
	payload := map[string]any{
		"method":      call.Method,
		"status":      status,
		"duration_ms": call.Duration.Milliseconds(),
	}
	if call.Tool != "" {
		payload["tool"] = hubCallTokenOr(call.Tool, hubCallUnknownTool)
	}
	if call.Failed && call.ErrorCode != "" {
		payload["error_code"] = hubCallTokenOr(call.ErrorCode, hubCallUnknownTool)
	}
	if call.SkillID != "" && hubCallToken.MatchString(call.SkillID) {
		payload["skill_id"] = call.SkillID
	}
	if call.Operation != "" && hubCallToken.MatchString(call.Operation) {
		payload["operation"] = call.Operation
	}
	if text := hubCallTaskText(workspacePath, call.TaskText); text != "" {
		payload["task_text"] = text
	}
	event := curationTelemetryEvent(telemetry.EventHubCall, payload)
	// A call log line is not tied to a catalog or policy revision. Naming none
	// keeps the recorder from reading the catalog on every call.
	event.CatalogSnapshot, event.PolicyRevision = hubCallNoPin, hubCallNoPin
	if hubCallToken.MatchString(call.ResolutionID) {
		event.ResolutionID = call.ResolutionID
	}
	if call.Client.Name != "" {
		event.Client = call.Client
	}
	recordCurationTelemetry(ctx, sink, workspacePath, event)
}

// hubCallTaskText returns the task text to store: empty while capture is off,
// otherwise the text with secrets removed, on one line and cut to a short length.
func hubCallTaskText(workspacePath, text string) string {
	if text == "" || !HubCallSettingsOf(workspacePath).CaptureTaskText {
		return ""
	}
	text = strings.Join(strings.Fields(telemetry.NewRedactor(workspacePath).Redact(text)), " ")
	if utf8.RuneCountInString(text) > telemetry.MaxTextRunes {
		text = string([]rune(text)[:telemetry.MaxTextRunes-1]) + "…"
	}
	if !utf8.ValidString(text) {
		return ""
	}
	return text
}

// HubCallSettings are the local choices about the hub call log. They live in
// the hub's runtime folder, which Git ignores, so they are per machine.
type HubCallSettings struct {
	// CaptureTaskText stores the task text of skill_resolve calls (redacted, cut
	// short, deleted with the rest of the log after HubCallRetentionDays). It is
	// off until the user turns it on.
	CaptureTaskText bool `json:"capture_task_text"`
}

const hubCallSettingsFile = "hub-call-log.json"

func hubCallSettingsPath(workspacePath string) string {
	return filepath.Join(workspacePath, "runtime", "config", hubCallSettingsFile)
}

// HubCallSettingsOf reads the settings of the hub at workspacePath. A missing or
// unreadable file means every choice is off.
func HubCallSettingsOf(workspacePath string) HubCallSettings {
	var settings HubCallSettings
	data, err := os.ReadFile(hubCallSettingsPath(workspacePath))
	if err != nil || json.Unmarshal(data, &settings) != nil {
		return HubCallSettings{}
	}
	return settings
}

// SaveHubCallSettings writes the settings atomically with owner-only access.
func SaveHubCallSettings(path string, settings HubCallSettings) error {
	root, err := workspace.Discover(path)
	if err != nil {
		return err
	}
	target := hubCallSettingsPath(root)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(target, data, 0o600)
}

func hubCallTokenOr(value, fallback string) string {
	if hubCallToken.MatchString(value) {
		return value
	}
	return fallback
}

// HubCallEntry is one logged hub request.
type HubCallEntry struct {
	OccurredAt time.Time `json:"occurred_at"`
	Method     string    `json:"method"`
	Tool       string    `json:"tool,omitempty"`
	Status     string    `json:"status"`
	ErrorCode  string    `json:"error_code,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	Client     string    `json:"client"`
	// SkillID is the skill a skill_get call asked for.
	SkillID    string             `json:"skill_id,omitempty"`
	Operation  string             `json:"operation,omitempty"`
	TaskText   string             `json:"task_text,omitempty"`
	Resolution *HubCallResolution `json:"resolution,omitempty"`
}

// HubCallResolution is what became of a skill_resolve call, taken from the
// resolution, load and feedback events that carry the same resolution id.
type HubCallResolution struct {
	ID          string   `json:"id"`
	Status      string   `json:"status,omitempty"`
	Recommended []string `json:"recommended,omitempty"`
	Loaded      []string `json:"loaded,omitempty"`
	Used        bool     `json:"used"`
	// Outcome is the task outcome the agent reported: completed or failed.
	Outcome string `json:"outcome,omitempty"`
}

// HubCallCount is a call and failure count for one tool or one day.
type HubCallCount struct {
	Method string `json:"method,omitempty"`
	Tool   string `json:"tool,omitempty"`
	Day    string `json:"day,omitempty"`
	Client string `json:"client,omitempty"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

// HubCallLog summarizes the retained hub calls.
type HubCallLog struct {
	RetentionDays int `json:"retention_days"`
	// CaptureTaskText says whether task text is being stored now.
	CaptureTaskText bool           `json:"capture_task_text"`
	Since           time.Time      `json:"since"`
	Total           int64          `json:"total"`
	Succeeded       int64          `json:"succeeded"`
	Failed          int64          `json:"failed"`
	ByTool          []HubCallCount `json:"by_tool"`
	ByDay           []HubCallCount `json:"by_day"`
	ByClient        []HubCallCount `json:"by_client"`
	Recent          []HubCallEntry `json:"recent"`
}

// HubCallService reads the hub call log.
type HubCallService struct {
	Telemetry TelemetryService
	Now       func() time.Time
}

// Log returns the hub calls of the last HubCallRetentionDays days, newest first.
func (service HubCallService) Log(ctx context.Context, path string) (log HubCallLog, resultErr error) {
	root, err := workspace.Discover(path)
	if err != nil {
		return HubCallLog{}, err
	}
	now := time.Now()
	if service.Now != nil {
		now = service.Now()
	}
	now = now.UTC()
	since := now.Add(-telemetry.HubCallRetention)

	recorder, err := service.Telemetry.Open(root)
	if err != nil {
		return HubCallLog{}, err
	}
	defer closeTelemetryRecorder(recorder, &resultErr)
	// Only hub calls of the exact window are read; their resolutions are read
	// afterwards for the calls on screen alone.
	calls, err := recorder.FilteredRawEvents(ctx, since.Format(time.RFC3339Nano), now.Format("2006-01-02"), telemetry.RawEventFilter{Kinds: []string{telemetry.EventHubCall}})
	if err != nil {
		return HubCallLog{}, storeFailure(err)
	}

	log = HubCallLog{RetentionDays: HubCallRetentionDays, CaptureTaskText: HubCallSettingsOf(root).CaptureTaskText, Since: since, ByTool: []HubCallCount{}, ByDay: []HubCallCount{}, ByClient: []HubCallCount{}, Recent: []HubCallEntry{}}
	tools := map[[2]string]*HubCallCount{}
	days := map[string]*HubCallCount{}
	clients := map[string]*HubCallCount{}
	for _, event := range calls {
		if event.Type != telemetry.EventHubCall || event.OccurredAt.Before(since) {
			continue
		}
		entry := hubCallEntry(event)
		if event.ResolutionID != "" {
			entry.Resolution = &HubCallResolution{ID: event.ResolutionID}
		}
		failed := entry.Status == telemetry.HubCallStatusError
		log.Total++
		if failed {
			log.Failed++
		} else {
			log.Succeeded++
		}
		toolKey := [2]string{entry.Method, entry.Tool}
		if tools[toolKey] == nil {
			tools[toolKey] = &HubCallCount{Method: entry.Method, Tool: entry.Tool}
		}
		day := entry.OccurredAt.Format("2006-01-02")
		if days[day] == nil {
			days[day] = &HubCallCount{Day: day}
		}
		if clients[entry.Client] == nil {
			clients[entry.Client] = &HubCallCount{Client: entry.Client}
		}
		for _, count := range []*HubCallCount{tools[toolKey], days[day], clients[entry.Client]} {
			count.Calls++
			if failed {
				count.Errors++
			}
		}
		log.Recent = append(log.Recent, entry)
	}
	for _, count := range tools {
		log.ByTool = append(log.ByTool, *count)
	}
	sort.Slice(log.ByTool, func(i, j int) bool {
		a, b := log.ByTool[i], log.ByTool[j]
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Method+a.Tool < b.Method+b.Tool
	})
	for _, count := range days {
		log.ByDay = append(log.ByDay, *count)
	}
	for _, count := range clients {
		log.ByClient = append(log.ByClient, *count)
	}
	sort.Slice(log.ByClient, func(i, j int) bool {
		a, b := log.ByClient[i], log.ByClient[j]
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Client < b.Client
	})
	sort.Slice(log.ByDay, func(i, j int) bool { return log.ByDay[i].Day > log.ByDay[j].Day })
	// RawEvents is oldest first; the screen shows the newest first.
	sort.SliceStable(log.Recent, func(i, j int) bool { return log.Recent[i].OccurredAt.After(log.Recent[j].OccurredAt) })
	if len(log.Recent) > hubCallRecentLimit {
		log.Recent = log.Recent[:hubCallRecentLimit]
	}
	if err := attachHubCallResolutions(ctx, recorder, log.Recent, since.Format("2006-01-02"), now.Format("2006-01-02")); err != nil {
		return HubCallLog{}, storeFailure(err)
	}
	return log, nil
}

// attachHubCallResolutions fills in what became of the resolutions of the
// given calls, reading only the events of those resolutions.
func attachHubCallResolutions(ctx context.Context, recorder *telemetry.Recorder, entries []HubCallEntry, from, to string) error {
	ids := []string{}
	for _, entry := range entries {
		if entry.Resolution != nil && !slices.Contains(ids, entry.Resolution.ID) {
			ids = append(ids, entry.Resolution.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	events, err := recorder.FilteredRawEvents(ctx, from, to, telemetry.RawEventFilter{
		Kinds:         []string{telemetry.EventResolutionCompleted, telemetry.EventSkillLoaded, telemetry.EventSkillUsed, telemetry.EventTaskOutcomeReported},
		ResolutionIDs: ids,
	})
	if err != nil {
		return err
	}
	outcomes := resolutionOutcomes(events)
	for index := range entries {
		if entries[index].Resolution == nil {
			continue
		}
		if outcome, ok := outcomes[entries[index].Resolution.ID]; ok {
			entries[index].Resolution = outcome
		}
	}
	return nil
}

// resolutionOutcomes gathers, per resolution id, the status and recommended
// skills of the resolution, the skills loaded after it, whether one was used and
// the reported task outcome.
func resolutionOutcomes(events []telemetry.Event) map[string]*HubCallResolution {
	byID := map[string]*HubCallResolution{}
	entry := func(id string) *HubCallResolution {
		if byID[id] == nil {
			byID[id] = &HubCallResolution{ID: id}
		}
		return byID[id]
	}
	for _, event := range events {
		if event.ResolutionID == "" {
			continue
		}
		text := func(field string) string { value, _ := event.Payload[field].(string); return value }
		switch event.Type {
		case telemetry.EventResolutionCompleted:
			resolution := entry(event.ResolutionID)
			resolution.Status = text("status")
			resolution.Recommended = payloadTokens(event.Payload["recommended_skill_ids"])
		case telemetry.EventSkillLoaded:
			if skillID := text("skill_id"); skillID != "" && text("status") == "" {
				resolution := entry(event.ResolutionID)
				if !slices.Contains(resolution.Loaded, skillID) {
					resolution.Loaded = append(resolution.Loaded, skillID)
				}
			}
		case telemetry.EventSkillUsed:
			entry(event.ResolutionID).Used = true
		case telemetry.EventTaskOutcomeReported:
			entry(event.ResolutionID).Outcome = text("status")
		}
	}
	return byID
}

func payloadTokens(value any) []string {
	switch items := value.(type) {
	case []string:
		return items
	case []any:
		tokens := make([]string, 0, len(items))
		for _, item := range items {
			if token, ok := item.(string); ok {
				tokens = append(tokens, token)
			}
		}
		return tokens
	}
	return nil
}

func hubCallEntry(event telemetry.Event) HubCallEntry {
	text := func(field string) string { value, _ := event.Payload[field].(string); return value }
	duration, _ := event.Payload["duration_ms"].(float64)
	return HubCallEntry{
		OccurredAt: event.OccurredAt.UTC(),
		Method:     text("method"),
		Tool:       text("tool"),
		Status:     text("status"),
		ErrorCode:  text("error_code"),
		DurationMS: int64(duration),
		Client:     event.Client.Name,
		SkillID:    text("skill_id"),
		Operation:  text("operation"),
		TaskText:   text("task_text"),
	}
}

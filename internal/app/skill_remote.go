package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/digest"
	"github.com/vantt/skillstation/internal/skill"
	"github.com/vantt/skillstation/internal/skillmeta"
	"github.com/vantt/skillstation/internal/skillruntime"
)

const (
	remoteSubmissionsPath = "/mcp/submissions"
	remoteBundlePath      = "/mcp/bundle"
	remoteRequestTimeout  = 2 * time.Minute
)

// RemoteHub is the hub a project is connected to on another machine.
type RemoteHub struct {
	URL       string
	TokenFile string
	// MaySendToken is true when this machine's own settings, not only the
	// project's files, name the hub (or it is on this machine): a hub named
	// only by a cloned project could be anyone's.
	MaySendToken bool
}

// TrustCommand is the connect command that makes this machine's own settings
// name the hub, so the hub token may go there: the user-scope entry, with the
// token file the project's entry already names. It must carry --token-file:
// connect writes the entry it is given, and one without it leaves the proxy
// with no token.
func (hub RemoteHub) TrustCommand() string {
	return reconnectRemoteCommand(hub.URL, hub.TokenFile, true)
}

// DetectRemoteHub finds the hub on another machine the current directory is
// connected to. A hub on this machine always wins, so it reports none then.
func DetectRemoteHub(report RoleReport) (RemoteHub, bool) {
	for _, entry := range report.EffectiveEntries() {
		if !entry.Remote() && IsHubRoot(entry.Workspace) {
			return RemoteHub{}, false
		}
	}
	if report.Hub != nil {
		return RemoteHub{}, false
	}
	for _, entry := range report.EffectiveEntries() {
		if entry.Remote() {
			return RemoteHub{URL: entry.HubURL, TokenFile: entry.TokenFile, MaySendToken: tokenMaySend(entry.HubURL, FindRemoteEntries(report))}, true
		}
	}
	return RemoteHub{}, false
}

// RemoteCommitFile is one file of a folder about to be sent.
type RemoteCommitFile struct {
	Path string `json:"path"`
	// Change is "A", "M" or "D" against the checkout the folder came from;
	// it is empty when no checkout is known.
	Change string `json:"change,omitempty"`
	Size   int64  `json:"size"`
}

// RemoteCommitPreview is what would be sent to a remote hub, before anything is.
type RemoteCommitPreview struct {
	Result
	SkillID string `json:"skill_id"`
	Hub     string `json:"hub"`
	Folder  string `json:"folder"`
	// CheckedOut says the folder carries a checkout marker.
	CheckedOut      bool               `json:"checked_out"`
	BaseVersion     string             `json:"base_version,omitempty"`
	Files           []RemoteCommitFile `json:"files"`
	Excluded        []ExcludedFile     `json:"excluded"`
	TotalBytes      int64              `json:"total_bytes"`
	NothingToCommit bool               `json:"nothing_to_commit"`
	// MaySendToken says whether sending without a person at the prompt is allowed.
	MaySendToken bool `json:"may_send_token"`
	// Note is what the reviewer will read next to the files, as it will be sent.
	Note string `json:"note,omitempty"`
	tree map[string]treeFile
	// baseBundle is the digest of the hub bundle the folder was checked out from.
	baseBundle string
}

func remotePreviewError(err *Error) RemoteCommitPreview {
	return RemoteCommitPreview{Result: ErrorResult(err), Files: []RemoteCommitFile{}, Excluded: []ExcludedFile{}}
}

// RemoteCommitService sends a project's skill folder to a hub on another
// machine, which queues it for a person there to review.
type RemoteCommitService struct {
	// Client sends the request; nil uses a client that follows no redirect.
	Client *http.Client
	// Label names this machine to the hub; empty uses the host name.
	Label string
	// Note is the sender's words to the person who reviews the submission.
	Note string
}

// Preview reads the folder with the rules a hub commit uses and says what
// would be sent. It never contacts the hub.
func (service RemoteCommitService) Preview(ctx context.Context, hub RemoteHub, target, projectDir string) (RemoteCommitPreview, error) {
	resolved, refusal := resolveCommitTarget(ctx, target, projectDir)
	if refusal != nil {
		return remotePreviewError(refusal), nil
	}
	tree, excluded, refusal, err := readProjectFolder(resolved.dir)
	if err != nil {
		return RemoteCommitPreview{}, err
	}
	if refusal != nil {
		return remotePreviewError(refusal), nil
	}
	entryRefusal, entryWarnings := checkSkillEntry(tree)
	if entryRefusal != nil {
		return remotePreviewError(entryRefusal), nil
	}
	if refusal := checkSkillSettings(tree); refusal != nil {
		return remotePreviewError(refusal), nil
	}
	preview := RemoteCommitPreview{SkillID: resolved.id, Hub: hub.URL, Folder: resolved.dir, Excluded: excluded, tree: tree, MaySendToken: hub.MaySendToken, Files: []RemoteCommitFile{}, Note: CleanSubmissionNote(service.Note)}
	if preview.Excluded == nil {
		preview.Excluded = []ExcludedFile{}
	}
	sort.Slice(preview.Excluded, func(i, j int) bool { return preview.Excluded[i].Path < preview.Excluded[j].Path })
	var baseFiles map[string]string
	if resolved.marker != nil {
		preview.CheckedOut, preview.BaseVersion, baseFiles = true, resolved.marker.BaseVersion, resolved.marker.BaseFiles
		preview.baseBundle = resolved.marker.BundleDigest
		if resolved.marker.Hub != hub.URL {
			preview.Warnings = append(preview.Warnings, Warning{Code: "other_hub", Summary: fmt.Sprintf("This folder was checked out from %s; you are sending it to %s.", resolved.marker.Hub, hub.URL)})
		}
	}
	preview.Files, preview.TotalBytes = describeRemoteFiles(tree, baseFiles)
	if warning, replaces := routingReplacementWarning(resolved.id, tree, baseFiles, resolved.marker != nil); replaces {
		preview.Warnings = append(preview.Warnings, warning)
	}
	preview.NothingToCommit = baseFiles != nil && len(preview.Files) == 0
	warnings := append(preview.Warnings, entryWarnings...)
	warnings = append(warnings, folderRuntimeWarnings(resolved.id, tree, folderSettings(tree))...)
	preview.Result = NewResult(StatusActionRequired, remoteCommitSummary(preview))
	if preview.NothingToCommit {
		preview.Result = NewResult(StatusOK, fmt.Sprintf("Nothing to send: %s matches the checkout.", preview.SkillID))
	}
	preview.Warnings = warnings
	return preview, nil
}

// routingReplacementWarning warns about a skill.yaml written in a checkout of
// a hub skill. The hub's bundle carries no skill.yaml, so a checkout has none,
// and the hub reads the one a submission carries as the whole of the skill's
// routing: a file naming one new trigger drops the examples, the not-for list
// and the other triggers the hub holds.
func routingReplacementWarning(id string, tree map[string]treeFile, baseFiles map[string]string, checkedOut bool) (Warning, bool) {
	file, has := tree["skill.yaml"]
	if _, hadBefore := baseFiles["skill.yaml"]; !checkedOut || !has || hadBefore {
		return Warning{}, false
	}
	routing := skill.RoutingOfFiles(map[string][]byte{"skill.yaml": file.Data})
	return Warning{Code: "routing_replaces", Summary: fmt.Sprintf(
		"The hub keeps the routing this skill.yaml lists (%s, %s, %s) and drops whatever else it holds for %s. To change part of the routing, run `skillstation skill edit %s --trigger ...`: it starts from what the hub holds.",
		pluralize(len(routing.Triggers), "trigger"), pluralize(len(routing.Examples), "example"), pluralize(len(routing.NotFor), "not-for item"), id, id)}, true
}

func remoteCommitSummary(preview RemoteCommitPreview) string {
	if preview.CheckedOut {
		return fmt.Sprintf("%s: %s changed since checkout; this sends it to the hub at %s for review.", preview.SkillID, pluralize(len(preview.Files), "file"), preview.Hub)
	}
	return fmt.Sprintf("%s was not checked out from the hub at %s: this submits it for review as a new skill, or as a replacement when the hub has one.", preview.SkillID, preview.Hub)
}

// describeRemoteFiles lists the files to send. With the digests of the
// checkout it lists only what changed; without them, every file.
func describeRemoteFiles(tree map[string]treeFile, base map[string]string) ([]RemoteCommitFile, int64) {
	paths := make([]string, 0, len(tree))
	var total int64
	for rel, file := range tree {
		paths = append(paths, rel)
		total += int64(len(file.Data))
	}
	for rel := range base {
		if _, kept := tree[rel]; !kept {
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	files := []RemoteCommitFile{}
	for _, rel := range paths {
		file, present := tree[rel]
		entry := RemoteCommitFile{Path: rel, Size: int64(len(file.Data))}
		if base != nil {
			before, had := base[rel]
			switch {
			case !present:
				entry.Change = "D"
			case !had:
				entry.Change = "A"
			case before != digest.Bytes(file.Data):
				entry.Change = "M"
			default:
				continue
			}
		}
		files = append(files, entry)
	}
	return files, total
}

// RemoteCommitResult reports what the hub answered to a submission.
type RemoteCommitResult struct {
	Result
	SkillID      string `json:"skill_id"`
	Hub          string `json:"hub"`
	SubmissionID string `json:"submission_id,omitempty"`
	// Status is the hub's word for it: "waiting".
	Status string `json:"status,omitempty"`
	// NoteKept says the hub stored the note. A hub from before notes accepts
	// the submission and drops the note, which the result then warns about.
	NoteKept bool `json:"note_kept,omitempty"`
	// StatusCommand is how to ask the hub later what became of the submission.
	StatusCommand string `json:"status_command,omitempty"`
}

// Send uploads the previewed folder as a bundle. The hub only queues it; the
// marker of the folder is left as it is, because nothing is released yet.
func (service RemoteCommitService) Send(ctx context.Context, hub RemoteHub, preview RemoteCommitPreview) (RemoteCommitResult, error) {
	failed := func(err *Error) (RemoteCommitResult, error) {
		return RemoteCommitResult{Result: ErrorResult(err), SkillID: preview.SkillID, Hub: hub.URL}, nil
	}
	if preview.Error != nil || preview.tree == nil {
		return failed(NewInvalidRequestError("there is no previewed folder to send", "Run `skillstation skill commit` again."))
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return failed(problem)
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return failed(NewInvalidRequestError(token.Problem.Detail, token.Problem.Fix))
	}
	excluded := make([]skillruntime.BundleExcluded, 0, len(preview.Excluded))
	for _, file := range preview.Excluded {
		excluded = append(excluded, skillruntime.BundleExcluded{Path: file.Path, Reason: file.Reason})
	}
	label := service.Label
	if label == "" {
		hostname, _ := os.Hostname()
		label = machineLabel(hostname)
	}
	archive, err := BuildSubmissionBundle(preview.SkillID, preview.tree, skillruntime.BundleSubmission{BaseVersion: preview.BaseVersion, ClientLabel: label, Excluded: excluded, Note: preview.Note})
	if err != nil {
		return RemoteCommitResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, hubURL+remoteSubmissionsPath, bytes.NewReader(archive))
	if err != nil {
		return RemoteCommitResult{}, err
	}
	request.Header.Set("Content-Type", skillruntime.BundleMediaType)
	request.Header.Set("Authorization", "Bearer "+token.Value)
	response, err := service.client().Do(request)
	if err != nil {
		return failed(NewInvalidRequestError(fmt.Sprintf("nothing answered at %s: %s", hubURL, probeFailureReason(err)), "Check the connection with `skillstation doctor`."))
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusAccepted {
		return failed(remoteRefusal(hubURL, response.StatusCode, body, "submit"))
	}
	var reply struct {
		SubmissionID string `json:"submission_id"`
		Status       string `json:"status"`
		NoteKept     bool   `json:"note_kept"`
	}
	if json.Unmarshal(body, &reply) != nil || !submissionIDPattern.MatchString(reply.SubmissionID) {
		return failed(NewInvalidRequestError(fmt.Sprintf("the hub at %s answered, but not as a skillstation that takes submissions", hubURL), "Check that the hub runs the same skillstation version as this machine."))
	}
	result := RemoteCommitResult{
		Result:  NewResult(StatusApplied, fmt.Sprintf("Sent %s as %s.", preview.SkillID, reply.SubmissionID)),
		SkillID: preview.SkillID, Hub: hubURL, SubmissionID: reply.SubmissionID, Status: reply.Status, NoteKept: reply.NoteKept,
		StatusCommand: "skillstation skill submission " + reply.SubmissionID,
	}
	if preview.Note != "" && !reply.NoteKept {
		result.Warnings = append(result.Warnings, Warning{Code: "note_not_kept", Summary: "The hub did not keep your note: it runs a Skillstation from before notes. The skill was sent without it; tell the person who reviews it, and update the hub to see notes."})
	}
	return result, nil
}

func (service RemoteCommitService) client() *http.Client {
	return remoteHTTPClient(service.Client)
}

func remoteHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{
		Timeout: remoteRequestTimeout,
		// A redirect could send the token or the files to another host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// remoteRefusal explains a hub's non-success answer in the words of the
// person's task.
func remoteRefusal(hubURL string, status int, body []byte, verb string) *Error {
	var envelope struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &envelope)
	message := strings.TrimSpace(envelope.Message)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if envelope.Code == "content_review_required" {
			return NewInvalidRequestError(message, "Ask the person who runs the hub to review it.")
		}
		return NewInvalidRequestError(fmt.Sprintf("the hub at %s refused the token", hubURL), "Copy the current token file from the hub machine again; `skillstation doctor` shows which one this project uses.")
	case http.StatusBadRequest:
		if verb == "lint" && message != "" {
			return NewInvalidRequestError(message, "Fix SKILL.md and skill.yaml in the folder, then lint again.")
		}
	case http.StatusNotFound:
		if verb == "submit" {
			return NewInvalidRequestError(fmt.Sprintf("the hub at %s does not take submissions", hubURL), "Update skillstation on the hub machine, then restart `skillstation serve` there.")
		}
		if verb == "lint" {
			return NewInvalidRequestError(fmt.Sprintf("the hub at %s does not check routing", hubURL), "Update skillstation on the hub machine, then restart `skillstation serve` there.")
		}
		return NewInvalidRequestError(message, "Only active skills can be checked out from a remote hub; ask the person who runs the hub to activate it.")
	case http.StatusRequestEntityTooLarge:
		return NewInvalidRequestError(firstNonEmpty(message, "the skill is larger than a skill may be"), "Send the skill's own folder; build output and secrets are left out automatically.")
	case http.StatusTooManyRequests:
		return NewInvalidRequestError(firstNonEmpty(message, "the hub is busy"), "Wait a moment and try again, or ask the person who runs the hub to review what is waiting.")
	}
	return NewInvalidRequestError(fmt.Sprintf("the hub at %s answered HTTP %d: %s", hubURL, status, firstNonEmpty(message, "no details")), "Look at the output of `skillstation serve` on the hub machine.")
}

// RemoteCheckoutService copies an active skill of a hub on another machine into
// the project, through the same bundle the agents' proxy fetches.
type RemoteCheckoutService struct {
	Client *http.Client
	Clock  Clock
}

// Checkout fetches the skill's bundle and writes it to
// <project>/.agents/skills/<id>/ with a marker that names the hub, and points
// Claude Code at it as a local checkout does.
func (service RemoteCheckoutService) Checkout(ctx context.Context, hub RemoteHub, input SkillCheckoutInput) (SkillCheckoutResult, error) {
	if service.Clock == nil {
		service.Clock = SystemClock{}
	}
	id := strings.TrimSpace(input.SkillID)
	if !skillmeta.ValidSkillID(id) {
		return checkoutError(NewInvalidRequestError(fmt.Sprintf("invalid skill id %q", id), "Use the lowercase id shown by `skillstation skill list` on the hub machine.")), nil
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return checkoutError(problem), nil
	}
	projectRoot := projectRootOf(ctx, input.ProjectDir)
	shared, claude := projectSkillFolders(projectRoot, id)
	if refusal := newProjectFolderRefusal(shared, claude, id); refusal != nil {
		return checkoutError(refusal), nil
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return checkoutError(NewInvalidRequestError(token.Problem.Detail, token.Problem.Fix)), nil
	}
	tree, bundleDigest, refusal, err := service.fetch(ctx, hubURL, token.Value, id)
	if err != nil {
		return SkillCheckoutResult{}, err
	}
	if refusal != nil {
		return checkoutError(refusal), nil
	}
	baseFiles := make(map[string]string, len(tree))
	for rel, file := range tree {
		baseFiles[rel] = digest.Bytes(file.Data)
	}
	marker := CheckoutMarker{
		SkillID: id, Hub: hubURL, BaseVersion: skillBaseVersion(tree), BundleDigest: bundleDigest, BaseFiles: baseFiles,
		CheckedOutAt: service.Clock.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	result, err := finishCheckout(projectRoot, shared, tree, marker, "active, "+pluralize(len(tree), "file"), hubURL)
	if err != nil || result.Error != nil {
		return result, err
	}
	result.State = "active"
	return result, nil
}

// fetch downloads the bundle and reads its files, accepting it only when its
// file list hashes to the version the hub named.
func (service RemoteCheckoutService) fetch(ctx context.Context, hubURL, token, id string) (map[string]treeFile, string, *Error, error) {
	tree, bundleDigest, _, refusal, err := service.fetchWithDetail(ctx, hubURL, token, id)
	return tree, bundleDigest, refusal, err
}

// fetchWithDetail is fetch that also returns the hub's own description of the
// skill (its routing, labels and the like), which the bundle carries beside its
// files and which a project folder may not hold.
func (service RemoteCheckoutService) fetchWithDetail(ctx context.Context, hubURL, token, id string) (map[string]treeFile, string, json.RawMessage, *Error, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, hubURL+remoteBundlePath+"?skill="+url.QueryEscape(id), nil)
	if err != nil {
		return nil, "", nil, nil, err
	}
	request.Header.Set("Accept", skillruntime.BundleMediaType)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := remoteHTTPClient(service.Client).Do(request)
	if err != nil {
		return nil, "", nil, NewInvalidRequestError(fmt.Sprintf("nothing answered at %s: %s", hubURL, probeFailureReason(err)), "Check the connection with `skillstation doctor`."), nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return nil, "", nil, remoteRefusal(hubURL, response.StatusCode, body, "checkout"), nil
	}
	digest := strings.Trim(strings.TrimPrefix(response.Header.Get("ETag"), "W/"), `"`)
	if !skillruntime.ValidBundleDigest(digest) {
		return nil, "", nil, NewInvalidRequestError("the hub sent a skill bundle without a valid digest", "Check that the hub runs the same skillstation version as this machine."), nil
	}
	staging, err := os.MkdirTemp("", "skillstation-checkout-")
	if err != nil {
		return nil, "", nil, nil, err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	metadata, err := skillruntime.ExtractBundle(response.Body, staging)
	if err != nil {
		return nil, "", nil, NewInvalidRequestError("the hub sent a skill bundle that cannot be read: "+err.Error(), "Try again; if it keeps failing, run `skillstation doctor`."), nil
	}
	if metadata.SkillID != id || metadata.ManifestVersion != "sha256:"+digest {
		return nil, "", nil, NewInvalidRequestError("the hub sent a bundle that is not the one asked for", "Try again; if it keeps failing, run `skillstation doctor`."), nil
	}
	tree := make(map[string]treeFile, len(metadata.Files))
	for _, file := range metadata.Files {
		if !safeTreePath(file.Path) {
			return nil, "", nil, NewInvalidRequestError(fmt.Sprintf("the bundle lists %q, which leaves the skill folder", file.Path), "Ask the person who runs the hub to check the skill."), nil
		}
		data, err := os.ReadFile(filepath.Join(staging, filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, "", nil, nil, err
		}
		tree[file.Path] = treeFile{Data: data}
	}
	return tree, digest, metadata.Detail, nil, nil
}

// probeFailureReason is a short, plain reason a request did not get an answer.
func probeFailureReason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "no answer in time"
	case errors.Is(err, context.Canceled):
		return "the request was cancelled"
	}
	return networkReason(err)
}

// localNetworkSuffixes are the host name endings a home or office network adds
// (macOS adds .local). They say nothing about which machine sent a skill, so
// the label a person reads leaves them out.
var localNetworkSuffixes = []string{".localdomain", ".local", ".lan", ".home"}

// machineLabel turns a host name into the label a hub shows for this machine.
func machineLabel(hostname string) string {
	label := strings.TrimSpace(hostname)
	for _, suffix := range localNetworkSuffixes {
		cut := len(label) - len(suffix)
		if cut > 0 && strings.EqualFold(label[cut:], suffix) {
			return label[:cut]
		}
	}
	return label
}

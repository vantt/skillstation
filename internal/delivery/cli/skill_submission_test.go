package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vantt/skillstation/internal/app"
	"github.com/vantt/skillstation/internal/hubtoken"
)

// These tests run a real listener and work from a project directory, so none
// of them runs in parallel.

// remoteProject connects a new project to a hub served on a random loopback
// port, the way a person on machine A does, and returns what the tests need.
type remoteProject struct {
	hub      nextStepHub
	hubURL   string
	dir      string
	tokenKey string
	machine  string
}

func newRemoteProject(t *testing.T) remoteProject {
	t.Helper()
	isolateClient(t)
	hub := newNextStepHub(t)
	hubURL, _ := startStoppableListener(t, hub.root)
	tokenFile, token := copyHubToken(t, hub.root)
	project := projectInDir(t)
	if code, stdout, stderr := runConnectCLI(t, project, "--hub", hubURL, "--token-file", tokenFile, "--host", "claude"); code != 0 {
		t.Fatalf("connect exit = %d\n%s%s", code, stdout, stderr)
	}
	hostname, _ := os.Hostname()
	return remoteProject{hub: hub, hubURL: hubURL, dir: project, tokenKey: token, machine: submittedFrom(hostname)}
}

// oneLine joins wrapped terminal text, so a long machine label that the text
// output wraps onto the next line still matches as one phrase.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// submittedFrom is the label a hub shows for a submission sent from a machine
// with this host name. The client drops a local network suffix (macOS adds
// .local); the hub keeps at most 64 characters, as whole leading dot-separated
// parts, and cuts a first part that alone is longer with an ellipsis.
func submittedFrom(hostname string) string {
	label := strings.TrimSpace(hostname)
	for _, suffix := range []string{".localdomain", ".local", ".lan", ".home"} {
		if cut := len(label) - len(suffix); cut > 0 && strings.EqualFold(label[cut:], suffix) {
			label = label[:cut]
			break
		}
	}
	if label == "" {
		return "unknown machine"
	}
	if len([]rune(label)) <= 64 {
		return label
	}
	parts := strings.Split(strings.TrimLeft(label, "."), ".")
	if first := []rune(parts[0]); len(first) > 64 {
		return string(first[:63]) + "…"
	}
	kept := parts[0]
	for _, part := range parts[1:] {
		if len([]rune(kept))+1+len([]rune(part)) > 64 {
			break
		}
		kept += "." + part
	}
	if kept == "" {
		return "unknown machine"
	}
	return kept
}

func (project remoteProject) bundleStatus(t *testing.T, id string) int {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, project.hubURL+"/mcp/bundle?skill="+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+project.tokenKey)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	return response.StatusCode
}

func submissionIDIn(t *testing.T, text string) string {
	t.Helper()
	index := strings.Index(text, "SUB-")
	if index < 0 || len(text) < index+30 {
		t.Fatalf("no submission id in:\n%s", text)
	}
	return text[index : index+30]
}

func TestRemoteProjectSendsASkillAndAPersonOnTheHubReviewsIt(t *testing.T) {
	project := newRemoteProject(t)
	if code, stdout, stderr := runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release",
		"--trigger", "cut a team release", "--not-for", "deploying to production", "--example", "tag and publish this week's build for the team", "--example", "we need the team release notes written up", "--example", "bump versions across the team repos"); code != 0 {
		t.Fatalf("create exit = %d\n%s%s", code, stdout, stderr)
	}
	folder := filepath.Join(project.dir, ".agents", "skills", "team-release")
	skillMD := "---\nname: team-release\ndescription: Cut a team release. Use when someone asks to cut a team release. Not for deploying to production.\n---\n\n# Team release\n\n" +
		"## When to use\n\n- someone asks to cut a team release\n\n## When not to use\n\n- deploying to production\n\n## Examples\n\n" +
		"1. tag and publish this week's build for the team\n2. we need the team release notes written up\n3. bump versions across the team repos\n\n" +
		"## Steps\n\nRun scripts/bump.sh to raise every version, tag the commit and publish the release notes.\n"
	if err := os.WriteFile(filepath.Join(folder, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(folder, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "scripts", "bump.sh"), []byte("#!/bin/sh\necho bump\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, ".env"), []byte("TOKEN=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	head, snapshot := hubHead(t, project.hub.root), gitOutput(t, project.hub.root, "ls-files")

	// Without --yes and without a terminal: a preview, and nothing is sent.
	code, stdout, stderr := runCLI(t, "skill", "commit", "team-release")
	if code != 0 || !strings.Contains(stdout, "Nothing sent yet.") || !strings.Contains(stdout, "Left out: .env (looks like a secret)") || !strings.Contains(stdout, "scripts/bump.sh") {
		t.Fatalf("preview: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if queued, _ := app.ListSubmissions(project.hub.root); len(queued) != 0 {
		t.Fatalf("a preview queued %+v", queued)
	}

	code, stdout, stderr = runCLI(t, "skill", "commit", "team-release", "--yes")
	if code != 0 || !strings.Contains(stdout, "Sent as SUB-") || !strings.Contains(flatText(stdout), "A person on the hub machine reviews it; agents get it only after approval.") {
		t.Fatalf("send: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	sub := submissionIDIn(t, stdout)
	if marker, found, _ := app.ReadCheckoutMarker(folder); found {
		t.Fatalf("a submission moved the marker: %+v", marker)
	}

	// The hub is as it was, and serves nothing of the skill.
	if hubHead(t, project.hub.root) != head || gitOutput(t, project.hub.root, "ls-files") != snapshot || !hubIsClean(t, project.hub.root) {
		t.Fatalf("the submission changed the hub")
	}
	if got := project.bundleStatus(t, "team-release"); got != http.StatusNotFound {
		t.Fatalf("bundle status = %d, want 404", got)
	}
	if listed := project.hub.mustRun("skill", "list", "--json"); strings.Contains(listed, "team-release") {
		t.Fatalf("the hub lists the submitted skill:\n%s", listed)
	}
	statusJSON := project.hub.mustRun("status", "--json")
	_, thisFile, _, _ := runtime.Caller(0)
	validateJSONDocument(t, filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "schemas", "curation-home-v1.schema.json"), []byte(statusJSON))
	var parsed struct {
		Actions []struct{ Kind, ID, Command, Summary string }
	}
	if err := json.Unmarshal([]byte(statusJSON), &parsed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, action := range parsed.Actions {
		if action.Kind == "submission_waiting" {
			found = action.ID == sub && action.Command == "skillstation skill review "+sub && strings.Contains(action.Summary, "1 submitted skill waits for review: team-release from "+project.machine)
		}
	}
	if !found {
		t.Fatalf("status lists no submission_waiting action for %s:\n%s", sub, statusJSON)
	}
	if text := project.hub.mustRun("status"); !strings.Contains(oneLine(text), "1 submitted skill waits for review: team-release from "+project.machine) || !strings.Contains(text, "Review: skillstation skill review "+sub) {
		t.Fatalf("status text:\n%s", text)
	}

	// Without a terminal an agent can only look: nothing is applied.
	code, stdout, stderr = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root)
	if code != 0 || !strings.Contains(oneLine(stdout), "team-release (submitted from "+project.machine+"): a new skill.") || !strings.Contains(stdout, "Submitted content is untrusted until you approve it.") || !strings.Contains(stdout, "Only a person approves") {
		t.Fatalf("review without a terminal: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "--yes") {
		t.Fatalf("an agent was shown a way to approve:\n%s", stdout)
	}
	if code, _, _ = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root, "--yes"); code == 0 {
		t.Fatalf("review --yes was accepted")
	}
	if hubHead(t, project.hub.root) != head {
		t.Fatalf("review without a terminal changed the hub")
	}

	// A person at a terminal: apply, approve, activate.
	atTerminal(t, "y\ny\ny\n")
	code, stdout, stderr = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root)
	if code != 0 {
		t.Fatalf("review at a terminal: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{"Add team-release as a draft? [y/N] ", "Approve this content for agents? [y/N] ", "Activate team-release now? [y/N] ", "Committed "} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("review output lacks %q:\n%s", want, stdout)
		}
	}
	if gitOutput(t, project.hub.root, "rev-list", "--count", head+"..HEAD") != "1" {
		t.Fatalf("expected exactly one hub commit:\n%s", stdout)
	}
	if subject := gitOutput(t, project.hub.root, "log", "-1", "--format=%s"); subject != "Add team-release (submitted from "+project.machine+"), approve content, activate" {
		t.Fatalf("subject = %q", subject)
	}
	if !hubIsClean(t, project.hub.root) {
		t.Fatalf("the review left files behind:\n%s", gitOutput(t, project.hub.root, "status", "--porcelain=v1"))
	}
	if queued, _ := app.ListSubmissions(project.hub.root); len(queued) != 0 {
		t.Fatalf("an applied submission is still queued: %+v", queued)
	}
	if got := project.bundleStatus(t, "team-release"); got != http.StatusOK {
		t.Fatalf("an activated skill is not served: bundle status = %d", got)
	}
	if strings.Contains(gitOutput(t, project.hub.root, "ls-files"), ".env") {
		t.Fatalf(".env reached the hub")
	}
}

func TestRemoteCheckoutEditAndCommitGivesASubmissionWithADiffAgainstTheBase(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")
	head := hubHead(t, project.hub.root)

	code, stdout, stderr := runCLI(t, "skill", "checkout", "review-checklist")
	if code != 0 || !strings.Contains(stdout, "Copied review-checklist (active, 1 file) from "+project.hubURL) {
		t.Fatalf("checkout: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	folder := filepath.Join(project.dir, ".agents", "skills", "review-checklist")
	marker, found, err := app.ReadCheckoutMarker(folder)
	if err != nil || !found || marker.Hub != project.hubURL || marker.BaseVersion == "" || marker.BundleDigest == "" || len(marker.BaseFiles) != 1 {
		t.Fatalf("marker = %+v found=%v err=%v", marker, found, err)
	}
	if hubHead(t, project.hub.root) != head {
		t.Fatalf("a checkout changed the hub")
	}
	if code, stdout, _ = runCLI(t, "skill", "commit", "review-checklist", "--yes"); code != 0 || !strings.Contains(stdout, "Nothing to send") {
		t.Fatalf("commit without edits: exit = %d\n%s", code, stdout)
	}

	appendToFile(t, filepath.Join(folder, "SKILL.md"), "\nOne more line from machine A.\n")
	code, stdout, stderr = runCLI(t, "skill", "commit", "review-checklist", "--yes")
	if code != 0 || !strings.Contains(stdout, "Sent as SUB-") {
		t.Fatalf("send: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	sub := submissionIDIn(t, stdout)
	submission, err := app.ReadSubmission(project.hub.root, sub)
	if err != nil || submission.BaseVersion != marker.BaseVersion {
		t.Fatalf("submission = %+v, %v", submission, err)
	}

	code, stdout, stderr = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root, "--diff")
	if code != 0 || !strings.Contains(oneLine(stdout), "review-checklist (submitted from "+project.machine+"): 1 file changed since checkout.") ||
		!strings.Contains(stdout, "+One more line from machine A.") || strings.Contains(stdout, "needs a decision") {
		t.Fatalf("review --diff: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	// skill review <id> names what waits for that skill.
	if out := project.hub.mustRun("skill", "review", "review-checklist"); !strings.Contains(out, sub) {
		t.Fatalf("skill review does not name the waiting submission:\n%s", out)
	}

	atTerminal(t, "y\ny\n")
	if code, stdout, stderr = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root); code != 0 || !strings.Contains(stdout, "Apply this change? [y/N] ") || !strings.Contains(stdout, "Approve this content for agents? [y/N] ") || strings.Contains(stdout, "Activate") {
		t.Fatalf("review: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if gitOutput(t, project.hub.root, "rev-list", "--count", head+"..HEAD") != "1" {
		t.Fatalf("expected exactly one hub commit")
	}
	if data := readFileText(t, filepath.Join(project.hub.root, "skills", "software", "review-checklist", "SKILL.md")); !strings.Contains(data, "One more line from machine A.") {
		t.Fatalf("the hub SKILL.md lacks the submitted line:\n%s", data)
	}
	// The sender can ask what became of it, and is told how to take it in.
	if _, stdout, _ = runCLI(t, "skill", "submission", sub); !strings.Contains(oneLine(stdout), sub+" (review-checklist) was reviewed and applied on the hub on ") ||
		!strings.Contains(stdout, "skillstation skill pull review-checklist") {
		t.Fatalf("after approval:\n%s", stdout)
	}
}

func TestDiscardDropsASubmissionAndAnAnswerOfNoWritesNothing(t *testing.T) {
	project := newRemoteProject(t)
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")
	_, stdout, _ := runCLI(t, "skill", "commit", "team-release", "--yes")
	sub := submissionIDIn(t, stdout)
	head := hubHead(t, project.hub.root)

	atTerminal(t, "y\nn\n")
	code, stdout, _ := runCLI(t, "skill", "review", sub, "--workspace", project.hub.root)
	if code != 0 || !strings.Contains(stdout, "Nothing written.") || hubHead(t, project.hub.root) != head {
		t.Fatalf("declining the approval: exit = %d\n%s", code, stdout)
	}
	if _, err := app.ReadSubmission(project.hub.root, sub); err != nil {
		t.Fatalf("a declined review dropped the submission: %v", err)
	}
	code, stdout, stderr := runCLI(t, "skill", "review", sub, "--workspace", project.hub.root, "--discard")
	if code != 0 || !strings.Contains(stdout, "Discarded "+sub) {
		t.Fatalf("discard: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if _, err := app.ReadSubmission(project.hub.root, sub); err == nil {
		t.Fatalf("a discarded submission is still queued")
	}
	if code, _, _ = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root, "--discard"); code == 0 {
		t.Fatalf("discarding twice succeeded")
	}
	if hubHead(t, project.hub.root) != head || !hubIsClean(t, project.hub.root) {
		t.Fatalf("discard changed the hub")
	}
}

func TestRemoteCommitLabelNamesTheMachineOnTheHub(t *testing.T) {
	project := newRemoteProject(t)
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")
	for _, label := range []string{"lap\ntop", strings.Repeat("a", 65), "  "} {
		if code, _, stderr := runCLI(t, "skill", "commit", "team-release", "--label", label, "--yes"); code == 0 || !strings.Contains(stderr, "--label") {
			t.Errorf("--label %q: exit = %d stderr=%s", label, code, stderr)
		}
	}
	if queued, _ := app.ListSubmissions(project.hub.root); len(queued) != 0 {
		t.Fatalf("an invalid label was sent: %+v", queued)
	}
	code, stdout, stderr := runCLI(t, "skill", "commit", "team-release", "--label", "laptop", "--yes")
	if code != 0 {
		t.Fatalf("send: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	sub := submissionIDIn(t, stdout)
	if text := project.hub.mustRun("status"); !strings.Contains(oneLine(text), "1 submitted skill waits for review: team-release from laptop") {
		t.Fatalf("status text:\n%s", text)
	}
	if _, stdout, _ = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root); !strings.Contains(oneLine(stdout), "team-release (submitted from laptop)") {
		t.Fatalf("review:\n%s", stdout)
	}
	// The hub on this machine has no use for a label, and no other command takes one.
	for _, args := range [][]string{
		{"skill", "commit", "team-release", "--workspace", project.hub.root, "--label", "laptop"},
		{"skill", "checkout", "team-release", "--label", "laptop"},
	} {
		if code, _, stderr := runCLI(t, args...); code == 0 || !strings.Contains(stderr, "--label") {
			t.Errorf("%v: exit = %d stderr=%s", args, code, stderr)
		}
	}
}

func TestRemoteProjectStillRefusesOtherCurationCommands(t *testing.T) {
	newRemoteProject(t)
	for _, args := range [][]string{
		{"skill", "activate", "x", "--yes"},
		{"skill", "review", "x"},
	} {
		code, _, stderr := runCLI(t, args...)
		if code == 0 || !strings.Contains(stderr, "Skills are curated on the hub machine.") {
			t.Errorf("%v: exit = %d stderr=%s", args, code, stderr)
		}
	}
}

func TestRemoteCommitDoesNotSendTheTokenToAHubOnlyTheProjectNames(t *testing.T) {
	isolateClient(t)
	tokenFile := filepath.Join(t.TempDir(), "hub-token")
	if _, err := hubtoken.Ensure(tokenFile); err != nil {
		t.Fatal(err)
	}
	project := projectInDir(t)
	// The user scope does not name this hub and it is not on this machine.
	if code, stdout, stderr := runConnectCLI(t, project, "--hub", "http://hub.example.invalid:7420", "--token-file", tokenFile, "--host", "claude"); code != 0 {
		t.Fatalf("connect exit = %d\n%s%s", code, stdout, stderr)
	}
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")
	code, _, stderr := runCLI(t, "skill", "commit", "team-release", "--yes")
	if code == 0 || !strings.Contains(stderr, "only this project's files name the hub at http://hub.example.invalid:7420") {
		t.Fatalf("--yes to an unknown hub: exit = %d stderr=%s", code, stderr)
	}
	// The command it names must be one connect takes, and must keep the token
	// file: connect writes the entry it is given, so one without it would leave
	// the proxy with no token.
	command := "skillstation connect -g --hub http://hub.example.invalid:7420 --token-file " + tokenFile
	if !strings.Contains(oneLine(stderr), command) {
		t.Fatalf("the trust command is not %q:\n%s", command, stderr)
	}
	if code, _, stderr := runCLI(t, "connect", "-g", "--hub", "http://hub.example.invalid:7420", "--token-file", tokenFile, "--dry-run"); code != 0 {
		t.Fatalf("connect does not take the command it was suggested: exit = %d stderr=%s", code, stderr)
	}
}

func TestRemoteCheckoutDoesNotSendTheTokenToAHubOnlyTheProjectNames(t *testing.T) {
	isolateClient(t)
	tokenFile := filepath.Join(t.TempDir(), "hub-token")
	if _, err := hubtoken.Ensure(tokenFile); err != nil {
		t.Fatal(err)
	}
	project := projectInDir(t)
	if code, stdout, stderr := runConnectCLI(t, project, "--hub", "http://hub.example.invalid:7420", "--token-file", tokenFile, "--host", "claude"); code != 0 {
		t.Fatalf("connect exit = %d\n%s%s", code, stdout, stderr)
	}
	code, _, stderr := runCLI(t, "skill", "checkout", "team-release")
	if code == 0 || !strings.Contains(stderr, "only this project's files name the hub at http://hub.example.invalid:7420") {
		t.Fatalf("checkout from an unknown hub: exit = %d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "team-release")); err == nil {
		t.Fatal("a refused checkout left a folder behind")
	}
}

func TestRemotePullBringsTheHubsNewVersionAndKeepsLocalEdits(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")
	if code, stdout, stderr := runCLI(t, "skill", "checkout", "review-checklist"); code != 0 {
		t.Fatalf("checkout: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	folder := filepath.Join(project.dir, ".agents", "skills", "review-checklist")
	writeCLIAddFixture(t, folder, "notes.md", "mine\n", 0o644)

	hubSkill := filepath.Join(project.hub.root, "skills", "software", "review-checklist")
	writeCLIAddFixture(t, hubSkill, "references/guide.md", "from the hub\n", 0o644)
	gitOutput(t, project.hub.root, "add", "-A")
	gitOutput(t, project.hub.root, "commit", "-q", "-m", "hub adds a guide")
	if code, stdout, stderr := runCLI(t, "maintain", "rebuild", "--workspace", project.hub.root); code != 0 {
		t.Fatalf("rebuild exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}

	code, stdout, stderr := runCLI(t, "skill", "pull", "review-checklist")
	if code != 0 || !strings.Contains(stdout, "Pulled review-checklist") {
		t.Fatalf("pull: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if got := readFileText(t, filepath.Join(folder, "references", "guide.md")); got != "from the hub\n" {
		t.Fatalf("hub file not pulled: %q", got)
	}
	if got := readFileText(t, filepath.Join(folder, "notes.md")); got != "mine\n" {
		t.Fatalf("local file lost: %q", got)
	}
	if code, stdout, _ = runCLI(t, "skill", "pull", "review-checklist"); code != 0 || !strings.Contains(stdout, "up to date") {
		t.Fatalf("second pull: exit = %d\n%s", code, stdout)
	}
}

func TestRemoteCommitNoteReachesTheReviewerAndTheSenderCanAskWhatBecameOfIt(t *testing.T) {
	project := newRemoteProject(t)
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")

	for _, note := range []string{strings.Repeat("a", app.MaxSubmissionNoteRunes+1)} {
		if code, _, stderr := runCLI(t, "skill", "commit", "team-release", "--note", note, "--yes"); code == 0 || !strings.Contains(stderr, "--note") {
			t.Errorf("a note of %d characters: exit = %d stderr=%s", len(note), code, stderr)
		}
	}
	if code, _, stderr := runCLI(t, "skill", "show", "team-release", "--note", "x"); code == 0 || !strings.Contains(stderr, "--note") {
		t.Errorf("--note on another command: exit = %d stderr=%s", code, stderr)
	}
	if queued, _ := app.ListSubmissions(project.hub.root); len(queued) != 0 {
		t.Fatalf("a refused note still sent something: %+v", queued)
	}

	code, stdout, stderr := runCLI(t, "skill", "commit", "team-release", "--note", "It skipped the tag step.\nFixed it.", "--json", "--yes")
	if code != 0 {
		t.Fatalf("send: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	var sent app.RemoteCommitResult
	if err := json.Unmarshal([]byte(stdout), &sent); err != nil || !sent.NoteKept || sent.StatusCommand != "skillstation skill submission "+sent.SubmissionID || len(sent.Warnings) != 0 {
		t.Fatalf("send result: %+v %v\n%s", sent, err, stdout)
	}
	if _, stdout, _ = runCLI(t, "skill", "review", sent.SubmissionID, "--workspace", project.hub.root); !strings.Contains(stdout, "Note from the sender:") || !strings.Contains(stdout, "| It skipped the tag step.") || !strings.Contains(stdout, "| Fixed it.") {
		t.Fatalf("the reviewer does not see the note:\n%s", stdout)
	}

	// The sender asks the hub, and hears how the submission left the queue.
	_, stdout, _ = runCLI(t, "skill", "submission", sent.SubmissionID)
	if !strings.Contains(oneLine(stdout), sent.SubmissionID+" (team-release) is waiting for a person on the hub to review it.") {
		t.Fatalf("waiting:\n%s", stdout)
	}
	if code, stdout, stderr := runCLI(t, "skill", "review", sent.SubmissionID, "--workspace", project.hub.root, "--discard"); code != 0 {
		t.Fatalf("discard: exit = %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, _ = runCLI(t, "skill", "submission", sent.SubmissionID, "--json")
	var asked app.RemoteSubmissionResult
	if err := json.Unmarshal([]byte(stdout), &asked); err != nil || code != 0 || asked.Status != app.SubmissionDiscarded || asked.DecidedAt == "" || asked.SkillID != "team-release" {
		t.Fatalf("discarded: exit %d %+v %v\n%s", code, asked, err, stdout)
	}
	if _, stdout, _ = runCLI(t, "skill", "submission", sent.SubmissionID); !strings.Contains(oneLine(stdout), "was discarded by the person who reviews it") {
		t.Fatalf("discarded text:\n%s", stdout)
	}

	code, _, stderr = runCLI(t, "skill", "submission", "SUB-00000000000000000000000000")
	if code == 0 || !strings.Contains(oneLine(stderr), "has no record of SUB-00000000000000000000000000") {
		t.Fatalf("an unknown id: exit = %d stderr=%s", code, stderr)
	}
	if code, _, stderr = runCLI(t, "skill", "submission", "team-release"); code == 0 || !strings.Contains(stderr, "not a submission id") {
		t.Fatalf("a skill id: exit = %d stderr=%s", code, stderr)
	}
}

func TestACommitToTheHubOnThisMachineTakesNoNote(t *testing.T) {
	project := newRemoteProject(t)
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")
	if code, _, stderr := runCLI(t, "skill", "commit", "team-release", "--workspace", project.hub.root, "--note", "why"); code == 0 || !strings.Contains(stderr, "--note") {
		t.Errorf("exit = %d stderr=%s", code, stderr)
	}
}

func TestRemoteEditFromAProjectSendsTheChangeForReviewWithoutACheckout(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")
	head := hubHead(t, project.hub.root)
	folder := filepath.Join(project.dir, ".agents", "skills", "review-checklist")
	edit := []string{"skill", "edit", "review-checklist", "--description", "Cut and tag a release. Use when the user wants a release.",
		"--trigger", "cut a release", "--trigger", "tag a release", "--example", "cut the release for this week", "--example", "tag version 2 of the app", "--example", "publish a release"}

	// Without --yes it only shows the change.
	code, stdout, stderr := runCLI(t, edit...)
	if code != 0 || !strings.Contains(oneLine(stdout), "review-checklist: this changes 2 files; it sends the result to the hub at "+project.hubURL+" for review.") ||
		!strings.Contains(stdout, "+description: Cut and tag a release.") || !strings.Contains(stdout, "+        - tag a release") || !strings.Contains(stdout, "Nothing sent yet.") {
		t.Fatalf("preview: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if queued, _ := app.ListSubmissions(project.hub.root); len(queued) != 0 {
		t.Fatalf("a preview sent something: %+v", queued)
	}

	// The preview says what the hub holds and what the edit leaves, so what a flag replaces is seen.
	code, stdout, _ = runCLI(t, append(edit, "--json")...)
	var shown app.RemoteEditPreview
	if err := json.Unmarshal([]byte(stdout), &shown); err != nil || code != 0 ||
		len(shown.RoutingBefore.Triggers) != 1 || shown.RoutingBefore.Triggers[0] != "cut a release" || shown.RoutingBefore.MinScope != "single_step" ||
		len(shown.RoutingAfter.Triggers) != 2 || shown.RoutingAfter.NotFor[0] != "deploying" || shown.RoutingAfter.MinScope != "single_step" {
		t.Fatalf("json preview: exit = %d %v\nbefore=%+v after=%+v\n%s", code, err, shown.RoutingBefore, shown.RoutingAfter, stdout)
	}

	code, stdout, stderr = runCLI(t, append(edit, "--note", "Triggers were too narrow.", "--yes")...)
	if code != 0 || !strings.Contains(stdout, "Sent as SUB-") || !strings.Contains(stdout, "skillstation skill submission SUB-") {
		t.Fatalf("send: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	sub := submissionIDIn(t, stdout)
	if _, err := os.Stat(folder); err == nil {
		t.Fatalf("an edit from a project left a checkout in it: %s", folder)
	}
	if hubHead(t, project.hub.root) != head {
		t.Fatal("sending an edit changed the hub before anyone reviewed it")
	}
	code, stdout, stderr = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root, "--diff")
	if code != 0 || !strings.Contains(oneLine(stdout), "review-checklist (submitted from "+project.machine+"): 2 files changed since checkout.") ||
		!strings.Contains(stdout, "Note from the sender:") || !strings.Contains(stdout, "| Triggers were too narrow.") || strings.Contains(stdout, "needs a decision") {
		t.Fatalf("review: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}

	atTerminal(t, "y\ny\n")
	if code, stdout, stderr = runCLI(t, "skill", "review", sub, "--workspace", project.hub.root); code != 0 {
		t.Fatalf("approve: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	hubFolder := filepath.Join(project.hub.root, "skills", "software", "review-checklist")
	if data := readFileText(t, filepath.Join(hubFolder, "SKILL.md")); !strings.Contains(data, "description: Cut and tag a release. Use when the user wants a release.") || !strings.Contains(data, "Cut a release and tag it.") {
		t.Fatalf("the hub SKILL.md after approval:\n%s", data)
	}
	_, stdout, _ = runCLI(t, "skill", "show", "review-checklist", "--workspace", project.hub.root)
	if !strings.Contains(stdout, "tag a release") {
		t.Fatalf("the hub did not take the routing:\n%s", stdout)
	}
	// A change to a skill that is active is live at once, and the sender is told so.
	if _, stdout, _ = runCLI(t, "skill", "submission", sub); !strings.Contains(oneLine(stdout), "The skill is active on the hub, so agents get it.") {
		t.Fatalf("status of an applied change to an active skill:\n%s", stdout)
	}

	// The same change again has nothing left to change.
	if code, stdout, _ = runCLI(t, append(edit, "--yes")...); code != 0 || !strings.Contains(stdout, "Nothing to change: review-checklist already has these values.") {
		t.Fatalf("repeat: exit = %d\n%s", code, stdout)
	}
}

func TestRemoteEditRefusesWhatOnlyTheHubChanges(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")
	for _, args := range [][]string{
		{"skill", "edit", "review-checklist", "--name", "Other", "--yes"},
		{"skill", "edit", "review-checklist", "--rationale", "why", "--yes"},
		{"skill", "edit", "review-checklist", "--runtime-file", "runtime.yaml", "--yes"},
		{"skill", "edit", "review-checklist", "--editor"},
	} {
		code, _, stderr := runCLI(t, args...)
		if code == 0 || !strings.Contains(oneLine(stderr), "skillstation skill checkout review-checklist") {
			t.Errorf("%v: exit = %d stderr=%s", args, code, stderr)
		}
	}
	if code, _, stderr := runCLI(t, "skill", "edit", "no-such-skill", "--description", "x", "--yes"); code == 0 || stderr == "" {
		t.Errorf("an unknown skill: exit = %d stderr=%s", code, stderr)
	}
	if queued, _ := app.ListSubmissions(project.hub.root); len(queued) != 0 {
		t.Fatalf("a refused edit sent something: %+v", queued)
	}
}

func TestAppliedStatusOfASkillThePersonWroteDoesNotOfferPull(t *testing.T) {
	project := newRemoteProject(t)
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")
	_, stdout, _ := runCLI(t, "skill", "commit", "team-release", "--yes")
	sub := submissionIDIn(t, stdout)
	atTerminal(t, "y\ny\nn\n")
	if code, stdout, stderr := runCLI(t, "skill", "review", sub, "--workspace", project.hub.root); code != 0 {
		t.Fatalf("approve: exit = %d\n%s%s", code, stdout, stderr)
	}
	_, stdout, _ = runCLI(t, "skill", "submission", sub)
	if !strings.Contains(oneLine(stdout), "was reviewed and applied on the hub") || !strings.Contains(oneLine(stdout), "The skill is a draft on the hub, so agents do not get it until the person who runs the hub activates it.") || strings.Contains(stdout, "skill pull") {
		t.Fatalf("a folder that is not a checkout has nothing to pull:\n%s", stdout)
	}
}

func TestCommitWarnsWhenASkillYAMLInACheckoutWouldReplaceTheHubsRouting(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")
	runCLI(t, "skill", "checkout", "review-checklist")
	folder := filepath.Join(project.dir, ".agents", "skills", "review-checklist")
	if err := os.WriteFile(filepath.Join(folder, "skill.yaml"), []byte("schema_version: 1\nrouting:\n  triggers:\n    - tag a release\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "skill", "commit", "review-checklist")
	if code != 0 || !strings.Contains(oneLine(stdout), "The hub keeps the routing this skill.yaml lists (1 trigger, 0 examples, 0 not-for items) and drops whatever else it holds for review-checklist.") ||
		!strings.Contains(oneLine(stdout), "skillstation skill edit review-checklist --trigger") {
		t.Fatalf("preview: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}

	// A skill the person wrote has no hub routing to replace, so it is not warned about.
	runCLI(t, "skill", "create", "team-release", "--description", "Cut a team release", "--trigger", "cut a team release", "--not-for", "deploying", "--example", "cut the release")
	if _, stdout, _ = runCLI(t, "skill", "commit", "team-release"); strings.Contains(stdout, "drops whatever else") {
		t.Fatalf("a new skill was warned about:\n%s", stdout)
	}
}

func TestRemoteEditWarnsOfTheEntriesAFlagWouldRemove(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")

	// The hub holds the trigger "cut a release"; a flag naming only a new one replaces the field.
	code, stdout, _ := runCLI(t, "skill", "edit", "review-checklist", "--trigger", "tag a release", "--json")
	var preview app.RemoteEditPreview
	if err := json.Unmarshal([]byte(stdout), &preview); err != nil || code != 0 {
		t.Fatalf("preview: exit = %d %v\n%s", code, err, stdout)
	}
	var found bool
	for _, warning := range preview.Warnings {
		found = found || (warning.Code == "routing_entries_removed" && strings.Contains(warning.Summary, `trigger "cut a release"`))
	}
	if !found {
		t.Fatalf("no warning that the edit removes the hub's trigger: %+v", preview.Warnings)
	}
	if _, text, _ := runCLI(t, "skill", "edit", "review-checklist", "--trigger", "tag a release"); !strings.Contains(oneLine(text), `This change removes 1 entry the hub holds: trigger "cut a release".`) {
		t.Fatalf("the text preview does not say it:\n%s", text)
	}

	// Repeating the entry it keeps removes nothing, so there is nothing to warn about.
	_, stdout, _ = runCLI(t, "skill", "edit", "review-checklist", "--trigger", "cut a release", "--trigger", "tag a release", "--json")
	preview = app.RemoteEditPreview{}
	if err := json.Unmarshal([]byte(stdout), &preview); err != nil {
		t.Fatal(err)
	}
	for _, warning := range preview.Warnings {
		if warning.Code == "routing_entries_removed" {
			t.Fatalf("a change that keeps every entry was warned about: %+v", warning)
		}
	}
}

func TestSkillShowNamesTheSubmissionsThatWaitForTheSkill(t *testing.T) {
	project := newRemoteProject(t)
	project.hub.createReady("review-checklist")
	project.hub.mustRun("skill", "activate", "review-checklist", "--yes")

	code, stdout, _ := runCLI(t, "skill", "show", "review-checklist", "--workspace", project.hub.root)
	if code != 0 || strings.Contains(stdout, "for this skill") {
		t.Fatalf("show without a submission: exit = %d\n%s", code, stdout)
	}

	runCLI(t, "skill", "checkout", "review-checklist")
	appendToFile(t, filepath.Join(project.dir, ".agents", "skills", "review-checklist", "SKILL.md"), "\nOne more line from machine A.\n")
	_, stdout, _ = runCLI(t, "skill", "commit", "review-checklist", "--yes")
	sub := submissionIDIn(t, stdout)

	code, stdout, _ = runCLI(t, "skill", "show", "review-checklist", "--workspace", project.hub.root)
	if code != 0 || !strings.Contains(stdout, "1 submission waits for this skill") || !strings.Contains(oneLine(stdout), "skillstation skill review "+sub) {
		t.Fatalf("show with a waiting submission: exit = %d\n%s", code, stdout)
	}
}

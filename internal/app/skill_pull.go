package app

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	sourcepkg "github.com/vantt/skillstation/internal/source"
	"github.com/vantt/skillstation/internal/workspace"
)

// SkillPullInput names the checkout to bring up to date with the hub.
type SkillPullInput struct {
	// Target is a skill id (read from the project's .claude/skills) or a folder.
	Target string `json:"target"`
	// ProjectDir is the directory the command ran in.
	ProjectDir string `json:"project_dir"`
	// Resolutions settle the files the hub and the project both changed.
	Resolutions []SkillCommitResolution `json:"resolutions,omitempty"`
}

// SkillPullResult says what a pull changed in the project folder. Nothing is
// written while Conflicts lists a file.
type SkillPullResult struct {
	Result
	SkillID string `json:"skill_id"`
	Folder  string `json:"folder"`
	Hub     string `json:"hub"`
	// UpToDate says the checkout already starts from the hub's version.
	UpToDate bool `json:"up_to_date"`
	// Files lists the changes the hub's version makes to the folder.
	Files []SkillCommitFile `json:"files"`
	// Kept lists the files where the folder keeps the project's own changes.
	Kept      []string `json:"kept,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
	// BaseVersion is the hub version the folder now starts from.
	BaseVersion string `json:"base_version,omitempty"`
}

// SkillPullService brings the hub's current version of a skill into its
// checkout in a project, keeping the project's own changes. It never writes
// to the hub.
type SkillPullService struct {
	Clock Clock
}

// Pull merges the hub's version of a checked-out skill into the folder. A
// file both sides changed in a way the merge cannot settle needs a resolution;
// until every such file has one, nothing is written.
func (service SkillPullService) Pull(ctx context.Context, path string, input SkillPullInput) (SkillPullResult, error) {
	if service.Clock == nil {
		service.Clock = SystemClock{}
	}
	root, err := workspace.Discover(path)
	if err != nil {
		return SkillPullResult{}, err
	}
	checkout, refusal, err := readPullCheckout(ctx, input)
	if err != nil || refusal != nil {
		return pullError(refusal), err
	}
	result := checkout.result(root)
	if !sameHub(checkout.marker.Hub, root) {
		result.Warnings = append(result.Warnings, Warning{Code: "other_hub", Summary: fmt.Sprintf("This folder was checked out from the hub at %s; you are pulling from %s.", checkout.marker.Hub, root)})
	}
	hub, err := loadHubSkill(root, checkout.id)
	if err != nil {
		return SkillPullResult{}, err
	}
	if hub == nil {
		return pullError(NewInvalidRequestError(fmt.Sprintf("the hub has no skill %s", checkout.id), "Commit the folder to add it: `skillstation skill commit "+checkout.id+"`.")), nil
	}
	if checkout.marker.BaseVersion == hub.version {
		return upToDate(result), nil
	}
	base, baseFound := findCheckoutBase(ctx, root, hub.relDir, checkout.id, checkout.marker)
	final, merged, refusal, err := mergeHubAndProject(ctx, root, base, baseFound, hub.tree, checkout.tree, input.Resolutions, "pull")
	if err != nil || refusal != nil {
		return pullError(refusal), err
	}
	marker := CheckoutMarker{SkillID: checkout.id, Hub: root, BaseVersion: hub.version, CheckedOutAt: service.Clock.Now().UTC().Format("2006-01-02T15:04:05Z")}
	if dirty, dirtyErr := workspace.DirtyPaths(ctx, root, []string{hub.relDir}); dirtyErr == nil && len(dirty) == 0 {
		marker.BaseCommit = workspace.HeadCommit(ctx, root)
	}
	return finishPull(result, checkout, hub.tree, final, merged.Unresolved, marker)
}

// RemotePullService pulls from a hub on another machine. Its checkout recorded
// a digest per file instead of the files, so a file only one side changed
// takes that side's version, and a file both changed needs a resolution.
type RemotePullService struct {
	Client *http.Client
	Clock  Clock
}

// Pull fetches the skill the remote hub serves today and merges it into the
// folder file by file.
func (service RemotePullService) Pull(ctx context.Context, hub RemoteHub, input SkillPullInput) (SkillPullResult, error) {
	if service.Clock == nil {
		service.Clock = SystemClock{}
	}
	hubURL, problem := NormalizeConnectHubURL(hub.URL)
	if problem != nil {
		return pullError(problem), nil
	}
	checkout, refusal, err := readPullCheckout(ctx, input)
	if err != nil || refusal != nil {
		return pullError(refusal), err
	}
	result := checkout.result(hubURL)
	if checkout.marker.Hub != hubURL {
		result.Warnings = append(result.Warnings, Warning{Code: "other_hub", Summary: fmt.Sprintf("This folder was checked out from %s; you are pulling from %s.", checkout.marker.Hub, hubURL)})
	}
	token := ReadHubToken(hubURL, hub.TokenFile)
	if token.Problem != nil {
		return pullError(NewInvalidRequestError(token.Problem.Detail, token.Problem.Fix)), nil
	}
	hubTree, digest, refusal, err := (RemoteCheckoutService{Client: service.Client}).fetch(ctx, hubURL, token.Value, checkout.id)
	if err != nil || refusal != nil {
		return pullError(refusal), err
	}
	version := skillBaseVersion(hubTree)
	if checkout.marker.BaseVersion == version {
		return upToDate(result), nil
	}
	final, unresolved, refusal := mergeByDigest(hubTree, checkout.tree, checkout.marker.BaseFiles, input.Resolutions)
	if refusal != nil {
		return pullError(refusal), nil
	}
	// A bundle carries no file modes: a script the project runs stays runnable.
	for rel, file := range final {
		if checkout.tree[rel].Executable && !file.Executable {
			file.Executable = true
			final[rel] = file
		}
	}
	baseFiles := make(map[string]string, len(hubTree))
	for rel, file := range hubTree {
		baseFiles[rel] = sourcepkg.Digest(file.Data)
	}
	marker := CheckoutMarker{SkillID: checkout.id, Hub: hubURL, BaseVersion: version, BundleDigest: digest, BaseFiles: baseFiles, CheckedOutAt: service.Clock.Now().UTC().Format("2006-01-02T15:04:05Z")}
	return finishPull(result, checkout, hubTree, final, unresolved, marker)
}

// pullCheckout is the checked-out folder a pull brings up to date.
type pullCheckout struct {
	id, dir string
	marker  CheckoutMarker
	tree    map[string]treeFile
}

func (checkout pullCheckout) result(hub string) SkillPullResult {
	return SkillPullResult{SkillID: checkout.id, Folder: checkout.dir, Hub: hub, Files: []SkillCommitFile{}}
}

// readPullCheckout reads the folder a pull targets. Only a checkout can be
// pulled: without its marker there is no base to keep the project's changes by.
func readPullCheckout(ctx context.Context, input SkillPullInput) (pullCheckout, *Error, error) {
	target, refusal := resolveCommitTarget(ctx, input.Target, input.ProjectDir)
	if refusal != nil {
		return pullCheckout{}, refusal, nil
	}
	if target.marker == nil {
		return pullCheckout{}, NewInvalidRequestError(
			fmt.Sprintf("%s was not checked out from a hub, so there is nothing to pull into it", target.dir),
			"Commit it with `skillstation skill commit "+commitArgument(input.Target)+"`, or move it away and run `skillstation skill checkout "+target.id+"`."), nil
	}
	tree, _, refusal, err := readProjectFolder(target.dir)
	if err != nil || refusal != nil {
		return pullCheckout{}, refusal, err
	}
	return pullCheckout{id: target.id, dir: target.dir, marker: *target.marker, tree: tree}, nil, nil
}

// commitArgument is the argument as typed, quoted when it needs to be.
func commitArgument(target string) string {
	if strings.ContainsAny(target, " \t'\"") {
		return fmt.Sprintf("%q", target)
	}
	return target
}

func pullError(err *Error) SkillPullResult {
	if err == nil {
		return SkillPullResult{}
	}
	return SkillPullResult{Result: ErrorResult(err), Files: []SkillCommitFile{}}
}

func upToDate(result SkillPullResult) SkillPullResult {
	warnings := result.Warnings
	result.Result = NewResult(StatusOK, fmt.Sprintf("%s is up to date with the hub.", result.SkillID))
	result.Warnings = warnings
	result.UpToDate = true
	return result
}

// finishPull reports the merge and, when no file needs a decision, writes it
// into the folder and moves the marker to the hub's version.
func finishPull(result SkillPullResult, checkout pullCheckout, hubTree, final map[string]treeFile, unresolved []string, marker CheckoutMarker) (SkillPullResult, error) {
	warnings := result.Warnings
	result.Files = describeFiles(checkout.tree, final, nil)
	for rel, file := range final {
		if hubFile, ok := hubTree[rel]; !ok || string(hubFile.Data) != string(file.Data) {
			result.Kept = append(result.Kept, rel)
		}
	}
	for rel := range hubTree {
		if _, ok := final[rel]; !ok {
			result.Kept = append(result.Kept, rel)
		}
	}
	sort.Strings(result.Kept)
	if len(unresolved) > 0 {
		result.Conflicts = unresolved
		result.Result = NewResult(StatusActionRequired, fmt.Sprintf("%s: the hub and this folder both changed %s; nothing was written.", checkout.id, pluralize(len(unresolved), "file")))
		result.Warnings = warnings
		return result, nil
	}
	if err := writeFolderTree(checkout.dir, checkout.tree, final); err != nil {
		return SkillPullResult{}, err
	}
	if err := WriteCheckoutMarker(checkout.dir, marker); err != nil {
		return SkillPullResult{}, err
	}
	result.BaseVersion = marker.BaseVersion
	result.Result = NewResult(StatusOK, fmt.Sprintf("Pulled %s: %s changed in this folder.", checkout.id, pluralize(len(result.Files), "file")))
	result.Warnings = warnings
	return result, nil
}

// mergeByDigest merges a remote hub's tree into the project's when the base is
// known only by a digest per file: the side that changed a file wins, and a
// file both sides changed differently needs a resolution. It returns the
// merged tree and the files still undecided, which keep the project's version.
func mergeByDigest(hubTree, projectTree map[string]treeFile, base map[string]string, choices []SkillCommitResolution) (map[string]treeFile, []string, *Error) {
	decided := make(map[string]SkillCommitResolution, len(choices))
	for _, choice := range choices {
		switch choice.Choice {
		case "hub", "project", "manual":
			decided[choice.Path] = choice
		default:
			return nil, nil, NewInvalidRequestError(fmt.Sprintf("%q is not a choice for %s", choice.Choice, choice.Path), "Choose hub, project, or a file with --manual.")
		}
	}
	paths := map[string]bool{}
	for rel := range hubTree {
		paths[rel] = true
	}
	for rel := range projectTree {
		paths[rel] = true
	}
	digest := func(tree map[string]treeFile, rel string) string {
		if file, ok := tree[rel]; ok {
			return sourcepkg.Digest(file.Data)
		}
		return ""
	}
	final := map[string]treeFile{}
	differing := map[string]bool{}
	var unresolved []string
	for rel := range paths {
		hubFile, inHub := hubTree[rel]
		projectFile, inProject := projectTree[rel]
		hubDigest, projectDigest := digest(hubTree, rel), digest(projectTree, rel)
		if hubDigest == projectDigest {
			if inHub {
				final[rel] = hubFile
			}
			continue
		}
		differing[rel] = true
		baseDigest := base[rel]
		choice, chosen := decided[rel]
		switch {
		case base != nil && projectDigest == baseDigest:
			if inHub {
				final[rel] = hubFile
			}
		case base != nil && hubDigest == baseDigest:
			if inProject {
				final[rel] = projectFile
			}
		case !chosen:
			unresolved = append(unresolved, rel)
			if inProject {
				final[rel] = projectFile
			}
		case choice.Choice == "hub":
			if inHub {
				final[rel] = hubFile
			}
		case choice.Choice == "project":
			if inProject {
				final[rel] = projectFile
			}
		default:
			final[rel] = treeFile{Data: choice.Content, Executable: projectFile.Executable}
		}
	}
	for _, choice := range choices {
		if !differing[choice.Path] {
			return nil, nil, NewInvalidRequestError(fmt.Sprintf("%s has no difference to decide", choice.Path), "Run `skillstation skill pull` without --accept to see which files need a decision.")
		}
	}
	sort.Strings(unresolved)
	return final, unresolved, nil
}

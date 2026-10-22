package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vantt/skillstation/internal/hostintegration"
	"github.com/vantt/skillstation/internal/shellquote"
	"github.com/vantt/skillstation/internal/userconfig"
)

// ConnectService writes the agent-host connection for a workspace into a
// project directory or the user's home configuration.
type ConnectService struct {
	// HomeDir resolves the user home directory for global connections.
	// It defaults to os.UserHomeDir.
	HomeDir func() (string, error)
	// Store holds the hub path `skillstation init` recorded; nil uses
	// userconfig.Default().
	Store userconfig.Store
}

// ConnectRequest describes one connection. Project is a folder of the project
// to connect, normally the current folder; inside Git the work tree top level
// is connected. It is ignored when Global is set.
type ConnectRequest struct {
	Project string
	// Workspace is the hub folder; empty means the hub folder Project is in,
	// else the hub `skillstation init` recorded on this machine.
	Workspace string
	Global    bool
	Hosts     []hostintegration.Host
	// DryRun reports the changes without writing them.
	DryRun bool
}

// Connect writes the connection, or with DryRun only reports it. It reuses
// the same preimage-pinned host integration machinery as workspace
// initialization.
func (service ConnectService) Connect(ctx context.Context, request ConnectRequest) (Result, error) {
	root, scope, label, problem := service.target(ctx, request)
	if problem != nil {
		return ErrorResult(problem), nil
	}
	workspacePath, problem := service.resolveHub(request, root)
	if problem != nil {
		return ErrorResult(problem), nil
	}
	isHub := scope == hostintegration.ScopeProject && sameDirectory(root, workspacePath)
	binary, err := currentBinary()
	if err != nil {
		return Result{}, err
	}
	hostRequest := hostintegration.Request{Workspace: workspacePath, Root: root, Scope: scope, Binary: binary, Hosts: request.Hosts}
	inspection, err := hostintegration.Inspect(ctx, hostRequest)
	if err != nil {
		return Result{}, err
	}
	levels := make(map[hostintegration.Host]hostintegration.IntegrationLevel, len(inspection.Hosts))
	var conflicts []string
	for _, host := range inspection.Hosts {
		levels[host.Host] = host.Level
		for _, file := range host.Files {
			if file.Conflict != "" {
				conflicts = append(conflicts, fmt.Sprintf("%s: %s", file.Path, file.Conflict))
			}
		}
	}
	if len(conflicts) > 0 {
		return ErrorResult(NewInvalidRequestError(
			"Existing host files cannot be updated safely: "+strings.Join(conflicts, "; "),
			"Fix or remove the conflicting managed content in those files, then run `skillstation connect` again. Nothing was written.",
		)), nil
	}
	plan, err := hostintegration.Plan(ctx, hostRequest)
	if err != nil {
		return Result{}, err
	}
	if len(plan.Changes) == 0 {
		summary := "Agent connection for " + label + " is already current; nothing to change."
		if isHub {
			summary = "This is the hub; `skillstation init` already connected its agents, and the connection is current."
		}
		result := NewResult(StatusReady, summary)
		appendSkillPlanItems(&result, root, plan, levels)
		return result, nil
	}
	if request.DryRun {
		result := NewResult(StatusActionRequired, "Dry run: agent connection for "+label+" was not written.")
		for _, change := range plan.Changes {
			item := hostChangeItem(root, change.Host, levels[change.Host], change.Kind, change.Path, "Would be written.")
			if change.Preview != "" {
				item.Summary += "\nManaged diff preview:\n" + change.Preview
			}
			result.Items = append(result.Items, item)
		}
		appendSkillPlanItems(&result, root, plan, levels)
		result.SuggestedActions = []Action{{Label: "Write the agent connection", Command: connectCommand(request), RequiresConfirmation: true}}
		return result, nil
	}
	applied, err := hostintegration.Apply(ctx, plan, hostintegration.ApplyOptions{Confirmed: true})
	if err != nil {
		return Result{}, err
	}
	summary := "Connected " + label + " to the hub at " + workspacePath + "."
	if isHub {
		summary = "This is the hub; `skillstation init` already connected its agents. Its connection was refreshed."
	}
	result := NewResult(StatusApplied, summary)
	for _, change := range applied.Changed {
		result.Items = append(result.Items, hostChangeItem(root, change.Host, levels[change.Host], change.Kind, change.Path, "Written."))
	}
	appendSkillPlanItems(&result, root, plan, levels)
	if len(applied.Changed) > 0 {
		result.Warnings = append(result.Warnings, hostBestEffortWarning())
	}
	return result, nil
}

// resolveHub picks the hub to connect: the folder the request names, else
// the hub that contains the project folder, else the hub `skillstation init`
// recorded on this machine. connect never creates a hub.
func (service ConnectService) resolveHub(request ConnectRequest, root string) (string, *Error) {
	if request.Workspace != "" {
		absolute, err := filepath.Abs(request.Workspace)
		if err != nil {
			return "", NewInvalidRequestError("The hub path cannot be resolved: "+err.Error(), "Pass the folder of a hub on this machine: `skillstation connect <path>`.")
		}
		if !IsHubRoot(absolute) {
			return "", NewInvalidRequestError("There is no hub at "+absolute+".", "Pass the folder where you ran `skillstation init`, or create a hub with `"+NewHubCommand()+"`.")
		}
		return filepath.Clean(absolute), nil
	}
	if IsHubRoot(root) {
		return root, nil
	}
	store := service.Store
	if store == nil {
		store = userconfig.Default()
	}
	config, err := store.Load()
	if err != nil {
		return "", NewInvalidRequestError("The Skillstation user configuration cannot be read: "+err.Error(), "Pass the hub folder: `skillstation connect <path>`.")
	}
	if config.Hub == "" {
		return "", NewInvalidRequestError("No hub is set up on this machine.", "Create one with `"+NewHubCommand()+"`, then run `skillstation connect` here again. For a hub on another machine, run `skillstation connect --hub <url> --token-file <file>`.")
	}
	if !IsHubRoot(config.Hub) {
		return "", NewInvalidRequestError("The hub stored at "+config.Hub+" is gone.", "If you moved it, run `"+shellquote.Chain("cd <new-location>", "skillstation init")+"` there, then `skillstation connect` here again.")
	}
	return filepath.Clean(config.Hub), nil
}

func (service ConnectService) target(ctx context.Context, request ConnectRequest) (root string, scope hostintegration.Scope, label string, problem *Error) {
	if request.Global {
		homeDir := service.HomeDir
		if homeDir == nil {
			homeDir = os.UserHomeDir
		}
		home, err := homeDir()
		if err != nil || !filepath.IsAbs(home) {
			return "", "", "", NewInvalidRequestError("The user home directory cannot be determined.", "Set HOME, or connect a single project without `--global`.")
		}
		return filepath.Clean(home), hostintegration.ScopeUser, "your user account (all projects)", nil
	}
	project := request.Project
	if project == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", "", "", NewInvalidRequestError("The current directory cannot be determined: "+err.Error(), "Run `skillstation connect` in the project folder.")
		}
		project = current
	}
	absolute, err := filepath.Abs(project)
	if err != nil {
		return "", "", "", NewInvalidRequestError("The project path cannot be resolved: "+err.Error(), "Run `skillstation connect` in the project folder.")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", "", "", NewInvalidRequestError("The project path is not an existing directory: "+absolute, "Run `skillstation connect` in the project folder.")
	}
	// A subfolder of a Git work tree connects the whole project, like checkout.
	absolute = projectRootOf(ctx, absolute)
	return absolute, hostintegration.ScopeProject, "project " + absolute, nil
}

// connectCommand is the command that writes what a dry run reported. It runs
// in the same folder, so it names the hub only when the request did.
func connectCommand(request ConnectRequest) string {
	parts := []string{"skillstation connect"}
	if request.Workspace != "" {
		parts = append(parts, shellquote.Arg(request.Workspace))
	}
	if request.Global {
		parts = append(parts, "--global")
	}
	for _, host := range request.Hosts {
		parts = append(parts, "--host", string(host))
	}
	return strings.Join(parts, " ")
}

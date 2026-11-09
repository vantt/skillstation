package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/vantt/skillstation/internal/skillruntime"
)

// Outcomes of a runtime trial.
const (
	TryNoRuntime = "no_runtime"
	TryPreview   = "preview"
	TryWorks     = "works"
	TryFails     = "fails"
)

const (
	tryOutputTail = 2 << 10
	tryFolderMode = 0o555
	tryFileMode   = 0o444
)

// SkillTryStep is one thing the trial ran.
type SkillTryStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Output string `json:"output,omitempty"`
}

// SkillTryResult is the trial of a folder's runtime block.
type SkillTryResult struct {
	Result
	SkillID string `json:"skill_id"`
	Folder  string `json:"folder"`
	Outcome string `json:"outcome"`
	// SetupCommand and CheckCommand are what a confirmed trial runs.
	SetupCommand string               `json:"setup_command,omitempty"`
	CheckCommand string               `json:"check_command,omitempty"`
	Steps        []SkillTryStep       `json:"steps"`
	Checks       []skillruntime.Check `json:"checks"`
}

// SkillTryService tries a skill folder's runtime block the way the hub serves
// it: the folder as a read-only copy, an empty state folder, and the three
// SKILLSTATION_* variables. It needs no hub and leaves nothing behind.
type SkillTryService struct{}

// Try reads the folder named by target (an id checked out in the project, or a
// path). The setup and check commands are the author's own, so they run only
// with confirm; without it the result says what would run.
func (SkillTryService) Try(ctx context.Context, target, projectDir string, confirm bool) (SkillTryResult, error) {
	failed := func(err *Error) (SkillTryResult, error) {
		return SkillTryResult{Result: ErrorResult(err), Steps: []SkillTryStep{}, Checks: []skillruntime.Check{}}, nil
	}
	resolved, refusal := resolveCommitTarget(ctx, target, projectDir)
	if refusal != nil {
		return failed(refusal)
	}
	tree, _, refusal, err := readProjectFolder(resolved.dir)
	if err != nil {
		return SkillTryResult{}, err
	}
	if refusal != nil {
		return failed(refusal)
	}
	result := SkillTryResult{SkillID: resolved.id, Folder: filepath.Clean(resolved.dir), Steps: []SkillTryStep{}, Checks: []skillruntime.Check{}}
	spec, declared, err := parseRuntimeSpec(folderSettings(tree))
	if err != nil {
		return failed(NewInvalidRequestError("the runtime block in skill.yaml is invalid: "+err.Error(), "Correct the `runtime:` block in the folder's skill.yaml, then run `skillstation skill try` again."))
	}
	if !declared {
		result.Outcome = TryNoRuntime
		result.Result = NewResult(StatusOK, resolved.id+": no runtime block in skill.yaml, so there is nothing to try.")
		return result, nil
	}
	result.SetupCommand, result.CheckCommand = spec.Setup.Command, spec.Setup.Check
	if (spec.Setup.Command != "" || spec.Setup.Check != "") && !confirm {
		result.Outcome = TryPreview
		result.Result = NewResult(StatusActionRequired, fmt.Sprintf("%s: trying the runtime block runs the skill's own check and setup commands on this machine, in a temporary folder that is removed afterwards. Run again with --yes to go ahead.", resolved.id))
		return result, nil
	}
	return runTry(ctx, result, tree, spec)
}

func runTry(ctx context.Context, result SkillTryResult, tree map[string]treeFile, spec skillruntime.Spec) (SkillTryResult, error) {
	scratch, err := os.MkdirTemp("", "skillstation-try-")
	if err != nil {
		return SkillTryResult{}, err
	}
	defer skillruntime.RemoveTree(scratch)
	skillDir, stateDir, configDir := filepath.Join(scratch, "skill"), filepath.Join(scratch, "state"), filepath.Join(scratch, "config")
	for _, dir := range []string{stateDir, configDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return SkillTryResult{}, err
		}
	}
	if err := writeReadOnlyCopy(skillDir, tree); err != nil {
		return SkillTryResult{}, err
	}
	env := map[string]string{skillruntime.EnvSkillDir: skillDir, skillruntime.EnvStateDir: stateDir, skillruntime.EnvConfigDir: configDir}
	var options skillruntime.DoctorOptions

	before := skillruntime.RunDoctor(ctx, spec, stateDir, env, true, options)
	result.Checks = before.Checks
	if before.State == skillruntime.StateUnsupportedPlatform {
		return finishTry(result, TryFails, "this platform is not one the runtime block lists"), nil
	}
	checkFailed := spec.Setup.Check != "" && !setupCheckPassed(before.Checks)
	if spec.Setup.Check != "" {
		step := SkillTryStep{Step: "check before setup", Status: skillruntime.StatusPass}
		if checkFailed {
			step.Status = skillruntime.StatusFail
		}
		step.Output = tailText(before.CheckOutput, tryOutputTail)
		result.Steps = append(result.Steps, step)
	}
	if spec.Setup.Check != "" && !checkFailed {
		result.Warnings = append(result.Warnings, Warning{Code: "check_passes_before_setup", Summary: "setup.check succeeded on an empty state folder, so it cannot tell a missing install from a present one, and setup was not run. Make the check test for what setup installs."})
	}
	if spec.Setup.Command != "" && (spec.Setup.Check == "" || checkFailed) {
		check, output := skillruntime.RunSetup(ctx, spec.Setup.Command, stateDir, env, options)
		result.Steps = append(result.Steps, SkillTryStep{Step: "setup", Status: check.Status, Detail: check.Detail, Output: tailText(output, tryOutputTail)})
		if check.Status != skillruntime.StatusPass {
			return finishTry(result, TryFails, "setup.command failed"), nil
		}
		if spec.Setup.Check == "" {
			result.Warnings = append(result.Warnings, Warning{Code: "no_setup_check", Summary: "setup ran, but the block has no setup.check, so nothing says whether a later run needs setup again."})
		}
		after := skillruntime.RunDoctor(ctx, spec, stateDir, env, true, options)
		result.Checks = after.Checks
		if spec.Setup.Check != "" {
			step := SkillTryStep{Step: "check after setup", Status: skillruntime.StatusPass, Output: tailText(after.CheckOutput, tryOutputTail)}
			if !setupCheckPassed(after.Checks) {
				step.Status = skillruntime.StatusFail
			}
			result.Steps = append(result.Steps, step)
		}
		before = after
	} else if checkFailed {
		return finishTry(result, TryFails, "setup.check fails on an empty state folder and the block has no setup.command to fix it"), nil
	}
	if before.State != skillruntime.StateReady {
		return finishTry(result, TryFails, "a requirement of the block is not met; see the checks"), nil
	}
	return finishTry(result, TryWorks, ""), nil
}

func finishTry(result SkillTryResult, outcome, reason string) SkillTryResult {
	warnings := result.Warnings // NewResult below starts a fresh envelope
	result.Outcome = outcome
	if outcome == TryWorks {
		result.Result = NewResult(StatusOK, result.SkillID+": the runtime block works from a read-only copy of the folder.")
	} else {
		result.Result = NewResult(StatusActionRequired, result.SkillID+": the runtime block does not work from a read-only copy: "+reason+".")
	}
	result.Warnings = warnings
	return result
}

func setupCheckPassed(checks []skillruntime.Check) bool {
	for _, check := range checks {
		if check.Kind == skillruntime.KindSetupCheck {
			return check.Status == skillruntime.StatusPass
		}
	}
	return false
}

// writeReadOnlyCopy lays the folder out as the hub serves a skill: no file is
// writable or executable, and neither is any directory.
func writeReadOnlyCopy(root string, tree map[string]treeFile) error {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, path := range paths {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, tree[path].Data, tryFileMode); err != nil {
			return err
		}
	}
	var dirs []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			dirs = append(dirs, path)
		}
		return err
	}); err != nil {
		return err
	}
	// Deepest first, so a parent is closed only after its children are.
	for index := len(dirs) - 1; index >= 0; index-- {
		if err := os.Chmod(dirs[index], tryFolderMode); err != nil {
			return err
		}
	}
	return nil
}

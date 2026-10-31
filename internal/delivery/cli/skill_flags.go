package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/vantt/skillstation/internal/app"
)

// skillValueFlags maps each skill flag that takes a value to the field it sets.
// Repeatable flags append.
var skillValueFlags = map[string]func(*skillFlags, string){
	"--workspace":       func(f *skillFlags, v string) { f.workspace = v },
	"--id":              func(f *skillFlags, v string) { f.id = v },
	"--collection":      func(f *skillFlags, v string) { f.collection = v },
	"--name":            func(f *skillFlags, v string) { f.name = v },
	"--description":     func(f *skillFlags, v string) { f.description = v },
	"--content-file":    func(f *skillFlags, v string) { f.contentFile = v },
	"--min-scope":       func(f *skillFlags, v string) { f.minScope = v },
	"--from-suggestion": func(f *skillFlags, v string) { f.fromSuggestion = v },
	"--rationale":       func(f *skillFlags, v string) { f.rationale = v },
	"--operation":       func(f *skillFlags, v string) { f.operations = append(f.operations, v) },
	"--trigger":         func(f *skillFlags, v string) { f.triggers = append(f.triggers, v) },
	"--not-for":         func(f *skillFlags, v string) { f.notFor = append(f.notFor, v) },
	"--example":         func(f *skillFlags, v string) { f.examples = append(f.examples, v) },
	"--counter-example": func(f *skillFlags, v string) { f.counterExamples = append(f.counterExamples, v) },
	"--approve-content": func(f *skillFlags, v string) { f.approveContent = v },
	"--runtime-file":    func(f *skillFlags, v string) { f.runtimeFile = v },
	"--proposal":        func(f *skillFlags, v string) { f.proposalID = v },
	"--proposal-digest": func(f *skillFlags, v string) { f.proposalDigest = v },
	"--base-version":    func(f *skillFlags, v string) { f.baseVersion = v },
	"--idempotency-key": func(f *skillFlags, v string) { f.idempotencyKey = v },
	"--state":           func(f *skillFlags, v string) { f.state = v },
	"--ref":             func(f *skillFlags, v string) { f.ref = v },
	"--path":            func(f *skillFlags, v string) { f.subPath = v },
	"--skill":           func(f *skillFlags, v string) { f.skills = append(f.skills, v) },
	"--target":          func(f *skillFlags, v string) { f.target = v },
	"--write-conflicts": func(f *skillFlags, v string) { f.writeConflicts = v },
	"--accept":          func(f *skillFlags, v string) { f.accepts = append(f.accepts, v) },
	"--manual":          func(f *skillFlags, v string) { f.manuals = append(f.manuals, v) },
	"--label": func(f *skillFlags, v string) {
		f.label = &v
		f.labels, f.labelsSet = append(f.labels, v), true
	},
	"--topic":      func(f *skillFlags, v string) { f.topics, f.topicsSet = append(f.topics, v), true },
	"--technology": func(f *skillFlags, v string) { f.technologies, f.technologiesSet = append(f.technologies, v), true },
	"--origin":     func(f *skillFlags, v string) { f.origin = v },
	"--search":     func(f *skillFlags, v string) { f.search = v },
	"--sort":       func(f *skillFlags, v string) { f.sortBy = v },
}

// skillBoolFlags maps each skill switch to the field it turns on.
var skillBoolFlags = map[string]func(*skillFlags) *bool{
	"--all":          func(f *skillFlags) *bool { return &f.all },
	"--json":         func(f *skillFlags) *bool { return &f.jsonOutput },
	"--yes":          func(f *skillFlags) *bool { return &f.yes },
	"--diff":         func(f *skillFlags) *bool { return &f.diff },
	"--no-commit":    func(f *skillFlags) *bool { return &f.noCommit },
	"--edit-routing": func(f *skillFlags) *bool { return &f.editRouting },
	"--discard":      func(f *skillFlags) *bool { return &f.discard },
	"--editor":       func(f *skillFlags) *bool { return &f.editor },
	"--verbose":      func(f *skillFlags) *bool { return &f.verbose },
	"--check":        func(f *skillFlags) *bool { return &f.check },
	"--no-check":     func(f *skillFlags) *bool { return &f.noCheck },
	"--exit-code":    func(f *skillFlags) *bool { return &f.exitCode },
}

func parseSkillFlags(subcommand string, args []string) (skillFlags, error) {
	flags, err := scanSkillFlags(args)
	if err != nil {
		return flags, err
	}
	positionals := flags.positionals
	flags.rerun = rerunCommand("skillstation skill", append([]string{subcommand}, args...))
	if err := checkSkillFlagScope(subcommand, flags); err != nil {
		return flags, err
	}
	if err := checkSkillSubcommandFlags(subcommand, &flags, positionals); err != nil {
		return flags, err
	}
	if subcommand == "create" {
		// A create writes in the project only, so it needs no hub.
		return flags, nil
	}
	return resolveSkillFlagsWorkspace(subcommand, flags)
}

// scanSkillFlags reads the flags and positional arguments of a skill command
// without checking which subcommand they belong to.
func scanSkillFlags(args []string) (skillFlags, error) {
	flags := skillFlags{}
	positionals := []string{}
	for index := 0; index < len(args); index++ {
		value := args[index]
		if set, ok := skillValueFlags[value]; ok {
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return flags, fmt.Errorf("%s requires a value", value)
			}
			index++
			set(&flags, args[index])
			continue
		}
		if field, ok := skillBoolFlags[value]; ok {
			*field(&flags) = true
			continue
		}
		if strings.HasPrefix(value, "-") {
			return flags, fmt.Errorf("unknown argument %q", value)
		}
		positionals = append(positionals, value)
	}
	flags.positionals = positionals
	return flags, nil
}

// checkSkillFlagScope rejects the flags that belong to a few subcommands only.
func checkSkillFlagScope(subcommand string, flags skillFlags) error {
	if flags.noCommit && !hubChangingSkillSubcommands[subcommand] {
		return fmt.Errorf("--no-commit is available only for commands that change the hub")
	}
	if err := flags.checkFacetFlags(subcommand); err != nil {
		return err
	}
	if flags.discard && subcommand != "review" {
		return errors.New("--discard is available only for skill review of a submission")
	}
	if flags.state != "" && subcommand != "list" {
		return errors.New("--state is available only for skill list")
	}
	if flags.runtimeFile != "" && subcommand != "edit" && subcommand != "commit" {
		return errors.New("--runtime-file is available only for skill edit and skill commit")
	}
	if flags.fromSuggestion != "" && subcommand != "edit" {
		return errors.New("--from-suggestion is available only for skill edit")
	}
	return nil
}

// checkSkillSubcommandFlags takes the positional arguments of a subcommand and
// rejects the flags it does not accept.
func checkSkillSubcommandFlags(subcommand string, flags *skillFlags, positionals []string) error {
	switch subcommand {
	case "outdated":
		if len(positionals) != 0 {
			return errors.New("outdated accepts no positional arguments")
		}
		return nil
	case "upstream":
		return singleSkillID(flags, positionals, "upstream")
	case "update":
		return parseSkillUpdateFlags(flags, positionals)
	case "add":
		return parseSkillAddFlags(flags, positionals)
	case "review":
		return parseSkillReviewFlags(flags, positionals)
	case "doctor":
		return parseSkillDoctorFlags(flags, positionals)
	case "env":
		return parseSkillEnvArgs(flags, positionals)
	case "list":
		return parseSkillListFlags(flags, positionals)
	case "create":
		return parseSkillCreateFlags(flags, positionals)
	case "checkout":
		return parseSkillCheckoutFlags(flags, positionals)
	case "pull":
		return parseSkillPullFlags(flags, positionals)
	case "lint":
		return parseSkillLintFlags(flags, positionals)
	case "commit":
		return parseSkillCommitFlags(flags, positionals)
	case "edit":
		return parseSkillEditFlags(flags, positionals)
	case "activate", "deprecate", "archive", "show":
		return parseSkillTransitionFlags(subcommand, flags, positionals)
	case "confirm":
		return parseSkillConfirmFlags(flags, positionals)
	default:
		return fmt.Errorf("unsupported skill subcommand %q", subcommand)
	}
}

func parseSkillUpdateFlags(flags *skillFlags, positionals []string) error {
	if len(positionals) > 1 || (len(positionals) == 0 && flags.id == "") {
		return errors.New("update requires a skill ID (e.g. `skillstation skill update <id>`). To update the skillstation binary itself, run `skillstation update`")
	}
	if err := singleSkillID(flags, positionals, "update"); err != nil {
		return err
	}
	for _, acc := range flags.accepts {
		parts := strings.SplitN(acc, "=", 2)
		if len(parts) != 2 || (parts[1] != "upstream" && parts[1] != "local" && parts[1] != "merged") {
			val := acc
			if len(parts) == 2 {
				val = parts[1]
			}
			return fmt.Errorf("invalid --accept value %q: allowed values are upstream, local, or merged", val)
		}
	}
	for _, man := range flags.manuals {
		parts := strings.SplitN(man, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --manual flag %q: must be <path>=<file>", man)
		}
		filePath := strings.TrimSpace(parts[1])
		if _, statErr := os.Stat(filePath); statErr != nil {
			return fmt.Errorf("manual file %q does not exist", filePath)
		}
	}
	return nil
}

func parseSkillAddFlags(flags *skillFlags, positionals []string) error {
	if len(positionals) != 1 {
		return errors.New("add requires exactly one locator")
	}
	flags.locator = positionals[0]
	if flags.all && len(flags.skills) > 0 {
		return errors.New("--all and --skill cannot be combined")
	}
	if flags.all && flags.id != "" {
		return errors.New("cannot specify --id when adding multiple skills")
	}
	return nil
}

func parseSkillReviewFlags(flags *skillFlags, positionals []string) error {
	if err := singleSkillID(flags, positionals, "review"); err != nil {
		return err
	}
	if _, isSubmission := app.SubmissionIDOf(flags.id); isSubmission {
		// A submission is applied only by a person at a terminal, so there
		// is no --yes. --discard drops it.
		if flags.yes || flags.idempotencyKey != "" || flags.editor || flags.contentFile != "" || flags.proposalID != "" || flags.approveContent != "" || flags.collection != "" || flags.runtimeFile != "" {
			return errors.New("a submission is approved by a person at a terminal; review accepts only --diff, --discard, --accept, --manual, --write-conflicts, --edit-routing and the routing flags for it")
		}
		if err := validateResolutionFlags(flags); err != nil {
			return err
		}
		if flags.discard && (flags.diff || flags.editRouting || len(flags.accepts)+len(flags.manuals) > 0 || flags.writeConflicts != "") {
			return errors.New("--discard cannot be combined with other review flags")
		}
		return nil
	}
	if flags.yes || flags.diff || flags.idempotencyKey != "" || flags.editor || flags.contentFile != "" || flags.proposalID != "" || flags.discard || flags.editRouting || len(flags.accepts)+len(flags.manuals) > 0 || flags.writeConflicts != "" {
		return errors.New("review does not accept mutation flags")
	}
	return nil
}

func parseSkillDoctorFlags(flags *skillFlags, positionals []string) error {
	if err := singleSkillID(flags, positionals, "doctor"); err != nil {
		return err
	}
	if flags.collection != "" || flags.name != "" || flags.description != "" || flags.contentFile != "" || flags.minScope != "" || flags.rationale != "" || flags.hasRoutingFlags() || flags.approveContent != "" || flags.editor || flags.yes || flags.diff || flags.idempotencyKey != "" || flags.proposalID != "" || flags.proposalDigest != "" || flags.baseVersion != "" || flags.all || len(flags.skills) != 0 || flags.ref != "" || flags.subPath != "" {
		return errors.New("doctor accepts only its skill ID, --workspace, and --json")
	}
	return nil
}

func parseSkillListFlags(flags *skillFlags, positionals []string) error {
	if len(positionals) != 0 {
		return errors.New("list accepts no positional arguments")
	}
	if flags.id != "" || flags.collection != "" || flags.name != "" || flags.description != "" || flags.contentFile != "" || flags.minScope != "" || flags.rationale != "" || flags.hasRoutingFlags() || flags.approveContent != "" || flags.editor || flags.yes || flags.diff || flags.idempotencyKey != "" || flags.proposalID != "" || flags.proposalDigest != "" || flags.baseVersion != "" {
		return errors.New("list accepts only --state, --origin, --topic, --label, --search, --sort, --workspace, and --json")
	}
	return nil
}

func parseSkillLintFlags(flags *skillFlags, positionals []string) error {
	if len(positionals) > 1 {
		return errors.New("lint accepts at most one skill id")
	}
	if len(positionals) == 1 {
		flags.id = positionals[0]
	}
	if flags.collection != "" || flags.hasEditFlags() || flags.hasRoutingFlags() || flags.approveContent != "" || flags.yes || flags.diff || len(flags.accepts)+len(flags.manuals) > 0 {
		return errors.New("lint accepts only a skill id, --all, --workspace, and --json")
	}
	return nil
}

func parseSkillEditFlags(flags *skillFlags, positionals []string) error {
	if err := singleSkillID(flags, positionals, "edit"); err != nil {
		return err
	}
	if flags.proposalID != "" || flags.proposalDigest != "" || flags.baseVersion != "" {
		return errors.New("edit does not accept confirmation pins")
	}
	if flags.editor && flags.contentFile != "" {
		return errors.New("--editor and --content-file cannot be combined")
	}
	if flags.fromSuggestion != "" {
		if flags.editor || flags.contentFile != "" || flags.name != "" || flags.description != "" || flags.rationale != "" || flags.hasRoutingFlags() || flags.hasFacetEditFlags() || flags.approveContent != "" || flags.runtimeFile != "" {
			return errors.New("--from-suggestion adds the suggested text as it is, so it cannot be combined with --example, --counter-example, --not-for, --trigger, --operation, --min-scope, --label, --topic, --technology, --description, --name, --rationale, --approve-content, --content-file, --runtime-file or --editor")
		}
	} else if !flags.editor && flags.contentFile == "" && flags.name == "" && flags.description == "" && flags.rationale == "" && !flags.hasRoutingFlags() && !flags.hasFacetEditFlags() && flags.approveContent == "" && flags.runtimeFile == "" {
		return errors.New("edit requires changed fields, --label, --topic, --technology, --content-file, --runtime-file, --editor, or --from-suggestion")
	}
	if flags.approveContent != "" && !contentDigestPattern.MatchString(flags.approveContent) {
		return errors.New("--approve-content requires a digest of the form sha256:<64 lowercase hex characters>")
	}
	return nil
}

// parseSkillTransitionFlags checks activate, deprecate, archive and show, which
// take one skill ID and no content.
func parseSkillTransitionFlags(subcommand string, flags *skillFlags, positionals []string) error {
	if err := singleSkillID(flags, positionals, subcommand); err != nil {
		return err
	}
	if flags.collection != "" || flags.name != "" || flags.description != "" || flags.contentFile != "" || flags.minScope != "" || flags.rationale != "" || flags.hasRoutingFlags() || flags.approveContent != "" || flags.editor || flags.proposalID != "" || flags.proposalDigest != "" || flags.baseVersion != "" {
		return fmt.Errorf("%s accepts only its skill ID and output, workspace, diff, or confirmation flags", subcommand)
	}
	if subcommand == "show" && (flags.yes || flags.diff || flags.idempotencyKey != "") {
		return errors.New("show does not accept mutation flags")
	}
	return nil
}

func parseSkillConfirmFlags(flags *skillFlags, positionals []string) error {
	if len(positionals) == 1 {
		if flags.proposalID != "" && flags.proposalID != positionals[0] {
			return fmt.Errorf("positional proposal ID %q conflicts with --proposal %q", positionals[0], flags.proposalID)
		}
		flags.proposalID = positionals[0]
	} else if len(positionals) > 1 {
		return errors.New("confirm accepts at most one positional proposal ID")
	}
	if flags.proposalID == "" {
		return errors.New("confirm requires a proposal ID")
	}
	if flags.id != "" || flags.collection != "" || flags.name != "" || flags.description != "" || flags.contentFile != "" || flags.minScope != "" || flags.rationale != "" || flags.hasRoutingFlags() || flags.approveContent != "" || flags.editor || flags.diff || flags.idempotencyKey != "" {
		return errors.New("confirm accepts only workspace, proposal pins, --yes, and output flags")
	}
	return nil
}

// resolveSkillFlagsWorkspace resolves the hub a skill command works on.
func resolveSkillFlagsWorkspace(subcommand string, flags skillFlags) (skillFlags, error) {
	resolved, resErr := resolveWorkspace(flags.workspace)
	if resErr != nil {
		// A project connected to a hub on another machine can still check a
		// skill out and send one back for review there.
		if subcommand == "commit" || subcommand == "checkout" || subcommand == "pull" {
			if hub, remote := remoteHubFor(flags.workspace); remote {
				flags.remote = &hub
				return flags, nil
			}
		}
		return flags, resErr
	}
	if subcommand == "commit" && flags.label != nil {
		return flags, errors.New("--label names this machine to a hub on another machine; a commit to the hub on this machine takes none")
	}
	flags.workspace = resolved
	return flags, nil
}

package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// skillstationNote ends every skill subcommand page: how the hub is found.
const skillstationNote = `
The hub is the one of the current folder (the hub itself or a connected
project); --workspace <path> names a hub folder instead. Run
` + "`skillstation help skill`" + ` for every skill subcommand.
`

// skillApplyNote is the shared paragraph of the subcommands that change the hub.
const skillApplyNote = `
In a terminal it shows the preview and asks "Apply this change? [y/N]".
Without a terminal (scripts, agents) or with --json it only previews; run the
same command with --yes to apply. The change becomes one Git commit of only
its own files in the hub (never pushed); --no-commit writes the files without
committing.
`

// lifecycleUsage is the page of activate, deprecate and archive.
func lifecycleUsage(name, what, example string) string {
	return `Usage: skillstation skill ` + name + ` <id> [--diff] [--yes] [--no-commit] [--json]

` + what + `
` + skillApplyNote + `
  --diff       Add the full diff to the preview
  --yes        Apply without asking
  --no-commit  Write the files without a Git commit
  --json       Machine-readable result

Example:
  ` + example + `
` + skillstationNote
}

// skillSubcommandUsage is the help page of each skill subcommand. Every flag
// named here is also on the skill page, which the docs are checked against.
var skillSubcommandUsage = map[string]string{
	"add": `Usage: skillstation skill add <github-url> [--skill <name> | --all] [--id <id>] [--collection <c>]
                          [--ref <r>] [--path <p>] [--edit-routing] [--diff] [--yes] [--no-commit]

Add draft skills from a GitHub repository, tracked for upstream updates. The
preview lists every file, the license, the routing suggested from the
description and the routing lint. In a terminal it asks to add the draft, to
approve the third-party content and, when the routing is complete, to activate
it, all in one commit. A folder on this machine is added with
` + "`skillstation skill commit <folder>`" + `.

  --skill <name>    Add only this skill of the repository (repeatable)
  --all             Add every skill the repository holds
  --id <id>         Hub id for the skill (default: its name)
  --collection <c>  Collection to file it under
  --ref <r>         Branch, tag or commit to add from
  --path <p>        Folder of the repository that holds the skill
  --edit-routing    Edit the suggested routing in $EDITOR first (terminal only)
  --diff            Print the content of every file
  --yes             Add without asking (never approves the content)
  --no-commit       Write the files without a Git commit

Examples:
  skillstation skill add https://github.com/anthropics/skills --path skills/pdf
  skillstation skill add https://github.com/anthropics/skills --path skills/pdf --yes
` + skillstationNote,

	"create": `Usage: skillstation skill create [<id>] [--description <d>] [--name <n>] [--trigger <t>]...
                             [--not-for <n>]... [--example <e>]...

Start a new skill in this project: write .claude/skills/<id>/SKILL.md and add
that folder to the project's .gitignore. Nothing changes on the hub until
` + "`skillstation skill commit <id>`" + `. In a terminal it asks what the skill does, when to
use it, when not to and for an example; without one, <id> and --description
are required and the flags fill the same file.

  --description <d>  What the skill does
  --name <n>         Title of the skill (default: the id)
  --trigger <t>      When an agent should use it (repeatable)
  --not-for <n>      When it should not (repeatable)
  --example <e>      A request that should use it (repeatable)

Examples:
  skillstation skill create
  skillstation skill create commit-style --description "Write commit messages" --trigger "write a commit message" --not-for "write release notes"
`,

	"checkout": `Usage: skillstation skill checkout <id>

Copy a skill with every file into .claude/skills/<id>/ of this project to edit
and try it, and add that folder to the project's .gitignore. The hub keeps
serving the current version until ` + "`skillstation skill commit <id>`" + `. In a project
connected to a hub on another machine it fetches the active skill from that hub.

Example:
  skillstation skill checkout commit-style
` + skillstationNote,

	"pull": `Usage: skillstation skill pull <id|folder> [--accept <p>=hub|project]... [--manual <p>=<f>]... [--json]

Bring the hub's current version of a checked-out skill into its folder in this
project. Your own changes stay: a file only the hub changed is updated, a file
only you changed is kept, and a file you both changed is merged line by line
(from a hub on another machine, file by file). When the merge cannot settle a
file, a terminal asks once whether to take the hub's version of every such
file; otherwise decide each with --accept <p>=hub or <p>=project. Nothing is
written until every file is settled, and nothing on the hub changes.

Examples:
  skillstation skill pull commit-style
  skillstation skill pull commit-style --accept SKILL.md=project
` + skillstationNote,

	"commit": `Usage: skillstation skill commit <id|folder> [--collection <c>] [--runtime-file <yaml>]
                             [--accept <p>=hub|project]... [--manual <p>=<f>]... [--write-conflicts <dir>]
                             [--trigger <t>]... [--not-for <n>]... [--example <e>]... [--min-scope <s>]
                             [--edit-routing] [--approve-content <digest>] [--diff] [--yes] [--no-commit] [--json]
       skillstation skill commit <id|folder> [--label <name>] [--yes] [--json]   (hub on another machine)

Bring a project folder back to the hub as one reviewed change and one hub
commit. It shows every changed file (and a three-way merge when the hub changed
since the checkout), runs the routing lint and asks. A folder that was never
checked out becomes a new draft skill, which a terminal offers to activate in
the same commit. When a previous commit wrote the files but Git did not commit
them, it commits them. In a project connected to a hub on another machine it
sends the folder to that hub's review queue.

A bare id reads .claude/skills/<id>/ of the project; name any other folder by
its path, such as ./my-skill or tools/my-skill.

  --collection <c>          Collection of a new skill
  --runtime-file <yaml>      Runtime block of a new skill
  --accept <p>=hub|project   Decide a file both sides changed (repeatable)
  --manual <p>=<f>           Use your own merged file for <p> (repeatable)
  --write-conflicts <dir>    Write the merged files with conflict markers there
  --trigger, --not-for, --example, --min-scope
                             Routing of a new skill, instead of the suggestion
  --edit-routing             Edit the suggested routing in $EDITOR (terminal only)
  --approve-content <digest> Approve third-party content without a terminal (a person only)
  --diff                     Add the full diff to the preview
  --yes                      Apply without asking
  --no-commit                Write the files without a Git commit
  --label <name>             Name the hub shows for this machine (another-machine hub only;
                             default: the host name)
  --json                     Machine-readable result

Examples:
  skillstation skill commit commit-style
  skillstation skill commit .claude/skills/team-release --collection tools --yes
  skillstation skill commit team-release --label laptop --yes
` + skillstationNote,

	"show": `Usage: skillstation skill show <id> [--verbose] [--json]

Show a skill in any state: its name, state, file, routing and SKILL.md.

  --verbose  Add the catalog snapshot
  --json     Machine-readable result

Example:
  skillstation skill show commit-style
` + skillstationNote,

	"review": `Usage: skillstation skill review <id> [--verbose] [--json]
       skillstation skill review <SUB-id> [--diff] [--discard] [--accept <p>=hub|project]... [--manual <p>=<f>]...
                             [--write-conflicts <dir>] [--trigger <t>]... [--not-for <n>]... [--example <e>]...
                             [--min-scope <s>] [--edit-routing] [--json]

With a skill id: a diagnostic review of the skill, its routing, its content
trust (with the --approve-content line for third-party content) and the next
step. With a SUB- id: review a skill another machine sent to this hub; in a
terminal it previews it like a commit, then applies it, approves its content
and (for a new skill) activates it, as one hub commit. --discard drops it.
Nothing is applied without a terminal.

  --verbose  Add every check
  --diff     Add the full diff of a submission
  --discard  Drop the submission
  --json     Machine-readable result

Examples:
  skillstation skill review commit-style
  skillstation skill review SUB-01JA2
` + skillstationNote,

	"doctor": `Usage: skillstation skill doctor <id> [--json]

Check this machine against a skill's runtime block: declared binaries and
versions, required environment variables (values are never shown) and, for
trusted content, the skill's setup check. It never runs setup. Exit codes: 0
ready, 1 setup required or unsupported platform, 2 invalid request.

Example:
  skillstation skill doctor pdf
` + skillstationNote,

	"env": `Usage: skillstation skill env set <id> <KEY>
       skillstation skill env unset <id> <KEY>
       skillstation skill env list <id>

Store secrets for a skill's scripts. set reads the value without echo (or from
stdin when piped) and writes it to runtime/config/<id>/env (mode 0600), never
to Git; list shows names only.

Example:
  skillstation skill env set pdf API_TOKEN
` + skillstationNote,

	"edit": `Usage: skillstation skill edit <id> [--name <n>] [--description <d>] [--rationale <r>]
                           [--content-file <f> | --editor] [--operation <o>]... [--trigger <t>]...
                           [--not-for <n>]... [--min-scope <s>] [--example <e>]... [--counter-example <e>]...
                           [--label <l>]... [--topic <t>]... [--technology <t>]...
                           [--runtime-file <yaml>] [--approve-content <digest>] [--diff] [--yes] [--no-commit]
       skillstation skill edit <id> --from-suggestion <suggestion-id> [--diff] [--yes] [--json]

Change a skill on the hub. Routing flags you pass replace that field; the
others keep their values. --label, --topic and --technology replace the whole
list (one empty value clears it). Labels are for your own grouping and never
affect routing; topics and technologies do.
` + skillApplyNote + `
  --content-file <f>         New SKILL.md from a file; --editor opens it in $EDITOR
  --trigger, --not-for       When an agent should, and should not, use the skill
  --min-scope <s>            single_step, multi_step or project
  --example, --counter-example
                             Requests that should (or should not) route to it
  --label <l>                Your own label, to group skills (repeatable)
  --topic, --technology      What the skill is about (repeatable); these feed routing
  --runtime-file <yaml>      Replace the runtime block ({} removes it)
  --approve-content <digest> Approve the content as it is now (a person only); the
                             digest is the one ` + "`skillstation skill review <id>`" + ` prints
  --from-suggestion <id>     Add a routing entry ` + "`skillstation telemetry cases suggest`" + ` found

Examples:
  skillstation skill edit commit-style --editor
  skillstation skill edit commit-style --trigger "write a commit message" --not-for "write release notes" --example "write the commit message for this diff" --yes
  skillstation skill edit pdf --label work --label favorite
` + skillstationNote,

	"outdated": `Usage: skillstation skill outdated [--check] [--all] [--exit-code] [--json]

Show repository skills that differ from upstream.

  --check      Contact the upstream repositories first
  --all        Include skills that are up to date
  --exit-code  Exit 1 when any skill needs attention

Example:
  skillstation skill outdated --check
` + skillstationNote,

	"upstream": `Usage: skillstation skill upstream <id> [--check] [--json]

Show the upstream repository of a skill and how far it drifted.

  --check  Contact the upstream repository first

Example:
  skillstation skill upstream pdf --check
` + skillstationNote,

	"update": `Usage: skillstation skill update <id> [--no-check] [--target <commit>] [--accept <p>=upstream|local|merged]...
                             [--manual <p>=<f>]... [--write-conflicts <dir>] [--yes] [--verbose] [--json]

Apply upstream changes to a skill with a three-way merge. This updates a
skill; ` + "`skillstation update`" + ` updates the skillstation program itself.
` + skillApplyNote + `
  --target <commit>                      Update to this upstream commit
  --accept <p>=upstream|local|merged     Decide a conflicting file (repeatable)
  --manual <p>=<f>                       Use your own merged file for <p>
  --write-conflicts <dir>                Write the merged files with conflict markers there

Example:
  skillstation skill update pdf --yes
` + skillstationNote,

	"activate": lifecycleUsage("activate", `Make a draft skill active, so agents using this hub get it. An active skill
needs a trigger, a --not-for entry (or a --rationale) and --min-scope;
`+"`skillstation skill review <id>`"+` names what is missing.`, "skillstation skill activate commit-style"),
	"deprecate": lifecycleUsage("deprecate", "Retire an active skill: agents no longer get it, and it stays on the hub.", "skillstation skill deprecate commit-style"),
	"archive":   lifecycleUsage("archive", "Archive a deprecated skill.", "skillstation skill archive commit-style"),

	"confirm": `Usage: skillstation skill confirm <proposal-id> [--yes] [--json]
       skillstation skill confirm --proposal <id> --proposal-digest <d> --base-version <v> [--yes] [--json]

Apply a skill add or change proposal that a preview printed.

Example:
  skillstation skill confirm PROP-123
` + skillstationNote,

	"list": `Usage: skillstation skill list [--state draft|active|deprecated|archived]
                           [--origin authored|learned|adopted|forked] [--topic <t>] [--label <l>]
                           [--search <text>] [--sort name|added|changed|uses|last-used] [--json]

List the skills of the hub. Origin is derived: an upstream makes a skill
adopted (forked once changed locally), else a learning source makes it
learned, else it is authored. Uses counts loads through the hub in the last
30 days.

  --state <s>    Only skills in this state
  --origin <o>   Only skills with this origin
  --topic <t>    Only skills with this topic or technology
  --label <l>    Only skills with this label
  --search <s>   Match id, name, description, topics, technologies and labels
  --sort <k>     name (default), added, changed, uses or last-used
  --json         Machine-readable result

Examples:
  skillstation skill list --state draft
  skillstation skill list --origin learned --sort uses
` + skillstationNote,
}

// writeSkillSubcommandHelp prints the page of one skill subcommand; ok is
// false when the name is not a skill subcommand.
func writeSkillSubcommandHelp(stdout io.Writer, name string) bool {
	usage, found := skillSubcommandUsage[name]
	if !found {
		return false
	}
	fmt.Fprint(stdout, usage)
	return true
}

// skillSubcommandHelp reports which skill subcommand a `skill` command line
// asks help for: `skill <sub> ... --help`.
func skillSubcommandHelp(args []string) (string, bool) {
	if len(args) < 2 || !slices.Contains(skillSubcommands, args[0]) {
		return "", false
	}
	return args[0], slices.ContainsFunc(args[1:], isHelpFlag)
}

// runSkillHelp handles `skillstation help skill [<sub>]`.
func runSkillHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		code, _ := writeCommandHelp(stdout, "skill")
		return code
	}
	if writeSkillSubcommandHelp(stdout, strings.TrimSpace(args[0])) {
		return 0
	}
	return writeInvalidRequest(stdout, stderr, false, fmt.Sprintf("The skill subcommand %q is not supported.", args[0]), "Run `skillstation help skill` to list the skill subcommands.")
}

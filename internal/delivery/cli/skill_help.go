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

Start a new skill in this project: write .agents/skills/<id>/SKILL.md, which
Codex, Gemini CLI, Antigravity, pi and omp read, and a pointer to it in
.claude/skills/<id>/SKILL.md for Claude Code, and add both folders to the
project's .gitignore. Nothing changes on the hub until
` + "`skillstation skill commit <id>`" + `. In a terminal it asks what the skill does, when to
use it, when not to and for an example; without one, <id> and --description
are required and the flags fill the same file.

  --description <d>  What the skill does
  --name <n>         Title for the heading of a new SKILL.md (default: the id)
  --trigger <t>      When an agent should use it (repeatable)
  --not-for <n>      When it should not (repeatable)
  --example <e>      A request that should use it (repeatable)

Examples:
  skillstation skill create
  skillstation skill create commit-style --description "Write commit messages" --trigger "write a commit message" --not-for "write release notes"
`,

	"checkout": `Usage: skillstation skill checkout <id>

Copy a skill with every file into .agents/skills/<id>/ of this project to edit
and try it. Codex, Gemini CLI, Antigravity, pi and omp read that folder; Claude
Code gets a pointer to it in .claude/skills/<id>/SKILL.md, which holds no skill
content. Both folders go into the project's .gitignore. The checkout is refused
while either folder already holds a copy of the skill. The hub keeps
serving the current version until ` + "`skillstation skill commit <id>`" + `. In a project
connected to a hub on another machine it fetches the active skill from that hub.

Example:
  skillstation skill checkout commit-style
` + skillstationNote,

	"lint": `Usage: skillstation skill lint [<id>] [--all] [--json]
       skillstation skill lint <folder|id>   (project connected to a hub on another machine)

List the skills whose routing needs work: fewer than three examples, triggers
too vague to route by, triggers that collide with another skill's, or examples
that do not route back to their skill. Active skills come first, because
agents are routed to them today; then drafts and deprecated skills. Archived
skills are left out unless named. --all lists skills without findings too.

Each entry carries the description, the path of SKILL.md, the current routing
and the findings. With --json it is the worklist an agent reads to draft better
routing, which it previews with ` + "`skillstation skill edit <id> --trigger ... --example ... --json`" + `
for a person to approve. It only reads.

In a project connected to a hub on another machine, name a skill folder (or the
id of a skill checked out here): the command sends that folder's SKILL.md and
skill.yaml to the hub, which checks them against its active skills and answers
with the same findings. The hub keeps nothing, and no other file leaves this
machine. It uses your hub token only for a hub this machine's own settings name.

The folder is also checked for its runtime block on this machine, whole, without
the hub: a package, script or install prose with no runtime block in skill.yaml, a
missing lockfile, an absolute install path, or a block that does not parse. They
are warnings, listed with the routing findings, and never stop a send.

Examples:
  skillstation skill lint
  skillstation skill lint pdf-to-epub --json
  skillstation skill lint ./my-skill
` + skillstationNote,

	"submission": `Usage: skillstation skill submission <SUB-id> [--json]

Say what became of a skill you sent to a hub on another machine: still waiting
for a person to review it, applied, discarded, or replaced by a newer
submission of the same skill from this machine. The id is the SUB-... the send
printed. It asks the hub and only reads.

On the hub machine the same command reads the hub's own queue. A hub keeps the
record of a submission that left its queue for 90 days. It uses your hub token
only for a hub this machine's own settings name, and a hub from before this
command answers that it cannot report.

Examples:
  skillstation skill submission SUB-01M6N2YPXZW8VGYN84EE1NHPX1
  skillstation skill submission SUB-01M6N2YPXZW8VGYN84EE1NHPX1 --json
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
       skillstation skill commit <id|folder> [--label <name>] [--note <text>] [--yes] [--json]   (hub on another machine)

Bring a project folder back to the hub as one reviewed change and one hub
commit. It shows every changed file, the routing it keeps from the folder's
skill.yaml, and, when the hub changed since the checkout, what changed there
and a three-way merge; it runs the routing lint and asks. A folder that was
never checked out becomes a new draft skill, which a terminal offers to
activate in the same commit. An applied commit closes the checkout (the
folder, its Claude Code pointer, the hub's record) unless the folder holds
files the commit leaves out, such as .env. When a previous commit wrote the files but Git did not commit
them, it commits them. In a project connected to a hub on another machine it
sends the folder to that hub's review queue.

A bare id reads .agents/skills/<id>/ of the project, or .claude/skills/<id>/
for a checkout an older release made there. When both hold a copy, the id is
refused and the copies are named; commit one by its path. Name any other folder
by its path, such as ./my-skill or tools/my-skill.

  --collection <c>          Collection of a new skill
  --runtime-file <yaml>      Runtime block of a new skill
  --accept <p>=hub|project   Decide a file both sides changed (repeatable)
  --manual <p>=<f>           Use your own merged file for <p> (repeatable)
  --write-conflicts <dir>    Write the merged files with conflict markers there
  --trigger, --not-for, --example, --min-scope
                             Routing to keep, written into the folder's skill.yaml too
  --edit-routing             Edit the suggested routing in $EDITOR (hubs before
                             layout 6, which suggest it; terminal only)
  --approve-content <digest> Approve third-party content without a terminal (a person only)
  --diff                     Add the full diff to the preview
  --yes                      Apply without asking
  --no-commit                Write the files without a Git commit
  --label <name>             Name the hub shows for this machine (another-machine hub only;
                             default: the host name)
  --note <text>              A few words to the person who reviews it, such as why the skill
                             changed (another-machine hub only; at most 500 characters)
  --json                     Machine-readable result

Examples:
  skillstation skill commit commit-style
  skillstation skill commit ./tools/team-release --collection tools --yes
  skillstation skill commit team-release --label laptop --yes
  skillstation skill commit team-release --note "It skipped the tag step; fixed" --yes
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

	"audit-record": `Usage: skillstation skill audit-record <id> --verdict ok|review|block --rules-digest <d> [--json]

Record a finished security audit as the audit block of the skill's
.meta/hub.yaml: the verdict, the time, the digest of the skill's content and
the digest of the rules the audit applied. The hub computes the content digest
the same way a content approval does; the skill-audit script supplies the
rules digest. It replaces an earlier record, keeps the skill's dates, and is
one Git commit of only that file.

Example:
  skillstation skill audit-record pdf --verdict ok --rules-digest sha256:` + strings.Repeat("1", 64) + `
` + skillstationNote,

	"audit-status": `Usage: skillstation skill audit-status <id>... | --all [--rules-digest <d>] [--json]

Show each skill's folder, the digest of its content as it is now, its recorded
audit, and since_last_audit: never-audited, content-changed, rules-changed or
unchanged. Rules are compared only when --rules-digest names them. Nothing is
written. The skill-audit script reads it to skip unchanged skills.

Example:
  skillstation skill audit-status --all --json
` + skillstationNote,

	"try": `Usage: skillstation skill try <folder|id> [--yes] [--json]

Try the runtime block of a skill folder before you send it, the way the hub
serves it: the folder is copied read-only into a temporary place, with an empty
state folder, and SKILLSTATION_SKILL_DIR, SKILLSTATION_STATE_DIR and
SKILLSTATION_CONFIG_DIR set. It runs the block's check (which must fail on the
empty state folder), then its setup, then the check again, and removes
everything. It needs no hub.

The check and setup commands are the skill's own and may install packages, so
the command first only says what it would run; --yes runs them. A skill with no
runtime block has nothing to try. Exit codes: 0 the block works or there is
nothing to try, 1 it does not work, 2 invalid request.

Example:
  skillstation skill try ./my-skill
  skillstation skill try ./my-skill --yes --json
` + skillstationNote,

	"scan": `Usage: skillstation skill scan <folder>... [--fail-on high] [--rules <file>] [--json]
       skillstation skill scan --self-test [--rules <file>]

Scan the files of a skill folder with the security rules that ship with
skillstation and print what they find: the rule, the file and line, why it
matters and how to fix it. It reads files only, runs nothing, and needs no hub,
so you can run it where the skill is made, before sending it. The exit code is
1 when a finding is at or above --fail-on (default high). A clean scan does not
prove the skill is safe, and the hub scans again before anyone approves it.
--rules adds the rules of a file to the shipped ones; --self-test runs every
rule's own examples.

Example:
  skillstation skill scan ./my-skill
` + skillstationNote,

	"edit": `Usage: skillstation skill edit <id> [--name <n>] [--description <d>] [--rationale <r>]
                           [--content-file <f> | --editor] [--operation <o>]... [--trigger <t>]...
                           [--not-for <n>]... [--min-scope <s>] [--example <e>]... [--counter-example <e>]...
                           [--label <l>]... [--topic <t>]... [--technology <t>]...
                           [--runtime-file <yaml>] [--approve-content <digest>] [--diff] [--yes] [--no-commit]
       skillstation skill edit <id> --from-suggestion <suggestion-id> [--diff] [--yes] [--json]
       skillstation skill edit <id> [--description <d>] [routing flags] [--note <text>] [--yes]   (hub on another machine)

Change a skill on the hub: routing, labels, topics and the runtime block in its
skill.yaml, the description in its SKILL.md. Routing flags you pass replace
that field; the others keep their values. --label, --topic and --technology
replace the whole list (one empty value clears it). Labels are for your own
grouping and never affect routing; topics and technologies do. A skill's name
is its id, so --name is accepted only when it equals the id. While the skill
is checked out in a project, the preview says so.

In a project connected to a hub on another machine, the same command changes
the description, the routing, labels, topics and technologies without a checkout:
it fetches the skill, shows the change as a diff, and with --yes (or a yes at a
terminal) sends the result to the hub's review queue, where a person approves it.
Fields you do not name keep the hub's values. The name, the SKILL.md content, the
runtime block and the approval are changed on the hub, or by checking the skill
out, editing its files and sending them with ` + "`skillstation skill commit`" + `.
--note tells the reviewer why, in at most 500 characters.
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
  --note <text>              A few words to the reviewer (another-machine hub only)

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

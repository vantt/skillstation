# Command reference

Find the command for a task here. For every flag, run `skillstation help` or `skillstation help <command>`; the built-in help wins over this page.

- Every command acts on the folder you run it in. Run hub commands inside the hub or a connected project: `cd <folder> && skillstation <command>`. For scripts, command help names `--workspace <path>` (a hub folder on this machine) and `SKILLSTATION_WORKSPACE`; `--hub` always takes a URL.
- Most commands accept `--json` for machine-readable output.
- Setup commands (`init`, `connect`, `doctor --fix`, `token rotate`, `cache clear`) act at once and print what they changed; add `--dry-run` to see the changes first.
- Commands that change skills or sources show a preview first: the files, the line counts and any routing warnings. In a terminal the command then asks `Apply this change? [y/N]` under the preview, so you do not retype it. Without a terminal (an agent, a script, `--json`) it only previews; run it again with `--yes` to apply. `--diff` adds the full diff to a preview, and the preview prints that hint. `source capture` and `skill env set|unset` write at once.
- Every change you apply becomes one Git commit in the hub, with only that change's files, a message Skillstation writes, and your hooks and signing. Nothing is pushed. `--no-commit` writes the files and leaves the commit to you. A change is refused when its files already differ from Git: commit or discard them, or use `--no-commit`.
- Only Claude Code is verified end to end; `connect` also writes files for Codex and Gemini CLI.

## Set up

| Task | Command |
|---|---|
| Make the current, empty folder your hub | `skillstation init [--dry-run]` |
| Fetch a hub from Git, or push a new hub to an empty repository | `skillstation init <git-url>` |
| Connect this project's agents to your hub | `skillstation connect [<path>] [--host claude\|codex\|gemini] [--dry-run]` |
| Connect your agents in every project | `skillstation connect -g [--host claude\|codex\|gemini] [--dry-run]` |
| Connect your agents to a hub on another machine | `skillstation connect --hub <url> --token-file <file> [-g] [--host claude\|codex\|gemini] [--dry-run]` |
| Remove a connection | `skillstation connect --remove [--cache] [-g] [--host claude\|codex\|gemini] [--dry-run]` |
| See the state of the hub and this folder, and the next command | `skillstation status [--json\|--quiet]` |
| Check the hub and the connections | `skillstation doctor [--json]` |
| Repair what doctor can, including recording a skill folder made by hand in the hub as a draft | `skillstation doctor --fix [--dry-run]` |
| Open the web UI | `skillstation serve web [--addr <host:port>] [--no-open]` |
| Print the version | `skillstation --version` |
| Check for a new release | `skillstation update --check` |
| Update the `skillstation` program | `skillstation update [--version <v>]` |

`doctor` exits with `0` when everything is healthy, `1` when problems remain (a failed check of a hub on another machine counts, so a script can rely on the exit code) and `2` for an invalid request. `status` exits with `2` in a folder that is neither a hub nor a connected project, and prints the command that sets one up.

## Work with skills

| Task | Command |
|---|---|
| List skills with origin, 30-day uses and labels | `skillstation skill list [--state draft\|active\|deprecated\|archived] [--origin authored\|learned\|adopted\|forked] [--topic <t>] [--label <l>] [--search <text>] [--sort name\|added\|changed\|uses\|last-used]` |
| Read a skill in any state | `skillstation skill show <skill-id> [--verbose]` |
| Add a skill from GitHub, as a draft | `skillstation skill add <github-url> [--path <p>] [--skill <name>\|--all] [--ref <r>] [--edit-routing] [--yes]` |
| Start a new skill in this project: `SKILL.md`, and `skill.yaml` with the routing (nothing changes on the hub) | `skillstation skill create` in a terminal asks the questions; scripts use `skillstation skill create <skill-id> --name <n> --description <d> [--trigger <t>] [--not-for <n>] [--example <e>]` |
| Copy a skill into this project to edit and try it; agents in this project use the checkout until it closes | `skillstation skill checkout <skill-id>` |
| Bring the hub's newer version into a checked-out folder, keeping your own changes (nothing on the hub changes) | `skillstation skill pull <skill-id\|folder> [--accept <path>=hub\|project]...` |
| Bring a project folder back to the hub as one commit (a new folder becomes a draft); the commit closes the checkout | `skillstation skill commit <skill-id\|folder> [--collection <c>] [--runtime-file <yaml>] [--diff] [--yes] [--no-commit]` |
| Review a skill before you activate it | `skillstation skill review <skill-id> [--verbose]` |
| Review a skill another machine sent to this hub (terminal: apply, approve, activate) | `skillstation skill review <SUB-id> [--diff]`; drop it with `skillstation skill review <SUB-id> --discard` |
| Edit instructions quickly in your editor (to try changes first, use `checkout`) | `skillstation skill edit <skill-id> --editor` |
| List skills whose routing needs work (few examples, vague or colliding triggers), active ones first; ask the curator agent to fix them | `skillstation skill lint [<skill-id>] [--all] [--json]` |
| Change the description or routing fields | `skillstation skill edit <skill-id> [--description <d>] [--trigger <t>] [--not-for <n>] [--min-scope <s>] [--yes]` |
| Group skills with labels, or set what a skill is about | `skillstation skill edit <skill-id> [--label <l>]... [--topic <t>]... [--technology <t>]... [--yes]` (one empty value clears the list) |
| Add requests that should, or should not, pick the skill | `skillstation skill edit <skill-id> --example <e> --counter-example <e> [--yes]` |
| Add routing entries that usage suggests | `skillstation telemetry cases suggest`, then `skillstation skill edit <skill-id> --from-suggestion <id> [--diff] [--yes]` |
| Declare the tools a skill needs | `skillstation skill edit <skill-id> --runtime-file <yaml> [--yes]` |
| Approve content from someone else's repository (a person, in a terminal: the questions of `skill add`, `skill commit` and `skill review <SUB-id>`, or the web page **Waiting for review**) | `skillstation skill edit <skill-id> --approve-content <digest>` |
| Check this machine against a skill's needs | `skillstation skill doctor <skill-id>` |
| Store, remove or list a skill's secrets | `skillstation skill env set\|unset <skill-id> <KEY>`, `skillstation skill env list <skill-id>` |
| Apply a previewed change | `skillstation skill confirm <proposal-id>` |
| Activate, deprecate or archive a skill | `skillstation skill activate\|deprecate\|archive <skill-id> [--yes]` |

`skill checkout` copies the whole skill into `.agents/skills/<skill-id>/` of the project, the folder Codex, Gemini CLI, Antigravity, pi and omp read, writes a pointer for Claude Code in `.claude/skills/<skill-id>/SKILL.md`, adds both folders to the project's `.gitignore`, and prints the full path of the one folder to edit; agents in that project use the checkout, also through `skill_resolve` and `skill_get`, while every other project keeps the current version. `skill commit` shows every changed file, the routing it keeps from the folder's `skill.yaml`, what changed on the hub since the checkout, and the routing lint; for a skill from someone else's repository whose content changed, it approves the new content in the same step (a change to `skill.yaml` alone needs no approval unless it touches the runtime block). An applied commit closes the checkout (folder, Claude Code pointer, the hub's record) unless the folder holds files the commit left out; the **Checkouts** page of `skillstation serve web` discards one. Without a terminal that needs `--approve-content <digest>`, which only a person gives. `skill pull` brings what changed on the hub since the checkout into the folder: a file only the hub changed is updated, a file only you changed is kept, and a file you both changed is merged; when the merge cannot settle a file, a terminal asks once whether to take the hub's version, and without one each such file needs `--accept <path>=hub|project`. In a project connected to a hub on another machine, `checkout` fetches the active skill, `pull` fetches its newer version (merging file by file), and `commit` sends the folder to that hub's review queue (`Sent as SUB-…`); see [From another machine](curating-skills.md#from-another-machine). `skill add` takes GitHub addresses only; to add a skill from a folder on this machine, run `skill commit <folder>`. A bare id such as `skill commit pdf-to-epub` reads `.agents/skills/<id>/`, or `.claude/skills/<id>/` for a checkout an older release made there; when both hold a copy, `commit` and `pull` refuse and say what each folder holds ([Where a skill's files live](curating-skills.md#where-a-skills-files-live)). Any other folder is named by its path (`./pdf-to-epub`), and the preview prints the `Folder:` it read.

Routing flags for `create` and `edit`: `--operation`, `--trigger`, `--not-for`, `--min-scope`, `--example`, `--counter-example`. Activation needs real instructions, not the untouched starter text. It also needs a trigger, a `--not-for` entry (or `--rationale`) and `--min-scope`.

## Sources and learning

| Task | Command |
|---|---|
| Check sources for updates | `skillstation source check [--all-due\|--all\|<source-id>...]` |
| See which copied skills differ from upstream | `skillstation skill outdated [--check] [--all] [--exit-code]` |
| Show one skill's upstream and drift | `skillstation skill upstream <skill-id> [--check]` |
| Pull upstream changes into a skill | `skillstation skill update <skill-id> [--target <commit>] [--yes]` |
| List candidates and sources | `skillstation source list [--status <s>]` |
| Show one candidate or source | `skillstation source show <id>` |
| Watch a repository as a learning reference | `skillstation source watch <locator> --skill-id <skill-id> [--cadence daily\|weekly\|manual] [--yes]` |
| Stop watching a source | `skillstation source unwatch <source-id> [--yes]` |
| Link or unlink a learning reference | `skillstation source attach\|detach <source-id> --skill-id <skill-id> [--yes]` |
| Fill in source records for older skills | `skillstation source backfill [--skill <skill-id>] [--yes]` |
| Import skills from a repository as drafts | `skillstation source import <locator\|source-id> [--path <p>] [--skill <name>\|--all] [--yes]` |
| Note a repository to look at later | `skillstation source capture <locator> --reason <text>` |
| Decide on a candidate | `skillstation source triage <candidate-id> --decision accept\|defer\|reject\|import` |

`source check` records whether a source changed; it never edits your skills, and nothing runs in the background. Lessons have no command: start them from **Sources** in the web UI with **Distill with Curator Agent**. Then read them in the skill's **Distill** tab ([Sources and learning](sources-and-learning.md#learn-from-other-repositories)).

## Maintain the hub

`skillstation doctor` runs these checks and `skillstation doctor --fix` the repairs most people need; the `maintain` commands are the single steps.

| Task | Command |
|---|---|
| See uncommitted hub changes | `skillstation maintain diff` |
| Check the hub files | `skillstation maintain validate` |
| Check only the files staged in Git, for a pre-commit hook | `skillstation maintain validate --staged` |
| Check the routing examples of every active skill, not only the changed ones | `skillstation maintain validate --full` |
| Rebuild the search index | `skillstation maintain rebuild [--verbose]` |
| Preview or apply a file format migration (layout 6 moves each skill's metadata into `skill.yaml` and `.meta/hub.yaml`, and stops on any value it would lose) | `skillstation maintain migrate [--to <version>] [--yes]` |

Skillstation commits each change you confirm, with only its own files, and never pushes. These commands cover what happens outside that, such as files you edited by hand in the hub folder. See [Curating skills](curating-skills.md#what-gets-committed-and-who-gets-it).

## Serve other machines

| Task | Command |
|---|---|
| Let agents on other machines use this hub | `skillstation serve [--addr <host:port>[,<host:port>...]] [--allow-host <name>]` |
| Keep the server on this machine, for an SSH tunnel | `skillstation serve --addr 127.0.0.1:7420` |
| Keep that server running as a systemd or launchd service | `skillstation serve [--addr <host:port>] --install-service` |
| Make a one-time code that gives another machine the hub token (terminal only) | `skillstation token invite [--addr <host:port>[,<host:port>...]] [--strict-pairing]` |
| Connect a project to a hub on another machine with that code | `skillstation connect --hub <url> --invite <code> [--token-file <path>] [--strict-pairing]` |
| Show the token those agents need (terminal only) | `skillstation token show` |
| Replace that token (terminal only) | `skillstation token rotate [--dry-run]` |
| Remove the skills this machine fetched from another hub | `skillstation cache clear [--hub <url>] [--dry-run]` |

`serve` and `serve web` listen on every network interface by default, over plain HTTP. [Use one hub from several machines](remote-hub-guide.md) explains when to narrow that with `--addr`.

## Routing and usage

These are developer commands; `skillstation help --dev` lists them.

| Task | Command |
|---|---|
| Ask which skill fits a request | `skillstation resolve --request <request.json>` |
| Measure routing quality across skill examples | `skillstation eval routing [--no-skill <file>] [--policy <file>]` |
| Check the local usage record | `skillstation telemetry health` |
| See how often skills are recommended and used | `skillstation telemetry funnel [--since <YYYY-MM-DD\|Nd>] [--skill <skill-id>]` |
| Stop a routing suggestion from coming back | `skillstation telemetry cases dismiss <suggestion-id>` |
| Import tool use from Claude Code transcripts | `skillstation telemetry import-transcripts --project <dir>` |
| Preview or write a sanitized export | `skillstation telemetry export [--output <file> --yes]` |

Your agent starts Skillstation by itself over stdio; you never start that by hand. The usage record stays on your machine unless you export it. Developer tools such as `eval run` and `resolution replay` are described in `skillstation help eval` and `skillstation help resolution`.

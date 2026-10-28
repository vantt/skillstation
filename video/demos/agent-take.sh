#!/usr/bin/env bash
# Set up a real Claude Code take for the intro's "agent asks the hub" shot.
#
#   video/demos/agent-take.sh up      build the demo project and Claude config (needs happy-path.sh to have run)
#   video/demos/agent-take.sh reset   put the project back: one staged change, nothing committed
#   video/demos/agent-take.sh shell   open a shell in the project with the demo Claude config
#   video/demos/agent-take.sh sync    after a take: copy the login back to ~/.claude if Claude Code refreshed it
#   video/demos/agent-take.sh fresh   curator takes only: a new empty hub (init, connect -g) and a manual-mode Claude config, then `up`
#
# Claude Code keeps your real login but nothing else of yours: CLAUDE_CONFIG_DIR points at a fresh folder that only
# holds a copy of the credentials file, so none of your hooks, plugins, memory or MCP servers load. The project's own
# .mcp.json serves the demo hub (the one happy-path.sh made) through the sandbox binary.
set -eu

real_home=$HOME # fresh() points HOME into the sandbox; the login always comes from the real one

root=${DEMO_AGENT_TAKE:-/tmp/demo}
sandbox=${DEMO_SANDBOX:-/tmp/skillstation-demo}
cfg=$root/claude-config
# DEMO_AGENT_CURATOR=1: use the sandbox's own ~/.claude as the Claude config. `skillstation connect -g` put the curator
# skill there as a pointer to ../../../.agents/skills/, so the pointer only resolves when the config sits in that home.
[ "${DEMO_AGENT_CURATOR:-0}" = 1 ] && cfg=$sandbox/home/.claude
project=$root/billing-service

[ -x "$sandbox/bin/skillstation" ] && [ -d "$sandbox/home/skillstation-hub" ] ||
	{ echo "agent-take.sh: run video/demos/happy-path.sh first (no demo hub in $sandbox)" >&2; exit 1; }

reset_project() {
	rm -rf "$project"
	mkdir -p "$project/src" "$project/.claude"
	cd "$project"
	git init -q -b main
	git config user.name "Demo User"
	git config user.email "demo@example.com"
	cat >src/worker.py <<'PY'
def charge(order):
    return gateway.charge(order.total)
PY
	git add -A && git commit -q -m "Add payment worker"
	cat >src/worker.py <<'PY'
MAX_RETRIES = 3


def charge(order):
    for attempt in range(MAX_RETRIES):
        result = gateway.charge(order.total)
        if result.ok:
            return result
    raise PaymentFailed(order.id)
PY
	git add src/worker.py
	cat >.mcp.json <<JSON
{
  "mcpServers": {
    "skillstation": {
      "type": "stdio",
      "command": "env",
      "args": [
        "HOME=$sandbox/home",
        "XDG_CONFIG_HOME=$sandbox/home/.config",
        "XDG_DATA_HOME=$sandbox/home/.local/share",
        "XDG_STATE_HOME=$sandbox/home/.local/state",
        "XDG_CACHE_HOME=$sandbox/home/.cache",
        "SKILLSTATION_USER_CONFIG_DIR=$sandbox/home/.config/skillstation",
        "$sandbox/bin/skillstation", "mcp", "serve", "--profile", "runtime",
        "--workspace", "$sandbox/home/skillstation-hub"
      ]
    }
  }
}
JSON
	# No approval prompts on camera: the agent may use the hub's tools and read the repo; it may not commit.
	cat >.claude/settings.local.json <<JSON
{
  "permissions": {
    "allow": ["mcp__skillstation", "Bash(skillstation:*)", "Bash(git diff:*)", "Bash(git status:*)", "Bash(git log:*)", "Bash(ls:*)", "Bash(cat:*)", "Bash(sh:*)"],
    "defaultMode": "default",
    "deny": ["Bash(git commit:*)", "Bash(git push:*)"],
    "additionalDirectories": ["$sandbox/home/.agents"]
  },
  "enableAllProjectMcpServers": true,
  "attribution": { "commit": "", "pr": "" },
  "promptSuggestionEnabled": false,
  "env": { "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION": "0" }
}
JSON
	# Curator take: the hub's own `connect -g` (user level) is all the agent should rely on, as in the tutorial. Project
	# files written above would make `doctor` report them as out of date.
	if [ "${DEMO_AGENT_CURATOR:-0}" = 1 ]; then rm -f .mcp.json .claude/settings.local.json; fi
}

up() {
	mkdir -p "$cfg"
	# A copy, not a link: Claude Code replaces the file when it refreshes the login, which would turn a link into a stale
	# copy of its own and leave the real file with a refresh token that no longer works. `sync` brings the fresh one back.
	rm -f "$cfg/.credentials.json"
	cp -p "$real_home/.claude/.credentials.json" "$cfg/.credentials.json"
	reset_project
	cat >"$cfg/.claude.json" <<JSON
{
  "hasCompletedOnboarding": true,
  "theme": "dark",
  "projects": { "$project": { "hasTrustDialogAccepted": true, "hasCompletedProjectOnboarding": true } }
}
JSON
	echo "project: $project"
	echo "config:  $cfg"
}

fresh() {
	[ "${DEMO_AGENT_CURATOR:-0}" = 1 ] || { echo "agent-take.sh: fresh needs DEMO_AGENT_CURATOR=1" >&2; exit 2; }
	export HOME=$sandbox/home XDG_CONFIG_HOME=$sandbox/home/.config XDG_DATA_HOME=$sandbox/home/.local/share \
		XDG_STATE_HOME=$sandbox/home/.local/state XDG_CACHE_HOME=$sandbox/home/.cache \
		SKILLSTATION_USER_CONFIG_DIR=$sandbox/home/.config/skillstation PATH=$sandbox/bin:$PATH
	rm -rf "$HOME/skillstation-hub" "$HOME/.agents" "$XDG_CONFIG_HOME/skillstation" "$XDG_DATA_HOME/skillstation" \
		"$XDG_STATE_HOME/skillstation" "$XDG_CACHE_HOME/skillstation"
	mkdir -p "$HOME/skillstation-hub" && (cd "$HOME/skillstation-hub" && skillstation init >/dev/null)
	(cd "$HOME/skillstation-hub" && skillstation connect -g >/dev/null)
	up
	# Manual mode, so the film shows the same Claude Code mode as the other takes and no auto-mode notice. The agent may
	# read, write and run what the curator skill needs inside the sandbox; commits to the hub still ask (connect -g rules).
	python3 - "$cfg/settings.json" <<'PY'
import json, sys
path = sys.argv[1]
settings = json.load(open(path))
perm = settings.setdefault("permissions", {})
perm["defaultMode"] = "default"
allow = perm.setdefault("allow", [])
for rule in ["Skill", "Read", "Write", "Edit", "Bash(cat:*)", "Bash(ls:*)", "Bash(cd:*)", "Bash(python3:*)", "Bash(git diff:*)", "Bash(git status:*)"]:
    if rule not in allow:
        allow.append(rule)
settings["promptSuggestionEnabled"] = False
json.dump(settings, open(path, "w"), indent=2)
PY
}

case "${1:-}" in
	up) up ;;
	fresh) fresh ;;
	reset) reset_project ;;
	sync)
		[ "$cfg/.credentials.json" -nt "$real_home/.claude/.credentials.json" ] &&
			cp -p "$cfg/.credentials.json" "$real_home/.claude/.credentials.json" && echo "login synced back to ~/.claude" ||
			echo "login unchanged" ;;
	shell)
		cd "$project"
		exec env CLAUDE_CONFIG_DIR="$cfg" CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=0 \
			bash --rcfile <(printf 'PS1=%q\n' 'dev@billing:\w$ ') -i ;;
	*) echo "usage: agent-take.sh up|fresh|reset|sync|shell" >&2; exit 2 ;;
esac

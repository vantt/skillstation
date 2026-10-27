#!/usr/bin/env bash
# Set up a real Claude Code take for the intro's "agent asks the hub" shot.
#
#   video/demos/agent-take.sh up      build the demo project and Claude config (needs happy-path.sh to have run)
#   video/demos/agent-take.sh reset   put the project back: one staged change, nothing committed
#   video/demos/agent-take.sh shell   open a shell in the project with the demo Claude config
#
# Claude Code keeps your real login but nothing else of yours: CLAUDE_CONFIG_DIR points at a fresh folder that only
# links the credentials file, so none of your hooks, plugins, memory or MCP servers load. The project's own
# .mcp.json serves the demo hub (the one happy-path.sh made) through the sandbox binary.
set -eu

root=${DEMO_AGENT_TAKE:-/tmp/demo}
sandbox=${DEMO_SANDBOX:-/tmp/skillstation-demo}
cfg=$root/claude-config
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
	cat >.claude/settings.local.json <<'JSON'
{
  "permissions": {
    "allow": ["mcp__skillstation", "Bash(git diff:*)", "Bash(git status:*)", "Bash(git log:*)", "Bash(ls:*)", "Bash(cat:*)", "Bash(sh:*)"],
    "deny": ["Bash(git commit:*)", "Bash(git push:*)"]
  },
  "enableAllProjectMcpServers": true,
  "attribution": { "commit": "", "pr": "" },
  "promptSuggestionEnabled": false,
  "env": { "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION": "0" }
}
JSON
}

up() {
	mkdir -p "$cfg"
	ln -sf "$HOME/.claude/.credentials.json" "$cfg/.credentials.json"
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

case "${1:-}" in
	up) up ;;
	reset) reset_project ;;
	shell)
		cd "$project"
		exec env CLAUDE_CONFIG_DIR="$cfg" CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=0 \
			bash --rcfile <(printf 'PS1=%q\n' 'dev@billing:\w$ ') -i ;;
	*) echo "usage: agent-take.sh up|reset|shell" >&2; exit 2 ;;
esac

# Scene: a real Claude Code (HOME and XDG_* point into the sandbox, or `skillstation` would read the owner's real hub) is asked "curate my Skillstation" on a new hub (tutorial scene 6).
# Needs a fresh hub with only init and connect -g (see video/demos/README.md "Agent takes"), then
#   DEMO_AGENT_CURATOR=1 DEMO_AGENT_TAKE=/tmp/demo-curate DEMO_SANDBOX=/tmp/skillstation-take-curate video/demos/agent-take.sh up
Output video/recordings/tapes/10-agent-curate.mp4
Source video/demos/tapes/settings.tape

Hide
Sleep 1500ms
Type "unset $(env | grep -o '^CLAUDE[A-Z_]*') && export HOME=/tmp/skillstation-take-curate/home XDG_CONFIG_HOME=/tmp/skillstation-take-curate/home/.config XDG_DATA_HOME=/tmp/skillstation-take-curate/home/.local/share XDG_STATE_HOME=/tmp/skillstation-take-curate/home/.local/state XDG_CACHE_HOME=/tmp/skillstation-take-curate/home/.cache SKILLSTATION_USER_CONFIG_DIR=/tmp/skillstation-take-curate/home/.config/skillstation PATH=/tmp/skillstation-take-curate/bin:$PATH CLAUDE_CONFIG_DIR=/tmp/skillstation-take-curate/home/.claude CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=0 PS1='$ ' && cd /tmp/demo-curate/billing-service && clear"
Enter
Sleep 500ms
Show

Type "claude"
Sleep 400ms
Enter
Wait+Screen@30s /Claude Code v/
Sleep 2s

Type "curate my Skillstation"
Sleep 800ms
Enter
Wait+Screen@120s /done [0-9]+:[0-9]+ [AP]M/
Sleep 5s
Screenshot video/recordings/tapes/10-agent-curate-end.png
Sleep 2s

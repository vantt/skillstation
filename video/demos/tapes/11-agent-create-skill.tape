# Scene: a real Claude Code is asked to create the commit-style skill; it drafts the folder in the project and previews the hub commit (tutorial scene 7). Same sandbox and setup as tape 10; nothing reaches the hub until tape 12.
# Needs a fresh hub with only init and connect -g (see video/demos/README.md "Agent takes"), then
#   DEMO_AGENT_CURATOR=1 DEMO_AGENT_TAKE=/tmp/demo-curate DEMO_SANDBOX=/tmp/skillstation-take-curate video/demos/agent-take.sh up
Output video/recordings/tapes/11-agent-create-skill.mp4
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

Type "Create a skill called commit-style. It writes Git commit messages in our team style: a short imperative summary, a blank line, then why the change was made, and a last line: Checked-by: skillstation-demo. Keep your answer brief."
Sleep 800ms
Enter
Wait+Screen@120s /done [0-9]+:[0-9]+ [AP]M/
Sleep 5s
Screenshot video/recordings/tapes/11-agent-create-skill-end.png
Sleep 2s

# Scene: a real Claude Code uses the commit-style skill from the hub (tutorial scenes 1 and 7c).
# Run `video/demos/agent-take.sh up` (after happy-path.sh) first, `agent-take.sh sync` after.
Output video/recordings/tapes/09-agent-commit-style.mp4
Source video/demos/tapes/settings.tape

Hide
Sleep 1500ms
Type "unset $(env | grep -o '^CLAUDE[A-Z_]*') && export CLAUDE_CONFIG_DIR=/tmp/demo/claude-config CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=0 PS1='$ ' && cd /tmp/demo/billing-service && clear"
Enter
Sleep 500ms
Show

Type "claude"
Sleep 400ms
Enter
Wait+Screen@30s /Claude Code v/
Sleep 2s

Type "Use my commit-style skill to write a commit message for my staged changes. Reply with just the message."
Sleep 800ms
Enter
Wait+Screen@90s /Checked-by/
Sleep 4s
Screenshot video/recordings/tapes/09-agent-commit-style-end.png
Sleep 2s

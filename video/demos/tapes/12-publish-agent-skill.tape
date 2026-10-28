# Scene: publish the skill the agent drafted in tape 11, in a terminal (tutorial scene 7). Run right after tape 11, in
# the same sandbox and project: the commit adds it to the hub as a draft, the terminal offers to activate it in the
# same command, and the checkout closes.
Output video/recordings/tapes/12-publish-agent-skill.mp4
Source video/demos/tapes/settings.tape

Hide
Sleep 1500ms
Type "unset $(env | grep -o '^CLAUDE[A-Z_]*') && export HOME=/tmp/skillstation-take-curate/home XDG_CONFIG_HOME=/tmp/skillstation-take-curate/home/.config XDG_DATA_HOME=/tmp/skillstation-take-curate/home/.local/share XDG_STATE_HOME=/tmp/skillstation-take-curate/home/.local/state XDG_CACHE_HOME=/tmp/skillstation-take-curate/home/.cache SKILLSTATION_USER_CONFIG_DIR=/tmp/skillstation-take-curate/home/.config/skillstation PATH=/tmp/skillstation-take-curate/bin:$PATH PS1='$ ' && cd /tmp/demo-curate/billing-service && clear"
Enter
Sleep 500ms
Show

Type "skillstation skill commit commit-style"
Sleep 400ms
Enter
Wait+Screen@15s /as a draft\?/
Sleep 3s
Type "y"
Enter
Wait+Screen@15s /Activate commit-style now/
Sleep 1500ms
Type "y"
Enter
Wait+Screen@15s /checkout is closed/
Sleep 2s
Type "skillstation skill list --state active"
Enter
Sleep 4s
Screenshot video/recordings/tapes/12-publish-agent-skill-end.png
Sleep 1s

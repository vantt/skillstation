# Scene: Create your first skill (docs/curating-skills.md): create, write the instructions, publish.
# Needs 03-connect.tape's sandbox. The instructions are written off camera from the same fixtures as
# happy-path.sh; the storyboard shows that step as an editor shot.
Output video/recordings/tapes/04-first-skill.mp4
Source video/demos/tapes/settings.tape
# Machine label "Hub computer": drawn by Remotion (VHS has no window title setting).

Hide
# Let bash finish starting before typing, or the first hidden line can land on screen.
Sleep 1500ms
Type ". video/demos/sandbox.sh /tmp/skillstation-tape-a 2>/dev/null && . $DEMO_DIR/tapes/helpers.sh && cd ~/my-project && clear"
Enter
# Wait for the demo prompt: sourcing a fresh sandbox can first download the release.
Wait+Line@120s /^\$\s*$/
Show

Type "skillstation skill create commit-style"
Sleep 400ms
Enter
Wait+Screen@15s /What does this skill do\?/
Type "Write Git commit messages in our team style"
Enter
Wait+Screen@10s /When should an agent use it\?/
Type "write a commit message"
Enter
Sleep 500ms
Enter
Wait+Screen@10s /When should it not\?/
Type "writing release notes"
Enter
Sleep 500ms
Enter
Wait+Screen@10s /example requests/
Type "write a commit message for my staged changes"
Enter
Sleep 500ms
Enter
Wait+Screen@15s /When ready/
Sleep 4s

Hide
Type "video_write_instructions && clear"
Enter
Sleep 800ms
Show

Type "cat .claude/skills/commit-style/SKILL.md"
Enter
Sleep 7s

Hide
Type "clear"
Enter
Show

Type "skillstation skill commit commit-style"
Sleep 400ms
Enter
Sleep 3s
Sleep 2s
Type "y"
Enter
Sleep 2s
Sleep 1500ms
Type "y"
Enter
Sleep 2s
Sleep 3s
Type "skillstation skill list --state active"
Enter
Sleep 5s

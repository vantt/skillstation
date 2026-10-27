# Scene: Connect your agent and check (README steps 3 and 4). Needs 02-create-hub.tape's sandbox.
# doctor runs in a project folder: inside ~/skillstation-hub it adds "not used here" lines that confuse.
Output video/recordings/tapes/03-connect.mp4
Source video/demos/tapes/settings.tape
# Machine label "Hub computer": drawn by Remotion (VHS has no window title setting).

Hide
# Let bash finish starting before typing, or the first hidden line can land on screen.
Sleep 1500ms
Type ". video/demos/sandbox.sh /tmp/skillstation-tape-a 2>/dev/null && cd ~/skillstation-hub && clear"
Enter
# Wait for the demo prompt: sourcing a fresh sandbox can first download the release.
Wait+Line@120s /^\$\s*$/
Show

Type "skillstation connect -g"
Sleep 400ms
Enter
Wait+Screen@30s /curate my Skillstation/
Sleep 7s

Hide
Type "mkdir -p ~/my-project && cd ~/my-project && git init -q && clear"
Enter
Sleep 500ms
Show

Type "cd ~/my-project && skillstation doctor"
Sleep 400ms
Enter
Sleep 3s
Sleep 6s

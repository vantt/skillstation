# Scene: Create your hub (README step 2): Git identity, then init.
Output video/recordings/tapes/02-create-hub.mp4
Source video/demos/tapes/settings.tape
# Machine label "Hub computer": drawn by Remotion (VHS has no window title setting).

Hide
# Let bash finish starting before typing, or the first hidden line can land on screen.
Sleep 1500ms
Type "DEMO_FRESH=1 . video/demos/sandbox.sh /tmp/skillstation-tape-a 2>/dev/null && git config --global --unset user.name; git config --global --unset user.email; clear"
Enter
# Wait for the demo prompt: sourcing a fresh sandbox can first download the release.
Wait+Line@120s /^\$\s*$/
Show

Type "git config user.email"
Enter
Sleep 2s
Type 'git config --global user.name "Demo User"'
Enter
Sleep 600ms
Type 'git config --global user.email "demo@example.com"'
Enter
Sleep 1500ms
Type "mkdir -p ~/skillstation-hub && cd ~/skillstation-hub && skillstation init"
Sleep 400ms
Enter
Wait+Screen@30s /Next:/
Sleep 5s

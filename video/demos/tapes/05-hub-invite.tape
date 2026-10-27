# Scene: Hub and agent on different computers, hub side (README): a one-time code for the agent
# computer. render.sh prepares the hub in /tmp/skillstation-tape-b and starts `skillstation serve` there
# off camera (`serve --install-service` would install a real background service).
Output video/recordings/tapes/05-hub-invite.mp4
Output video/recordings/tapes/05-hub-invite.txt
Source video/demos/tapes/settings.tape
# Machine label "Hub computer": drawn by Remotion (VHS has no window title setting).

Hide
# Let bash finish starting before typing, or the first hidden line can land on screen.
Sleep 1500ms
Type ". video/demos/sandbox.sh /tmp/skillstation-tape-b 2>/dev/null && cd ~/skillstation-hub && clear"
Enter
# Wait for the demo prompt: sourcing a fresh sandbox can first download the release.
Wait+Line@120s /^\$\s*$/
Show

Type "skillstation token invite"
Sleep 400ms
Enter
Sleep 3s
Sleep 7s

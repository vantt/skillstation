# Scene: the same take as 07, landscape, for the 16:9 README cut of the intro. Run video/demos/agent-take.sh reset first.
# 1200x960 window, shown at 1:1 next to the captions in a 1920x1080 frame.
Output video/recordings/tapes/08-intro-agent-wide.mp4
Set Shell bash
Set Width 1200
Set Height 960
Set FontSize 24
Set LineHeight 1.25
Set Padding 28
Set Framerate 30
Set TypingSpeed 38ms
Set CursorBlink false
Set Theme {"name":"skillstation-p3","background":"#140c38","foreground":"#f0ece2","cursor":"#9ff07f","selection":"#3d2c94","black":"#140c38","red":"#ff6fc8","green":"#9ff07f","yellow":"#ffd166","blue":"#8f86ff","magenta":"#ff6fc8","cyan":"#7fe0e0","white":"#f0ece2","brightBlack":"#b6addf","brightRed":"#ff6fc8","brightGreen":"#9ff07f","brightYellow":"#ffd166","brightBlue":"#8f86ff","brightMagenta":"#ff6fc8","brightCyan":"#7fe0e0","brightWhite":"#ffffff"}

Hide
Sleep 1500ms
Type "export CLAUDE_CONFIG_DIR=/tmp/demo/claude-config CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=0 PS1='$ ' && cd /tmp/demo/billing-service && clear"
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
Sleep 20s
Screenshot video/recordings/tapes/08-intro-agent-wide-end.png
Sleep 2s

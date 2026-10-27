# Scene: a real Claude Code asks the hub for a skill (intro film, shot 5). Run video/demos/agent-take.sh reset first.
# Narrow and tall on purpose: the film is 9:16, so the terminal is shown at about 1:1 on a 1080 px wide frame.
Output video/recordings/tapes/07-intro-agent.mp4
Set Shell bash
Set Width 1000
Set Height 1180
Set FontSize 27
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
Screenshot video/recordings/tapes/07-intro-agent-end.png
Sleep 2s

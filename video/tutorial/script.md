# Skillstation intro video: script and storyboard

Brief: [plans/261016-1000-intro-video/film-2-tutorial.md](../../plans/261016-1000-intro-video/film-2-tutorial.md). Status: **approved at
CHECKPOINT 1 (2026-10-16)**; next are the voice-over and the edit. Changes to the scenes go back to the owner.
Scenes 6 and 10 were revised for the `skill-curator` skill at the owner's request (2026-10-23).

- Version filmed: `skillstation` 0.9.0, the latest release (2026-10-27). `happy-path.sh` passes all 13 steps on it. 0.9.0 changed where a new skill lives (`.agents/skills/<id>/`, with `skill.yaml` for routing); scenes 7 and 9 follow it.
- Voice-over: English, edge-tts (`en-US-AndrewNeural`). Subtitles: English and Vietnamese, from the narration
  below.
- Structure: **hub first, then agents**, the same order as the README Quickstart. Everyone does Install, Create and
  Connect. One more chapter covers an agent on another computer: it reuses the hub and only changes how the agent
  connects.
- Length of the current cut: 4:11. A scene lasts as long as its shots in `edit.json`, or longer when its voice needs it.

## Sources for each shot

| Source | Made by | Output |
|---|---|---|
| `T01`–`T04`, `T05`, `T06` | VHS tapes in `video/demos/tapes/`, run by `render.sh` | `video/recordings/tapes/NN-*.mp4` |
| `W-edit`, `W-approve` | `video/demos/web/run.sh` (Playwright) | `video/recordings/web/web-*.webm` |
| `OBS-1`, `OBS-2` | Claude Code in a demo config; see "Claude Code shots" below | `video/recordings/obs/*.mp4` |
| `MG` | Remotion motion graphics | built while editing |

Each tape writes what this version really prints, in a sandbox. The tapes leave out only the steps the video shows
another way; each tape's header comment names them.

## Chapters (YouTube)

| Start | Chapter |
|---|---|
| 0:00 | Why Skillstation |
| 0:32 | Install |
| 0:46 | Create your hub |
| 1:04 | Connect your agent |
| 1:23 | Your first skill |
| 2:25 | Edit in the browser |
| 2:47 | An agent on another computer |
| 3:57 | Tips and links |

Times of the current cut (4:11), from `audio/timeline.json`; they change when `edit.json` or the narration changes.

---

## Scene 1. The problem (0:00, about 8 s)

| | |
|---|---|
| Screen | `MG`: three repo tiles, each with its own copy of `commit-style/` at a different version (v3 green, v2 yellow, v1 pink), then the question "The same skill, copied three times. Which one is right?" |
| On-screen text | **The same skill, copied three times. Which one is right?** |
| Narration | "Your agent skills end up copied into every project and every computer, and the copies drift apart." |
| Note | The film opens on the problem, so that the idea in scene 2 is the answer to it. |

## Scene 2. The idea (0:08, about 20 s)

| | |
|---|---|
| Screen | **(a)** `MG`: one box "Your hub: a Git repository you own", with Claude Code, Codex, Gemini CLI and "Another computer" connected to it. **(b)** `OBS-2` (take 09): the agent answers "use my commit-style skill ..." and its commit message ends with `Checked-by: skillstation-demo`. **(c)** `MG`: three short cards one after another. |
| On-screen text | (c): **Your library, in Git** · **Review before agents get it** · **Nothing to install per project** |
| Narration | "Skillstation keeps them in one Git repository you own. Your agents connect to it over MCP and pull the right skill, on demand. You collect, edit and approve skills before any agent gets them." |
| Note | At (a), Codex and Gemini CLI show as lighter boxes and a small footnote says "Verified end to end with Claude Code." Do not say they are fully supported. |

## Scene 3. Install (0:45, about 20 s): chapter "Install"

| | |
|---|---|
| Screen | `T01`: the `curl ... install.sh \| sh` line, the install output, then `skillstation --version`. |
| Step title | **Step 1 · Install** |
| On-screen text | Lower third: "You need Git and an agent, such as Claude Code." Small card: "Windows: PowerShell command in the README." |
| Highlight | Box around `Installed skillstation <version> to ...`, then around `skillstation version: <version>`. |
| Narration | "In this film you install it, create your hub, and write your first skill. You need Git and an agent such as Claude Code. Install Skillstation with one line. Open a new terminal, and check the version." |
| Note | `T01` cuts the opening of a new terminal off camera. The cosign line ("Signature not checked ...") is four lines long: speed it up or crop it. Do not hide it. |

## Scene 4. Create your hub (1:05, about 25 s): chapter "Create your hub"

| | |
|---|---|
| Screen | `T02`: `git config user.email` prints nothing; set the name and email; `mkdir -p ~/skillstation-hub && cd ~/skillstation-hub && skillstation init`. |
| Step title | **Step 2 · Create your hub** |
| Machine label | "Hub computer" (shown from here on whenever the chapter on another computer could make it unclear; here it can stay small). |
| Highlight | `Hub created at ...` and `This is now the hub of this machine`. |
| Narration | "The hub is a folder in its own Git repository. Git records every change with your name and email, so set them once if you have not. Then create the hub." |
| Note | Since 0.7.2, `init` ends with "Next: connect your agents in every project ... `skillstation connect -g`", the same as step 3: keep it. |

## Scene 5. Connect your agent and check (1:30, about 35 s): chapter "Connect your agent"

| | |
|---|---|
| Screen | `T03`, part 1: `skillstation connect -g`. Part 2: `cd ~/my-project && skillstation doctor`. |
| Step title | **Step 3 · Connect your agent**, then **Step 4 · Check** |
| Highlight | Part 1: the `Claude Code: connected` line, then `Next: restart your agent ...`. Part 2: the last line, `Healthy.` |
| On-screen text | Over the `WARNING:` lines: "Expected: each agent still asks you for permission." Small card: "Claude Code asks you to trust `Bash(skillstation:*)` the first time. Allow it." |
| Narration | "Connect your agents once, for every project. The warning in the middle is expected. Then run doctor from any project. Healthy means you are ready." |
| Note | `doctor` runs in a project folder, not in `~/skillstation-hub`: inside the hub folder it adds "not used here" lines. Before filming, make sure no hub server of yours listens on port 7420, or doctor adds a "Network server" notice. |

## Scene 6. Ask your agent (2:05, about 25 s)

| | |
|---|---|
| Screen | **(a)** `OBS-1`: Claude Code opened in `~/my-project`; the person types **curate my Skillstation**; the agent loads the `skill-curator` skill, runs `skillstation version`, then `skillstation status --json`, and suggests creating a first skill. **(b)** `MG`: three short requests in speech bubbles, one after another: "which skills are weak?" · "publish commit-style" · "ok, apply it". |
| On-screen text | **Step 5 · Ask your agent** · the prompt in a large speech bubble. Small card at (a): "Or type `/skill-curator`." Small card at (b): "The agent shows a preview and asks before it changes your hub." |
| Narration | "Restart your agent, open any project, and say: curate my Skillstation. The agent loads its curator skill once for this chat. A new hub has no skills yet, so it suggests your first one. From then on, short requests are enough, and the agent asks before it changes your hub." |
| Note | `connect` installs the `skill-curator` skill, but the agent loads it only when asked. In ordinary work it sees the three runtime tools (`skill_resolve`, `skill_get`, `skill_feedback`) and one line describing the curator. That is why the request comes first. The curator works through the CLI (`skillstation … --json`), so no extra MCP server is loaded. The bubbles in (b) map to `skill lint`, a `skill commit` preview, and applying that preview. Do not show token or size numbers on screen: the brief allows no figures without a published way to measure them. `skill-curator` is in 0.9.0 (0.8.0 still named it `system-curator`). |

## Scene 7. Your first skill (2:20, about 47 s): chapter "Your first skill"

| | |
|---|---|
| Screen | **(a)** `OBS-3` (tape 11): in Claude Code the person asks "Create a skill called commit-style. It writes Git commit messages in our team style: a short imperative summary, a blank line, then why the change was made, and a last line: Checked-by: skillstation-demo."; the agent loads `skill-curator`, runs `skillstation skill create`, writes `SKILL.md` and `skill.yaml`, previews the hub commit and says nothing is on the hub yet. **(b)** `OBS-2` (take 09): "use my commit-style skill to write a commit message for my staged changes"; the message ends with `Checked-by: skillstation-demo`. **(c)** `T12` (tape 12): `skillstation skill commit commit-style`, answer `y` twice, then `skillstation skill list --state active`. |
| Step titles | **Ask for a skill** · **Try it** · **Publish it** |
| Highlight | (a) the prompt and the line saying the preview changed nothing on the hub; (b) the `Checked-by` line in the agent's answer; (c) `Activate commit-style now? [y/N] y`, the line `The checkout is closed`, and the `active` row. |
| Narration | "A skill is a folder with instructions your agent follows. Ask your agent to create one: say what it does, and give it a name. It writes the skill in your project and shows what it would add to the hub. Nothing is published yet. Try it with your agent: here, every commit message ends with a check line. When it works, publish it with one command. Say yes to add it, and yes to activate it. Every agent that uses your hub gets it now, in every project, and your project's copy is closed." |
| Note | Why the agent drafts and the person publishes: in a terminal, `skill commit` offers to add the skill and to activate it in one command; an agent has no terminal, so it can only add a draft and activation is a second approval (measured in `plans/261027-0830-video-redo/curate-flow-test.md`). The agent's own wording at the end of (a) offers to commit it for you; that is also valid, the film shows the terminal path. The agent picks its own routing words, so tape 11 must be read word by word after every take. Small card at (a): "Prefer to type it yourself? `skillstation skill create`." Claude Code reads a pointer file in `.claude/skills/commit-style/`; it is not shown and not edited. |

## Scene 8. Edit in the browser (3:05, about 25 s): chapter "Edit in the browser"

| | |
|---|---|
| Screen | Command card `cd ~/skillstation-hub && skillstation serve web`, then `W-edit`: **Skills**, `commit-style`, **Files**, `SKILL.md`, **Edit**, a typed line, **Review**, **Commit to the hub**, ending on "Committed .... This machine's agents get it now." |
| On-screen text | "Anyone with the link can change your skills. Keep it on this machine: see the Web UI guide." |
| Narration | "Prefer clicking? Start the web UI from your hub folder. Open a skill, edit a file, review the change, and commit it to the hub." |
| Note | The command is shown as a card, not as a terminal recording. `serve web` prints a link that holds a session token, so it must never be filmed. `W-edit` has no address bar. |

## Scene 9. An agent on another computer (3:30, about 60 s): chapter "An agent on another computer"

| | |
|---|---|
| Screen | **(a)** `MG`: two computers on one network, "Hub computer" (dark terminal) and "Agent computer" (light terminal). **(b)** Hub side: card with `cd ~/skillstation-hub && skillstation serve --install-service`, then `T05`: `skillstation token invite` and the connect line it prints. **(c)** Agent side: `T06`: the pasted line with `-g`, `skillstation doctor`, `Healthy.` **(d)** `T06` end: `skillstation skill commit release-checklist` and `Sent as SUB-...`. **(e)** Hub side: `W-approve`: **Waiting for review**, open each file, **Approve**. |
| Step titles | **On the hub computer** · **On the agent computer** · **Send a skill for review** · **Approve it on the hub** |
| Machine label | Always on, top left: "Hub computer" or "Agent computer", in the same colour as that terminal. For (c) and (d), split screen with the invite on the left. |
| Highlight | (b) the `skillstation connect --hub ... --invite ...` line; (c) the pasted line and `Hub check: reachable, token accepted`; (d) `Sent as SUB-...`; (e) the **Approve** button. |
| Narration | "Your hub can serve agents on other computers, on the same network or over Tailscale. On the hub computer, start the server as a background service and make a one-time code. On the other computer, install Skillstation and paste the line it printed. Doctor checks the connection. That computer keeps no skills: its agents ask the hub. Curation stays on the hub. A skill made on the other computer is sent for review, and you approve it on the hub." |
| Note | `serve --install-service` is a card: it installs a real background service, so `render.sh` runs `skillstation serve` off camera. The code works once and expires in 10 minutes. On Windows, `skillstation serve` runs in its own terminal. `T05`/`T06` show the LAN address of the filming computer (`192.168.x.x`), a private address that is not reachable from the internet: kept as is (owner, 2026-10-16). The two computers are two isolated sandboxes on one computer, talking over real HTTP on its LAN address; that is the final footage (owner, 2026-10-16). The promotion plan's launch gate, a real person following the README on two real computers, is a separate product check. |

## Scene 10. Tips and links (4:30, about 15 s): chapter "Tips and links"

| | |
|---|---|
| Screen | `MG`: two command cards, then the end card. |
| On-screen text | `skillstation status`: "what to do next" · `skillstation doctor`: "what is wrong, and one fix" · "New chat? Ask to curate again, or type `/skill-curator`." · end card: `github.com/vantt/skillstation` · "Docs: Using Skillstation, Curating skills, One hub from several machines" · "Questions and ideas: GitHub Discussions" · "Verified with Claude Code". |
| Narration | "Lost? skillstation status tells you the next step, and doctor tells you what is wrong. In a new chat, ask to curate again. The code and the guides are on GitHub. Tell us what works, and what does not." |
| Note | A new session does not load the curator by itself. A long chat that gets compacted can lose the curator's details; typing `/skill-curator` again reloads it. Only the first point fits on screen. |

---

## Claude Code shots

Film them in a demo account or a fresh user, not in your daily setup: no real projects, chats or notifications on
screen. Terminal and Claude Code at 1920x1080, font large enough for a phone. Prompt `$ `.

1. **OBS-1, "curate my Skillstation"** (scene 6). Before you start, run `video/demos/tapes/render.sh create-hub
   connect` and open Claude Code in `~/my-project` of that sandbox (`. video/demos/sandbox.sh /tmp/skillstation-tape-a`,
   then `cd ~/my-project && claude`). Approve the `Bash(skillstation:*)` trust dialog on camera. Type `curate my
   Skillstation`; the agent should load `skill-curator` and run `skillstation version` first. Keep filming until it
   suggests creating a first skill.
2. **OBS-2, "use /commit-style"** (scenes 1 and 7c). After `render.sh first-skill`, in the same sandbox, change a
   file and stage only that file (`git add <file>`). Ask: `use /commit-style to write a commit message for my staged
   changes`. Keep filming until the message ending in `Checked-by: skillstation-demo` is on screen.

Claude Code needs to be signed in, and the sandbox has no credentials. Use a throwaway login, or film OBS-1 and OBS-2
on a separate machine with the same commands.

## Checks before the final cut

- Every command on screen matches `skillstation help <command>` of the filmed version.
- No real path, hostname, user name, e-mail, token or session link on screen. The private LAN address in `T05` and
  `T06` is allowed; check every `OBS` shot.
- Each chapter works alone: a viewer who jumps to "An agent on another computer" sees which earlier steps it reuses
  (Install, Create your hub, Check).

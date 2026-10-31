# Research: static security audit for agent skills (`skillstation skill audit <id>`)

Date: 2026-10-31. Scope: industry scanners/registries, rule categories, severity, FP handling, output, exit codes.

## Bottom line

1. Build a local, deterministic, regex + file-fact scanner (Go RE2, no network, no LLM). Treat it as triage that feeds the
   existing `review_required` gate. It does not replace human approval. Every vendor says "no findings is not the same as safe".
2. Scan everything: SKILL.md prose, code fences, `references/`, and `scripts/`. Real malware is hybrid. Snyk found 91% of
   confirmed malicious skills combined prompt injection with code, and ClawHavoc hid its payload in fake "prerequisites" prose.
3. Weight rules by correlation, not by single tokens. Reading a secret path plus a network sink in the same file is critical.
   Either one alone is lower. This is how Snyk claims a 0% false-positive rate on the top-100 skills.sh skills at critical level.
4. Use 5 severities (critical/high/medium/low/info), `--format text|json|sarif`, `--fail-on <level>` (default high), and
   exit codes 0/1/2. Bind suppressions to a content fingerprint so a later content change revives the finding.

## Landscape (ranked by usefulness to us)

| Source | What it is | Takeaway for us | Credibility |
|---|---|---|---|
| Cisco AI Defense skill-scanner (OSS, Python) | Static YAML+YARA, pipeline taint, Python AST dataflow, optional LLM/VirusTotal | Closest model. Flags `--fail-on-severity`, formats json/sarif/markdown/html, policy presets. **Rules alone: 7.7% recall at HIGH, 4.0% FPR** on its own benchmark | High (primary README) |
| Snyk ToxicSkills + agent-scan | Audit of 3,984 skills: 36.8% had an issue, 13.4% critical, 76 confirmed malicious. agent-scan risk taxonomy | Best category list (below). agent-scan **sends skill content to a remote API**, which we should avoid | High (primary blog + repo docs) |
| ClawHub + VirusTotal Code Insight | Bundle SHA-256 lookup, LLM verdict benign/suspicious/malicious, daily rescans | Pin results to a content hash and re-audit when content changes. States that signatures miss natural-language attacks | High (OpenClaw primary post) |
| OWASP Agentic Skills Top 10 (AST01-10, 2026, incubator) | Risk list: Malicious Skills, Supply Chain, Over-Privileged, Insecure Metadata, Untrusted External Instructions, Weak Isolation, Update Drift, Poor Scanning, No Governance, Cross-Platform Reuse | Map each rule to an AST id. AST08 explicitly warns that signature scanning is weak | Medium (incubator, still changing) |
| OWASP LLM Top 10 2025 | LLM01 Prompt Injection, LLM02 Sensitive Info Disclosure, LLM03 Supply Chain, LLM06 Excessive Agency | Secondary mapping labels | High |
| Anthropic docs | "Use Skills only from trusted sources." Audit all bundled files for unexpected network calls and file access. External URLs are risky. `allowed-tools` pre-approves tools and **is not gated by workspace trust**. `` !`cmd` `` runs at render without prompting (but permission rules still apply) | Justifies the allowed-tools and `` !` `` rules | High (primary) |
| Invariant Labs tool poisoning | Hidden `<IMPORTANT>` blocks telling the model to read `~/.ssh/id_rsa` / `~/.cursor/mcp.json`, "do not mention" to the user. Rug pulls change content after approval | Phrases to detect. Hash pinning against drift | High (primary) |
| Koi ClawHavoc (via secondary press) | 341/2,857 ClawHub skills malicious, 335 one campaign. Fake prerequisite installs led to AMOS stealer | The prerequisite-install lure is the top real-world vector | Medium (Koi page now redirects. Figures from search summaries, **unverified first-hand**) |

## Prioritized rule list (P1 = ship first)

Notes on the table:
- Regexes are RE2-compatible, case-insensitive unless noted. "ctx" is where the match sits: prose, code fence, script, reference, or frontmatter.
- `|` inside regex cells is a literal regex alternation. Only the column separators are table syntax.

| P | id | Matches | Sev | Example check |
|---|---|---|---|---|
| 1 | `rce.pipe-to-shell` | Download piped to an interpreter | critical | `\b(curl\|wget\|iwr\|Invoke-WebRequest)\b[^\n]*\|\s*(sudo\s+)?(ba\|z\|da\|k)?sh\b` and `\|\s*(python3?\|node\|perl\|ruby\|iex)\b` after a fetch |
| 1 | `rce.decode-exec` | Decode-then-execute | critical | `base64\s+(-d\|--decode\|-D)[^\n]*\|\s*\w*sh\b`; `\b(eval\|exec)\s*\(\s*(atob\|Buffer\.from\|base64\.b64decode\|codecs\.decode)` |
| 1 | `exfil.secret-to-network` | Secret path AND network sink in the same file | critical | Secret paths: `~/\.ssh\|id_(rsa\|ed25519)\|\.aws/credentials\|\.config/gcloud\|\.kube/config\|\.docker/config\.json\|\.npmrc\|\.pypirc\|\.netrc\|\.git-credentials\|(^\|/)\.env\b\|Keychain\|Login Data`. Sinks: `curl\|wget\|requests\.(post\|put)\|fetch\(\|http\.request\|\bnc\b\|scp` |
| 1 | `inject.unicode-tags` | Invisible Unicode Tag chars (ASCII smuggling) | critical | `[\x{E0000}-\x{E007F}]` |
| 1 | `inject.bidi` | Bidi overrides (Trojan Source) | high | `[\x{202A}-\x{202E}\x{2066}-\x{2069}]` |
| 1 | `inject.zero-width` | Zero-width chars outside a leading BOM | medium (high in frontmatter) | `[\x{200B}-\x{200D}\x{2060}\x{FEFF}]` |
| 1 | `inject.override-phrase` | Instruction override or concealment | high | `ignore (all )?(previous\|prior\|above) (instructions\|rules)\|disregard (the )?(system\|previous)\|developer mode\|do not (tell\|mention\|inform) the user\|without (asking\|telling) the user\|<IMPORTANT>` |
| 1 | `fs.symlink` | Any symlink. Critical if it resolves outside the skill | high/critical | `lstat`. Note: `skill add` already rejects unsafe symlinks, so reuse that check |
| 1 | `fs.binary` | ELF/Mach-O/PE magic, `.so/.dylib/.dll/.exe`, `.pyc`, nested archives | high | Magic bytes `7F454C46`, `FEEDFACE/FEEDFACF/CAFEBABE`, `4D5A`, `PK\x03\x04` |
| 1 | `deps.remote-install` | Install from a URL, VCS, or custom index | high | `pip3? install [^\n]*(git\+\|https?://)\|--(extra-)?index-url\|npm (i\|install) [^\n]*(https?://\|git\+\|github:)\|--registry` |
| 1 | `deps.lifecycle-script` | `preinstall/install/postinstall` in a bundled package.json | high | JSON parse `scripts` keys |
| 1 | `secret.hardcoded` | Literal credentials | high | `AKIA[0-9A-Z]{16}\|gh[pousr]_[A-Za-z0-9]{36}\|sk-[A-Za-z0-9_-]{20,}\|xox[baprs]-\|-----BEGIN [A-Z ]*PRIVATE KEY-----` |
| 2 | `exfil.known-sink-host` | Paste, webhook, and tunnel hosts | high | `webhook\.site\|requestbin\|pipedream\.net\|ngrok(-free)?\.(io\|app)\|discord(app)?\.com/api/webhooks\|api\.telegram\.org/bot\|pastebin\.com\|transfer\.sh\|\.oast\.\|interact\.sh` |
| 2 | `exfil.secret-read` | Secret path without a sink, or `env`/`printenv`/`os.environ` dumps | medium | Same secret-path list, standalone |
| 2 | `dl.protected-archive` | Password-protected archive, or download+chmod+run | high | `unzip\s+-P\|7z\s+x\s+[^\n]*-p\|chmod\s+\+x[^\n]*&&\s*\./` |
| 2 | `persist.autostart` | cron, services, shell rc, git hooks, ssh keys | high | `crontab\|/etc/cron\|systemctl\s+(--user\s+)?enable\|launchctl\s+load\|LaunchAgents\|>>?\s*~?/?\.?(bashrc\|zshrc\|profile\|bash_profile)\|\.git/hooks/\|core\.hooksPath\|authorized_keys` |
| 2 | `destructive.command` | Destructive file, git, or DB operations | high (abs/home path), medium (relative) | `\brm\s+-[a-z]*r[a-z]*f?[a-z]*\s+(/\|~\|\$HOME\|\*)\|git\s+push\s+[^\n]*(-f\b\|--force)\|git\s+reset\s+--hard\|mkfs\|dd\s+if=[^\n]*of=/dev/\|DROP\s+(DATABASE\|TABLE)` |
| 2 | `privesc.command` | sudo, world-writable or setuid | medium (high when prose tells the agent to run it) | `\bsudo\b\|chmod\s+(-R\s+)?(777\|666\|a\+w\|[ug]\+s)\|chown\s+root` |
| 2 | `perm.allowed-tools-broad` | Unscoped or broad `allowed-tools` | high: `Bash` unscoped / `Bash(*)` / `Bash(curl:*)`; medium: `WebFetch` + Bash | Parse frontmatter. Anthropic: grants are not trust-gated |
| 2 | `perm.render-shell` | `` !`cmd` `` dynamic shell in SKILL.md | high | `` (?m)!`[^`\n]+` `` outside code fences |
| 2 | `obf.blob` | Long base64/hex blobs in text or scripts | medium (high when near an exec/sink) | `[A-Za-z0-9+/]{200,}={0,2}`, `(\\x[0-9a-f]{2}){20,}`, `String\.fromCharCode\(`, chained `chr(` |
| 2 | `inject.html-comment` | HTML comment in markdown with imperative text, a URL, or a command | medium | `<!--[\s\S]*?-->`, then re-run P1 rules on the body and escalate |
| 3 | `fs.size` | File > 2 MiB or total > 8 MiB (current `internal/source` limits). SKILL.md > 500 lines | medium/low | stat |
| 3 | `fs.exec-bit` | Executable bit on non-script or unexpected files | low | mode & 0111 |
| 3 | `net.mutable-instructions` | Raw IPs, URL shorteners, `raw.githubusercontent.com/.../(main\|master)/` instruction fetch (AST05/07) | medium | `https?://\d{1,3}(\.\d{1,3}){3}\|bit\.ly\|tinyurl` |
| 3 | `deps.typosquat` | Install name within edit distance 1 of a popular package | medium | Embedded top-N list. Needs a data source (open question) |
| 3 | `meta.invalid` | Reserved words `anthropic`/`claude` in name, XML tags in description, description over 1,024 chars | low/medium | Anthropic field rules |

Existing code to reuse: `internal/skillruntime/hints.go` already detects install prose and setup headings. Report plain
`pip/npm install` as `info` there instead of duplicating the logic in a new rule (DRY).

## Severity, false positives, output, exit codes

- **Severity model.** Each rule has a base level. Context adjusts it:
  - +1 for prose that instructs the agent ("run", "execute", setup/prerequisites headings) or for frontmatter;
  - +1 when the match correlates with a sink in the same file;
  - -1 inside `references/` code fences that only document a pattern.
  The skill verdict is the maximum severity. SARIF `security-severity` per GitHub: critical 9.0-10, high 7.0-8.9, medium 4.0-6.9, low 0.1-3.9. Snyk uses scores 1000/600/300/100 for the same four tiers.
- **False positives.** Security-teaching skills and installers trigger rules legitimately. Lessons from the sources:
  - Correlate before calling something critical (Snyk).
  - Offer presets (Cisco: strict/balanced/low-noise).
  - Allow appeals (ClawHub).
  - Recommended suppression design: a hub-side entry with `{rule, path, fingerprint, reason}`, where fingerprint = hash of rule + path + normalized matched line. It expires automatically when content changes. This guards against rug pulls and update drift (Invariant, AST07). Never use inline magic comments, because a malicious author could write them too.
- **Output.** Default text grouped by severity. `--format json` with a schema in `schemas/` (project convention for public contracts). `--format sarif` must be 2.1.0 with stable `ruleId`, at least one `locations[]`, `partialFingerprints` (same fingerprint as above), and rule `properties.tags:["security"]` plus `security-severity`.
- **Exit codes (proposal; neither Cisco nor Snyk publishes a numeric table).** `0` means no finding at or above `--fail-on`. `1` means findings at or above the threshold. `2` means a usage or operational error (unknown id, unreadable file). Cisco and Snyk both expose a severity-threshold or `--ci` flag and return non-zero.

## Lessons learned

1. Static rules catch commodity payloads but miss natural-language attacks. Cisco measured 7.7% recall at HIGH for rules alone, and both OpenClaw and AST08 say the same. The UI and docs must say the audit is advisory.
2. The dominant real vector is the "prerequisites" lure: prose telling the agent to download and run something (ClawHavoc, Snyk). Prose rules belong in P1.
3. Pin to content. ClawHub rescans daily, and Invariant documented rug pulls. Store the audit result against the existing content digest and re-audit when the digest changes.
4. Keep the scan local. agent-scan uploads content and VirusTotal uploads bundles. A local scanner fits skillstation's offline Git-canonical model and avoids leaking private skills.
5. `allowed-tools` matters even in untrusted folders (Anthropic). Treat broad grants as a finding, not as metadata.

## Limitations

- Did not read Snyk's full GitHub technical report, the NVIDIA SkillSpector docs, or the Cisco per-rule YAML. Rule regexes above are my synthesis and need a corpus test.
- Koi/ClawHavoc details come from search summaries only. THN returned 403 and koi.ai redirects. The 824 and 1,184 later counts are unverified.
- Unicode Tag range U+E0000-E007F is from Unicode knowledge. The Embrace The Red post names the block but not the range. Bidi code points are from general knowledge; trojansource.codes does not list them.
- Did not cover semantic checks (description vs. behavior mismatch, `direct_money_access`), which need an LLM. They are out of scope for a static command.

## Unresolved questions

1. Should audit findings at or above high automatically set a `review_required` reason code, or stay informational in V1?
2. Typosquat data source: embed a top-N npm/PyPI list (size, staleness), or skip until OSV integration?
3. Where should suppressions live in the hub (per-skill metadata vs. a hub-level file)? This affects the Git-canonical contract.

## Sources (read)

- https://snyk.io/blog/toxicskills-malicious-ai-agent-skills-clawhub
- https://github.com/snyk/agent-scan and https://raw.githubusercontent.com/snyk/agent-scan/main/docs/risks.md
- https://github.com/cisco-ai-defense/skill-scanner (README)
- https://www.openclaw.ai/blog/virustotal-partnership
- https://owasp.github.io/www-project-agentic-skills-top-10/top10.html
- https://genai.owasp.org/llm-top-10/
- https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview (Security considerations)
- https://code.claude.com/docs/en/skills (allowed-tools, shell injection)
- https://invariantlabs.ai/blog/mcp-security-notification-tool-poisoning-attacks
- https://embracethered.com/blog/posts/2024/hiding-and-finding-text-with-unicode-tags/
- https://trojansource.codes/
- https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning
- Search summaries only (unverified first-hand): https://koi.ai/blog/clawhavoc-341-malicious-clawedbot-skills-found-by-the-bot-they-were-targeting, https://thehackernews.com/2026/02/researchers-find-341-malicious-clawhub.html

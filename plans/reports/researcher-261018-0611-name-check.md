# Name check: "Skill Hub" / `skillhub-mcp` / `skillhub` binary

Date checked: 2026-10-18 (UTC) for every row below unless noted. This is a read-only check; nothing was registered or posted. It is not legal advice.

## Bottom line

The legal risk is **low to medium** and the confusion risk is **high**, so keeping "Skill Hub (skillhub-mcp)" for a public OSS launch is overall **medium-high risk**.

- **Legal:** the only live US registration is SKILLHUB for resume-writing services (class 35). I found no US mark in software classes 9 or 42. "Skill hub" is close to descriptive, which makes it hard for anyone to enforce, including you.
- **Confusion:** the name is saturated in exactly this niche. iFlytek's `iflytek/skillhub` (5.2k stars) is an agent-skill registry. Tencent runs SkillHub (skillhub.cn), an agent-skill store with a `skillhub install` CLI. skillhub.club ships an MCP server. On PyPI, `skillhub-mcp` is already "an MCP server that exposes Claude-style skills to any MCP client", which is this project's own one-line pitch. At least six npm packages install a `skillhub` executable. GitHub has 1,838 repos with "skillhub" in the name.

**Recommendation:** rename before launch if the launch is meant to build a recognisable brand. A rename costs least now, while the repo has 0 stars. If the name stays, apply the mitigations in section 5. Even then, expect people to mistake this project for iFlytek's or Tencent's, and expect search results to bury it.

## 1. Trademarks

| Register | Query | Result | Classes / owner | Source |
|---|---|---|---|---|
| USPTO (tmsearch backend, exact phrase on wordmark and pseudo-text) | `skillhub`, `skill hub`, `skill-hub` | 1 live mark: **SKILLHUB**, serial 97365144, registered 2023-05-23 | IC 035 only ("resume preparation; resume writing..."); Skillhub FZ-LLC (UAE). No class 9 or 42. | https://tmsearch.uspto.gov (record 97365144) |
| USPTO | `skills hub` | 1 live mark: ELECTRIC REVOLUTION SKILLS HUB, reg. 2026-07-28 | IC 035; CU Services Ltd (UK). Not relevant. | https://tmsearch.uspto.gov |
| USPTO | `skillshub`, `skilhub`, `skills*hub*` | 0 hits | none | https://tmsearch.uspto.gov |
| UK IPO (via web search) | "SKILLS HUB" | UK00004380742, filed 2026-04-29 | Classes 9, 35, 41, 42; owner not captured. Different word ("Skills") but software classes. | https://www.ipo.gov.uk/t-tmj/tm-journals/2026-021/UK00004380742.html |
| EUIPO / TMview | `skillhub` | **Not verified.** tmdn.org and euipo.europa.eu refused connections from this environment and from the fetch tool. Web search surfaced no EU SKILLHUB word mark. | unknown | https://www.tmdn.org/tmview/ , https://euipo.europa.eu/eSearch/ |
| WIPO Global Brand DB | `skillhub` | **Not verified.** The API rejected unauthenticated queries (HTTP 400), and the UI is JS-only. | unknown | https://branddb.wipo.int/ |
| CNIPA (China) | none | Not checked. Tencent and iFlytek both use "SkillHub" commercially, so Chinese filings are plausible. | unknown | none |

## 2. Package registries

| Registry | Name | Status | What it is | Source |
|---|---|---|---|---|
| npm | `skillhub` | **Taken** (airano, v0.4.1, created 2026-01-28, 1,107 downloads/month) | CLI for managing agent skills; installs `skillhub` bin | https://www.npmjs.com/package/skillhub |
| npm | `skillhub-mcp` | **Taken** (artuntan, v0.6.0, 2026-03-09, 100 downloads/month) | MCP server that recommends skills; installs **both `skillhub` and `skillhub-mcp` bins** | https://www.npmjs.com/package/skillhub-mcp |
| npm | `skill-hub` | Free (404) | none | https://registry.npmjs.org/skill-hub |
| npm | other `skillhub` bins | `@astron-team/skillhub` (iFlytek, 4,276 downloads/month), `@uzolab/skillhub`, `@want-team/skillhub`, `@micar-tsp/skillhub` all install a `skillhub` bin. `@skill-hub/mcp-server` (skillhub.club) installs a `skillhub-mcp` bin. | **PATH collisions** with this Go binary | https://registry.npmjs.org/-/v1/search?text=skillhub |
| PyPI | `skillhub` | **Taken** (v1k22, v0.1.0, 2025-12-28) | "Package manager for AI agent workflows" | https://pypi.org/project/skillhub/ |
| PyPI | `skillhub-mcp` | **Taken** (214140846 / "Eleanor Berger", v1.0.5, 2026-01-18) | "MCP server that exposes Claude-style skills to any MCP client", which matches this project's function | https://pypi.org/project/skillhub-mcp/ |
| PyPI | `skill-hub` | Free (404) | none | https://pypi.org/pypi/skill-hub/json |
| Homebrew core | formula `skillhub` / cask `skillhub` | Free (404 for both) | none | https://formulae.brew.sh/api/formula/skillhub.json |
| crates.io | `skillhub` | Taken (aios-rs, 2026-04-20, 41 downloads) | "CLI for SkillHub - AI Agent Skill Registry" | https://crates.io/crates/skillhub |
| Go | `github.com/vantt/skillhub-mcp` | Yours. It is namespaced, so there is no module-path conflict, and it resolves on proxy.golang.org (HTTP 200). Other modules build `cmd/skillhub` binaries (CassianFlorin/skill-hub, actionlab-ai/aisphere-hub, hitzhangjie/skillhub). | Binary-name overlap only | https://pkg.go.dev/search?q=skillhub |

## 3. Domains (RDAP)

| Domain | Status | Registered / expires | Registrar / what is served | Source |
|---|---|---|---|---|
| skillhub.com | Taken | 2008-01-07 / 2027-01-07 | GoDaddy; "Skillhub Freelance Platform" | rdap.verisign.com |
| skillhub.dev | Taken | 2020-02-02 / 2027-02-02 | GoDaddy; parked `/lander` page, possibly for sale | pubapi.registry.google/rdap |
| skillhub.io | Taken | 2022-05-31 / 2027-05-31 | GoDaddy; parked `/lander` page, possibly for sale | rdap.identitydigital.services |
| skillhub.sh | Taken | 2026-02-15 / 2027-02-15 | Cloudflare; no page served | rdap.identitydigital.services |
| skillhub.app | Taken | 2025-10-24 / 2028-10-24 | MarkMonitor (a corporate brand-protection registrar); no page served | pubapi.registry.google/rdap |
| skillhub.ai | Taken | 2023-05-03 / 2027-05-03 | Namecheap; "Learn Coding with AI Tutors" | rdap.identitydigital.services |
| skillhub.club | Taken (in use) | not queried | skillhub.club agent-skills finder | https://skillhub.club |
| skillhub.cn | Taken (in use) | not queried | Tencent SkillHub | https://skillhub.cn |
| skillhub-mcp.dev | **Free** (RDAP 404) | none | none | pubapi.registry.google/rdap |
| skillhub-mcp.com | **Free** (RDAP 404) | none | none | rdap.verisign.com |
| skillhub-mcp.io | **Free** (RDAP 404) | none | none | rdap.identitydigital.services |
| skillhubmcp.dev | **Free** (RDAP 404) | none | none | pubapi.registry.google/rdap |

An RDAP 404 means the domain is not registered. It does not guarantee the domain can be bought, because some names are premium or reserved; confirm at a registrar before buying.

## 4. Existing products and projects named SkillHub

| Project | What it is | Popularity (2026-10-18) | Same space? | Confusion risk | Source |
|---|---|---|---|---|---|
| iflytek/skillhub (Astron SkillHub) | Self-hosted enterprise agent-skill registry with RBAC and versioning; Apache-2.0; npm CLI `@astron-team/skillhub` | 5,240 stars, pushed 2026-10-16; trending; derivatives exist (`@want-team/skillhub`) | Yes: skill registry for agents | **High.** It owns "SkillHub" search results and is also self-hosted. | https://github.com/iflytek/skillhub , https://skill.xfyun.cn |
| Tencent SkillHub | Agent-skills community and store for Chinese users (launched around 2026-03), about 13k skills, `skillhub install` CLI | Big-company backing; GitHub org repo has 37 stars | Yes | **High** in the Chinese and Asian market; likely trademark interest. Tencent has the money and motive to defend the name in China. | https://skillhub.cn , https://github.com/Tencent/skillhub , https://toolnavs.com/en/tool/1154 |
| skillhub.club | Agent-skills finder, desktop app, and **MCP server** (`@skill-hub/mcp-server`, bin `skillhub-mcp`) | skillhub-desktop has 599 stars | Yes, including MCP | **High.** It shares the `skillhub-mcp` bin name. | https://skillhub.club , https://github.com/skillhub-club/mcp-server |
| PyPI `skillhub-mcp` (214140846/skillhub-mcp) | Exposes Claude-style skills over MCP | 1 star; dormant since 2026-01 | Yes, the same function | Medium. It is small but has the identical name and pitch. | https://pypi.org/project/skillhub-mcp/ |
| npm `skillhub-mcp` (artuntan) | MCP skill recommender | 1 star, 100 downloads/month | Yes | Medium. Same package name and `skillhub` bin. | https://github.com/artuntan/skillhub-mcp |
| airano-ir/skillhub (npm `skillhub`) | Agent-skills marketplace and CLI | 19 stars, 1.1k downloads/month | Yes | Medium | https://github.com/airano-ir/skillhub |
| saker-ai/skillhub | "AI Agent skills hub", MIT | 7 stars, last push 2026-08-19 | Yes | Low | https://github.com/saker-ai/skillhub |
| SkillPort (gotalab/skillport) | Agent skills served via CLI or MCP ("manage once, serve anywhere"). Different name, but the closest functional competitor. | 414 stars | Yes | No name risk; it is a positioning competitor | https://github.com/gotalab/skillport |
| openJiuwen-ai/skillhub, BiglionX/SkillHub, cow-skill-hub, and others | Further skill registries and markets | 30 to 50 stars each | Yes | Adds to the noise | GitHub search |
| Skillhub FZ-LLC; skillhub.com; skillhub.ai | Resume services, freelance platform, coding tutor | n/a | No | Low | see sections 1 and 3 |

## 5. Mitigations and options, ranked

1. **Rename before launch (preferred).** The repo has 0 stars and was created 2026-09-28, so a rename costs almost nothing now and much more later. Choose a distinctive coined word, not "skill" + generic noun, and run this same checklist on it: USPTO classes 9 and 42, npm, PyPI, Homebrew, .dev/.com, and a GitHub search. The product strings (MCP server name `skillhub`, `SKILLHUB_*` env vars, `skillhub` CLI) would all change, so scope this as a separate plan.
2. **If you keep the name**, these cheap steps reduce damage:
   - Register **skillhub-mcp.dev** (and **skillhub-mcp.com** if you want the .com). Both are free per RDAP. Use the .dev one as the canonical docs URL.
   - Ship a Homebrew tap formula `skillhub` (core is free), and say plainly in the README that the binary is the Go `skillhub` from vantt/skillhub-mcp.
   - Consider renaming the **binary** to something unique, such as `skillhub-mcp` or a short coined alias. Six npm packages and Tencent's installer already put a `skillhub` on users' PATH, and a collision there breaks MCP client configs silently.
   - Do not publish to npm or PyPI under `skillhub-mcp`; both names are taken. If you need a wrapper there, use a scoped npm name such as `@vantt/skillhub-mcp`.
   - Add a one-line disambiguation in the README ("not affiliated with iFlytek SkillHub, Tencent SkillHub, or skillhub.club") and always lead with the qualifier "Skill Hub for MCP, Git-owned skills".
   - Avoid filing your own trademark on "Skill Hub". It is likely too descriptive to register and would invite opposition.
   - Do not buy skillhub.dev or skillhub.io from parking resale. The bare name is not defensible, so the cost is unlikely to pay off.

## Limitations

- EUIPO, TMview and WIPO Global Brand DB could not be queried from this environment (connection refused, or an API that requires a key). CNIPA was not checked. Tencent or iFlytek filings in the EU, China, or under the Madrid system remain **unknown**, and they are the most likely place for a software-class SKILLHUB mark. Run those three searches by hand before launch: https://www.tmdn.org/tmview/ , https://branddb.wipo.int/ , and https://sbj.cnipa.gov.cn/.
- The USPTO tmsearch index may lag new filings by several weeks.
- Star and download counts are a single snapshot. Web-search summaries about Tencent SkillHub come from aggregator sites (medium credibility), although skillhub.cn itself was confirmed live.
- I did not evaluate alternative names.

## Unresolved questions

1. Is the launch goal brand-building (which argues for a rename) or simply publishing a useful tool (where keeping the name with mitigations is acceptable)?
2. If you rename, is changing the binary, the MCP server name, and the `SKILLHUB_*` env vars in scope before launch?
3. Should someone run the EUIPO, WIPO and CNIPA checks by hand, or is US-only clearance enough for an MIT OSS project?

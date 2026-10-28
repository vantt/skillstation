---
title: "Promote Skillstation: get the first real users"
description: "A staged promotion plan for the repository: make the repo page convert visitors, list it where people look for MCP and agent-skill tools, launch it with one honest demo, then keep the momentum with fast feedback. Every outward post waits for the owner's go."
status: pending
priority: P3
effort: 12h
branch: main
tags: [promotion, marketing, docs, github]
blockedBy: []
blocks: []
created: 2026-10-14
---

# Promote Skillstation

## Outcome, constraints, non-goals

- **Outcome.** In the first 30 days after launch: people who fit the audience can understand the project in one screen,
  try it in two minutes from the README, and tell us what broke. Targets to review at day 30, not promises: 100 stars,
  10 people who installed and connected (counted from issues, Discussions replies and release downloads), 5 pieces of
  usable feedback, 3 listings in directories or awesome lists.
- **Audience.** People who already use an agent (Claude Code first, then Codex and Gemini CLI) and keep skills in
  `~/.claude/skills` or in per-project folders, especially those who use more than one computer or more than one agent.
- **Positioning (owner's two selling points, 2026-10-13).** (a) Skills served over MCP: nothing to install per project,
  agents pull skills on demand. (b) Because the library is central, you can collect skills from anywhere, edit them and
  tune them to your own criteria, with a Git repository you own. Lead with (b), supported by (a): skills over MCP is
  becoming a feature of other tools, owning and tuning your library is harder to copy. Tagline in use: "Your skills in
  one place. Pulled by any agent, on demand."
- **Constraints.**
  - Nothing is posted, submitted or published without the owner's explicit go for that item. Drafts are written in this
    repository; the owner sends them, or says "post it" for a specific item.
  - No fake traffic: no bought stars, no sock-puppet accounts, no unmarked self-promotion. Say who made it.
  - Claims must be true and checkable: no benchmark or "saves N% context" number unless it has been measured and the
    method is linked.
  - Respect each community's self-promotion rules; read them before posting.
  - No secrets, no private hub content, no real tokens or personal addresses in screenshots, recordings or posts.
- **Non-goals.** Paid ads, a marketing site, a logo (the product was renamed from Skill Hub to Skillstation in v0.8.0, see the [rename plan](../../archive/plans/261019-0425-rename-to-skillstation/plan.md)),
  a Discord server of our own, press outreach.

## Honest starting point

- The repository is public with **0 stars, no topics, no homepage, Discussions off**. The description is "Skills over MCP.
  A hub for curating skills and serving skills over mcp."
- Latest published release: v0.6.6. v0.6.8 is being released (it adds the invite flow and the shorter README).
- The earlier name, `skillhub`, collided with `iflytek/skillhub`, `saker-ai/skillhub`, SkillPort and others (see the PRD reference survey), which is why the product was renamed to Skillstation. A new name is not yet searchable, so discovery still has to come from topics, directory listings and posts.
- Claude Code is the only verified host; Codex and Gemini CLI are not verified end to end. Promotion must say so.
- The two-computer setup has not been proven on two real machines by anyone yet (open item in the observation backlog).
  Do not launch until one person has run the README steps on two real computers.
- Trademark, npm, PyPI and domain checks for the name were deferred by the owner; revisit before any push beyond
  directories (phase 3).

## Phases

| # | Phase | Output | Owner |
|---|-------|--------|-------|
| 1 | [Repository page](phase-01-repository-page.md) | Description, topics, social preview, Discussions, issue templates, demo asset, FAQ | me prepares, owner applies GitHub settings |
| 2 | [Directories and lists](phase-02-directories-and-lists.md) | Listings and awesome-list pull requests | me drafts, owner submits |
| 3 | [Launch](phase-03-launch.md) | One launch post and one demo, posted in 2 or 3 places | me drafts, owner posts |
| 4 | [Momentum and measurement](phase-04-momentum-and-measurement.md) | Feedback loop, releases, day-30 review | owner and me |

Phases 1 and 2 can run in parallel; phase 3 starts only after phase 1 is done and the two-computer check has passed.

## Gate before any launch (phase 3 does not start until all are true)

- [ ] The latest release installs on Linux, macOS and Windows with the README's install command (the release workflow's
      smoke installs pass).
- [ ] Someone has followed the README quickstart on a fresh machine: one computer, and two computers.
- [ ] README states what is verified (Claude Code) and what is not.
- [ ] Topics, description and social preview are set; Discussions are on; an issue template exists.
- [ ] The owner has decided about the name checks (trademark, npm, PyPI, domain) and recorded it here.

## Risks

| Risk | Effect | Mitigation |
|---|---|---|
| Launch while a basic step is broken | First impression is a failure | The gate above; one real two-computer run first |
| People know the old name or confuse the product with another | Traffic leaks | Write "Skillstation (formerly Skill Hub)" where old links exist; lead with the repo link |
| Over-claiming | Loss of trust | Only verified claims; say what is not verified |
| Reads as spam in a community | Post removed, reputation cost | Read the rules, post where it is on topic, disclose authorship, answer every reply |
| Support load after a good launch | Burnout, slow replies | Issue templates, a FAQ, a response-time promise of "within two days", not "instant" |
| Security finding goes public | Harm to early users | The invite and web UI defaults are permissive on purpose; the README says where and how to tighten (`--strict-pairing`, loopback) |

## Success measures (review at day 30; record in phase 4)

Stars, release downloads, unique cloners, issues opened and closed, Discussions threads, directory listings that went
live, and the sentence people use to describe the project when they write about it (does it match the positioning?).

## Owner decisions (2026-10-18)

Recorded in [launch strategy](../reports/launch-strategy-261018-0611-beta-then-public.md), which adds a closed beta
(5 to 10 people, exit criteria include two people on two real machines) before phase 3.

- Controlled beta first, public launch after.
- Channels under the owner's name: Reddit, X, LinkedIn, Vietnamese Facebook/Discord groups. No Show HN in this round.
- No fixed date: launch when the gate passes.
- Name checks (trademark, npm, PyPI, domain) run before launch; this is now part of the gate.
- 2026-10-19: rename to Skillstation before launch (clean break): [rename plan](../../archive/plans/261019-0425-rename-to-skillstation/plan.md).

## Open questions for the owner (answered 2026-10-18, see above)

1. Which communities do you already belong to or are willing to post in under your own name?
2. Do you want the post to go out under your name or the project's? (A personal name usually gets more trust.)
3. Is there a deadline, such as a talk or a release milestone, that the launch should line up with?
4. Are you willing to do a recorded demo, or should the demo be a scripted terminal recording (asciinema or a GIF)?

## Status (2026-10-27)

Not started. Checked against code, tests and git history on 2026-10-27; `make check` and CI were green for the repository at the latest commit, but the web end-to-end suite was not re-run for this review.

- Phase 1 repository page: not done. The GitHub description is still the old text, topics are empty, Discussions are off, and there is no issue template, FAQ or demo media.
- Phases 2 to 4 (directories, launch, momentum): not started; no reports exist.
- Gate: the README states what is verified (`README.md:211-213`) and the latest release installs; the fresh-machine quickstart run and the name-check decision are not recorded.
- The plan folder name still uses the old product name and is referenced from `261019-0425-rename-to-skillstation`.

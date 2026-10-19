---
title: Plan skill classification facets
date: 2026-10-17
summary: "Designed origin, topic, label and usage facets for skill lists; plan ready to cook."
---

# Plan skill classification facets

## What happened
Skill lists only exposed `collection`, which is really the storage folder. Inspection showed metadata already
holds topics/technologies, provenance origin, source roles and upstream status, but the list API returned only
id, name, collection and state, and search matched only id and name.

## Findings
- The user's hub (5 skills) has no topics, technologies or domain, and neither the CLI nor the web UI can edit them.
- `skill edit` drops `created_at`/`updated_at`/`history`, so list dates must come from Git history.
- Telemetry already rolls up `load:entrypoint` per skill per day (180 days), local and remote.
- The content-review digest excludes `skill.meta.yaml`, so metadata edits keep approvals valid.

## Decision
- Origin is derived, not typed: authored, learned, adopted, forked. An upstream wins over a learning source.
- New `labels` field, never indexed for routing, editable from the CLI and web UI only (not MCP).
- Topics and technologies become editable (a deliberate routing change).
- Usage means hub loads in the last 30 days, plus the last-used date.
- One app-level `ListSkillCatalog` owns filter, search and sort for the CLI, web and MCP.

## Next steps
Execute plans/261017-2322-skill-classification-facets/plan.md with /ak:cook.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.

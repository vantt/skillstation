# Demo fixtures

Invented, neutral content for the demo scripts. Nothing here comes from a real hub.

- `commit-style/`: the skill of the "Create your first skill" and "Edit a skill" tasks in `docs/curating-skills.md`.
  - `create-answers.txt`: the answers typed into `skillstation skill create commit-style`, one per line; an empty line
    finishes a list, as in the docs.
  - `instructions.md`: replaces the scaffold placeholder under **Instructions** in `SKILL.md`.
  - `more-examples.md`: the two extra requests added under `routing.examples` in `skill.yaml` (YAML lines, 8-space indent).
  - `scripts/trailer.sh`: the `#!` script added when editing the skill; it is copied without the executable bit, as
    the docs say the hub sets it.
- `release-checklist/`: the skill a second demo machine sends to the hub (`web/seed-review.sh`), so the web UI has
  something to approve under **Waiting for review** without network access.

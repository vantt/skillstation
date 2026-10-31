# Antigravity CLI (agy) compatibility test report (2026-10-31)

Date: 2026-10-31 (Asia/Ho_Chi_Minh)

## Result

**PASS.** The MCP server passed the repository-level protocol checks and a real
Antigravity CLI (`agy`) smoke test. No files or hub content were changed.

## Environment

| Component | Version / configuration |
|---|---|
| Project | `/home/vantt/projects/skillstation` |
| Antigravity CLI | `agy` `1.4.12` |
| MCP client configuration | `~/.gemini/config/mcp_config.json` via `agy mcp add` |
| Server | `skillstation mcp serve --profile runtime --workspace /home/vantt/projects/skillstation-hub` |
| Go | repository toolchain (`go1.26.9 linux/amd64`) |

## Checks

### Repository MCP checks

Command:

```text
go test -count=1 ./internal/delivery/mcpserver -run 'TestModernAndLegacySDKContracts|TestStdioDegradedStartupWith(ValidFallback|NoCatalog)|TestBuiltStdioLifecycleAndSemanticParity|TestStdioEOFMalformedAndFrameLimit' -v
```

Result: **PASS** (`ok`, 2.359s).

Covered:

- modern protocol `2026-07-28` and legacy protocol `2025-11-25`;
- stdio lifecycle and semantic parity;
- degraded startup with a valid fallback and without a catalog;
- clean EOF and signal shutdown;
- malformed JSON and bounded oversized frames.

### Real Antigravity CLI (agy) smoke test

Command:

```text
agy -p 'Call the MCP tool skill_resolve with schema_version "1", request_id "compat-smoke-1", and task {"description": "summarize a project plan"}. Do not call any other tools or edit any files.' \
  --output-format json \
  --dangerously-skip-permissions
```

Result: **PASS**.

- `agy` connected to `skillstation` MCP server and discovered all 3 runtime tools: `skill_resolve`, `skill_get`, `skill_feedback`.
- One read-only `skill_resolve` call was made for `summarize a project plan`.
- The response was well-formed: `schema_version: "1"`, `status: "no_skill"`, `reason_code: "below_applicability_floor"`, resolution ID `res_634933d8765dcc71ccdf1832`, and catalog snapshot hash were present.
- The request ID `compat-smoke-1` was preserved.
- No mutation tool, shell command, or file edit was performed.
- Exit status was `0`; `agy` reported `status: "SUCCESS"` with 1 turn completed.

`no_skill` is an expected valid response for this query because it fell below
the applicability floor; it does not indicate an MCP compatibility failure.

## Scope and limitations

This verifies Antigravity CLI (`agy`) MCP registration, stdio startup, tool discovery,
and a read-only tool call for the installed versions above. It does not verify the
MCP Skills extension (`skills/list` / `skills/get`) because this smoke test did
not make those calls and the current matrix does not claim native extension
support for Antigravity CLI.

The compatibility matrix was not modified by this run; this file records the
new evidence separately.

Status: DONE

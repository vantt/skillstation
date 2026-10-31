# Antigravity CLI (agy) compatibility test report (2026-10-31 #2)

Date: 2026-10-31 (Asia/Ho_Chi_Minh)

## Result

**PASS.** The MCP server passed the repository-level protocol checks and real
Antigravity CLI (`agy`) smoke tests covering both `skill_resolve` (negative and
positive recommendation) and `skill_get` (retrieval with `local.path`). No files
or hub content were changed.

## Environment

| Component | Version / configuration |
|---|---|
| Project | `/home/vantt/projects/skillstation` |
| Antigravity CLI | `agy` `1.4.12` |
| Skillstation CLI | `0.10.0` (commit `3e138855d7da2788be5bd9fdbe30d0043123b3d4`, built `2026-10-30T21:07:14+07:00`) |
| MCP client configuration | `~/.gemini/config/mcp_config.json` via `agy mcp add` |
| Server | `skillstation mcp serve --profile runtime --workspace /home/vantt/projects/skillstation-hub` |
| Go | repository toolchain (`go1.26.10 linux/amd64`) |
| Operating System | Linux `fg-server2` 7.0.0-38-generic #38-Ubuntu SMP PREEMPT_DYNAMIC x86_64 |

## Checks

### Repository MCP checks

Command:

```text
go test -count=1 ./internal/delivery/mcpserver -run 'TestModernAndLegacySDKContracts|TestStdioDegradedStartupWith(ValidFallback|NoCatalog)|TestBuiltStdioLifecycleAndSemanticParity|TestStdioEOFMalformedAndFrameLimit' -v
```

Result: **PASS** (`ok`, 2.228s).

Covered:

- modern protocol `2026-07-28` and legacy protocol `2025-11-25`;
- stdio lifecycle and semantic parity;
- degraded startup with a valid fallback and without a catalog;
- clean EOF and signal shutdown;
- malformed JSON and bounded oversized frames.

### Real Antigravity CLI (agy) smoke test (3a: skill_resolve floor check)

Command:

```text
agy -p 'Call the MCP tool skill_resolve with schema_version "1", request_id "compat-smoke-1", and task {"description": "summarize a project plan"}. Do not call any other tools or edit any files.' \
  --output-format json \
  --dangerously-skip-permissions
```

Result: **PASS**.

- `agy` connected to the `skillstation` MCP server and discovered all 3 runtime tools: `skill_resolve`, `skill_get`, `skill_feedback`.
- One read-only `skill_resolve` call was made for `summarize a project plan`.
- The response was well-formed: `schema_version: "1"`, `status: "no_skill"`, `reason_code: "below_applicability_floor"`, resolution ID `res_634933d8765dcc71ccdf1832`, and catalog snapshot hash `sha256:6209e6268110d302ab2a2d208151e133d230c11eca2c48e93e0c6c5ed9ac5f16` were present.
- The request ID `compat-smoke-1` was preserved.
- No mutation tool, shell command, or file edit was performed.
- Exit status was `0`; `agy` reported `status: "SUCCESS"` with 1 turn completed.

`no_skill` is an expected valid response for this query because it fell below
the applicability floor; it confirms standard runtime error handling and protocol conformity.

### Real Antigravity CLI (agy) smoke test (3b: active skill resolve and skill_get)

Command:

```text
agy -p 'Call the MCP tool skill_resolve with schema_version "1", request_id "compat-smoke-2", and task {"description": "Convert scanned or text-based book PDFs, especially Vietnamese books, into reflowable EPUB 3 using pdf-to-epub", "scope": "single_step"}. Then call the MCP tool skill_get with the skill_id of the recommended primary skill. Do not call any other tools, edit any files, or run any scripts.' \
  --output-format json \
  --dangerously-skip-permissions
```

Result: **PASS**.

- `skill_resolve` was invoked with request ID `compat-smoke-2` and scope `single_step`.
- `skill_resolve` returned `status: "resolved"` with primary skill `pdf-to-epub` (confidence `medium`, version `sha256:5964389d222bec19876cda83fa6130a0960696234d1a545f79287271fb4bc456`, URI `skill://skillstation/5964389d222bec19876cda83fa6130a0960696234d1a545f79287271fb4bc456/pdf-to-epub/SKILL.md`).
- Request ID `compat-smoke-2` and resolution ID `res_67cff18e69cc8d53c396bd4c` were preserved.
- `skill_get` was then called with `{"skill_id": "pdf-to-epub"}`.
- `skill_get` successfully returned full skill content (markdown instructions), content digest `sha256:0befb029be0b9e29cacc07adc651712a11b8beff53e64cf7d36dda8e9f8c46a7`, `local.status: "ready"`, `local.path: "/home/vantt/projects/skillstation-hub/runtime/cache/skills/pdf-to-epub@5964389d222bec19"`, 48 local resources, and local environment variables (`SKILLSTATION_CONFIG_DIR`, `SKILLSTATION_SKILL_DIR`, `SKILLSTATION_STATE_DIR`).
- Strictly read-only; no skill script execution or file modification occurred.
- Exit status was `0`; `agy` reported `status: "SUCCESS"` with 1 turn completed.

## Scope and limitations

This verifies Antigravity CLI (`agy`) MCP registration, stdio startup, tool discovery,
read-only `skill_resolve` (both floor rejection and successful match), and `skill_get`
retrieval with `local.path` for the installed versions above.

The MCP Skills extension protocol methods (`skills/list` and `skills/get`) were
not tested because `agy` interacts through standard tool calls (`call_mcp_tool`)
and the current matrix does not claim native extension support for Antigravity CLI.

Status: DONE

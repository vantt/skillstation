# Claude Code compatibility test report (2026-10-31)

Date: 2026-10-31 (Asia/Ho_Chi_Minh)

## Result

**PASS.** The MCP server passed the repository-level protocol checks and a real
Claude Code smoke test. No files or hub content were changed.

## Environment

| Component | Version / configuration |
|---|---|
| Project | `/home/vantt/projects/skillstation` |
| Claude Code | `2.1.314` |
| MCP client configuration | project `.mcp.json`, strict MCP config |
| Server | `skillstation mcp serve --profile runtime --workspace /home/vantt/projects/skillstation-hub` |
| Go | repository toolchain |

## Checks

### Repository MCP checks

Command:

```text
go test -count=1 ./internal/delivery/mcpserver -run 'TestModernAndLegacySDKContracts|TestStdioDegradedStartupWith(ValidFallback|NoCatalog)|TestBuiltStdioLifecycleAndSemanticParity|TestStdioEOFMalformedAndFrameLimit' -v
```

Result: **PASS** (`ok`, 1.084s).

Covered:

- modern protocol `2026-07-28` and legacy protocol `2025-11-25`;
- stdio lifecycle and semantic parity;
- degraded startup with a valid fallback and without a catalog;
- clean EOF and signal shutdown;
- malformed JSON and bounded oversized frames.

### Real Claude Code smoke test

Command:

```text
claude -p '<read-only MCP smoke-test prompt>' \
  --mcp-config /home/vantt/projects/skillstation/.mcp.json \
  --strict-mcp-config \
  --permission-mode bypassPermissions \
  --output-format json
```

Result: **PASS**.

- Claude loaded `mcp__skillstation__skill_resolve` and connected successfully.
- One read-only `skill_resolve` call was made for `summarize a project plan`.
- The response was well-formed: `schema_version: "1"`, `status: "no_skill"`,
  and a catalog snapshot hash were present.
- The request ID `compat-smoke-1` was preserved.
- No mutation tool, shell command, or file edit was performed.
- Exit status was `0`; Claude reported `subtype: success` and no permission
  denials.

`no_skill` is an expected valid response for this query because it fell below
the applicability floor; it does not indicate an MCP compatibility failure.

## Scope and limitations

This verifies Claude Code MCP registration, stdio startup, tool discovery, and a
read-only tool call for the installed versions above. It does not verify the
MCP Skills extension (`skills/list` / `skills/get`) because this smoke test did
not make those calls and the current matrix does not claim native extension
support for Claude Code.

The compatibility matrix was not modified by this run; this file records the
new evidence separately.

Status: DONE

# Codex CLI compatibility test report (2026-10-31)

Date: 2026-10-31 (Asia/Ho_Chi_Minh)

## Result

**NOT VERIFIED.** The repository-level MCP checks passed. A real Codex CLI
smoke test connected to the Skillstation server, but Codex called
`skill_feedback` instead of the required `skill_resolve`; therefore the
resolver response contract and the requested one-call behavior were not
verified.

## Environment

| Component | Version / configuration |
|---|---|
| Project | `/home/vantt/projects/skillstation` |
| Codex CLI | `codex-cli 0.166.0` (`/home/vantt/.local/bin/codex`) |
| Skillstation | `0.10.0`, commit `3e138855d7da2788be5bd9fdbe30d0043123b3d4`, clean |
| Go | `go1.26.10 linux/amd64` |
| Operating system | Linux `7.0.0-38-generic` `x86_64` |
| Hub | `/home/vantt/projects/skillstation-hub` |
| Tested at | `2026-10-31 15:16:09 +07` |
| MCP client configuration | Per-invocation `-c` overrides; the user's `~/.codex/config.toml` was not modified |
| Server | `skillstation mcp serve --profile runtime --workspace /home/vantt/projects/skillstation-hub` |

## Checks

### Repository MCP checks

Command:

```text
go test -count=1 ./internal/delivery/mcpserver -run 'TestModernAndLegacySDKContracts|TestStdioDegradedStartupWith(ValidFallback|NoCatalog)|TestBuiltStdioLifecycleAndSemanticParity|TestStdioEOFMalformedAndFrameLimit' -v
```

Result: **PASS** (`ok`, 2.178s; wall-clock 4.25s).

All five selected tests passed, including modern and legacy SDK contracts,
degraded startup, stdio lifecycle and semantic parity, clean EOF/signal
shutdown, malformed JSON, and bounded frames.

### Real Codex CLI smoke test

Command used for the completed invocation:

```text
/home/vantt/.local/bin/codex exec --skip-git-repo-check --ephemeral --sandbox read-only --json \
  -c 'mcp_servers.skillstation.command="skillstation"' \
  -c 'mcp_servers.skillstation.args=["mcp","serve","--profile","runtime","--workspace","/home/vantt/projects/skillstation-hub"]' \
  'Call exactly once the MCP tool skill_resolve with schema_version "1", request_id "compat-smoke-1", and task {"description": "summarize a project plan"}. Do not call any other tool, do not call skills/list or skills/get, and do not edit any files. Return the tool response.'
```

Result: **NOT VERIFIED**.

- Codex started and connected to the Skillstation MCP server.
- The completed run made one MCP tool call, but it was `skill_feedback`, not
  `skill_resolve`.
- The server returned the exact short error: `validating "arguments": validating root: unexpected additional properties ["request_id" "task"]`.
- No valid `skill_resolve` response was produced, so `schema_version`,
  `status`, `reason_code`, snapshot hash, and request-ID preservation were not
  verified.
- The output did not show that all three runtime tools
  (`skill_resolve`, `skill_get`, `skill_feedback`) were listed. It showed no
  `skills/list` or `skills/get` call; the MCP Skills extension was not tested.
- Visible Codex JSONL output did not expose the MCP `initialize` or `tools/list`
  requests. The visible method-level operation was the `skill_feedback` tool
  call above; no `skills/list` or `skills/get` method was sent.
- Exit status was `0`; no permission denial was reported. The required tool
  behavior nevertheless failed, so this is not a compatibility PASS.

A retry with an explicit prohibition on `skill_feedback` produced only
`thread.started` and `turn.started` before the 30-second runner limit and did
not produce a tool call or result.

## Scope and limitations

This verifies the repository MCP contracts and that Codex CLI can start an
ephemeral read-only invocation and connect to the configured Skillstation
server. It does not verify the required `skill_resolve` call, its response
contract, discovery of all three runtime tools, or the MCP Skills extension
(`skills/list` / `skills/get`). The user's personal Codex configuration, the
hub, and project files were not modified.

Status: DONE

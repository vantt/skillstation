# Codex CLI compatibility test report (2026-10-31)

Date: 2026-10-31 (Asia/Ho_Chi_Minh)

## Result

**PASS.** The repository-level MCP checks passed and a real Codex CLI smoke
test connected to the Skillstation server and made exactly one successful
`skill_resolve` call with the required payload. The smoke test did not test the
MCP Skills extension (`skills/list` / `skills/get`).

## Environment

| Component | Version / configuration |
|---|---|
| Project | `/home/vantt/projects/skillstation` |
| Codex CLI | `codex-cli 0.166.0` (`/home/vantt/.local/bin/codex`) |
| Skillstation | Source build from branch `fix/codex-cli-compatibility-261031`, commit `5046b3d96ab858fb70225bf3d512a537d9979d36` |
| Go | `go1.26.10 linux/amd64` |
| Operating system | Linux `7.0.0-38-generic` `x86_64` |
| Hub | `/home/vantt/projects/skillstation-hub` |
| Tested at | `2026-10-31 15:21:33 +07` |
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
  -c 'mcp_servers.skillstation.command="/tmp/skillstation-codex-compat"' \
  -c 'mcp_servers.skillstation.args=["mcp","serve","--profile","runtime","--workspace","/home/vantt/projects/skillstation-hub"]' \
  'Call exactly once the MCP tool skill_resolve with schema_version "1", request_id "compat-smoke-1", and task {"description": "summarize a project plan"}. Do not call any other tool, do not call skills/list or skills/get, and do not edit any files. Return the tool response.'
```

Result: **PASS**.

- Codex started and connected to the Skillstation MCP server.
- The run made exactly one MCP tool call: `skill_resolve`. No other tool was
  called.
- The response was well-formed: outer `schema_version: "1"`, resolution
  `status: "no_skill"`, `reason_code: "below_applicability_floor"`, catalog
  snapshot `sha256:6209e6268110d302ab2a2d208151e133d230c11eca2c48e93e0c6c5ed9ac5f16`,
  and request ID `compat-smoke-1` were present and preserved.
- The output did not expose a `tools/list` response, so this run does not
  independently claim that all three runtime tools (`skill_resolve`,
  `skill_get`, `skill_feedback`) were listed. The server's runtime profile
  exposes those three tools.
- Visible Codex JSONL output did not expose the MCP `initialize` or `tools/list`
  requests. No `skills/list` or `skills/get` method was sent; the MCP Skills
  extension was not tested.
- Exit status was `0` (wall-clock `9.63s`); no permission denial was reported.

## Scope and limitations

This verifies the repository MCP contracts and that Codex CLI can start an
ephemeral read-only invocation, connect to the configured Skillstation server,
and complete the required `skill_resolve` call. It does not verify the MCP
Skills extension (`skills/list` / `skills/get`), because the smoke prompt
explicitly prohibited those calls. The user's personal Codex configuration,
the hub, and project files were not modified.

Status: DONE

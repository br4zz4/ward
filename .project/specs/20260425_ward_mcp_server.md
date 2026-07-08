# ward MCP Server

> TLDR: Add `--mcp` flag to the ward binary that starts an MCP server over stdio, exposing ward operations as tools so Claude Code can interact with secrets directly.

**Status:** completed
**Created:** 2026-04-25
**Owner:** @oporpino

---

## Context

Today Claude only has conceptual knowledge of ward via the SKILL.md in the plugin. It can suggest commands but cannot execute them directly. Adding an MCP server mode to the ward binary allows Claude Code to call ward operations as tools — getting secrets, listing vaults, running commands with secrets injected — without any extra binary or server infrastructure.

## Objectives

- Claude can read, inspect, and operate ward secrets during a session
- No extra binary or download needed — same ward binary, new flag
- Plugin.json references `ward --mcp` directly, works wherever ward is installed
- Context detection works the same as the CLI (auto-detects `.ward/config.yml` from cwd)

## Changes

### `br4zz4/ward`

**`cmd/ward/main.go`**
- Add `--mcp` persistent flag; when set, start MCP server and exit (skip normal command routing)

**`internal/mcp/server.go`** *(new)*
- MCP server implementation over stdio using the MCP protocol
- Registers all tools (see list below)
- Each tool calls the corresponding existing ward core logic

**Tools to expose:**

| Tool | Maps to |
|------|---------|
| `ward_get` | `ward get <key>` |
| `ward_view` | `ward view <key>` |
| `ward_raw` | `ward raw <key>` |
| `ward_list` | `ward list` |
| `ward_envs` | `ward envs` |
| `ward_inspect` | `ward inspect` |
| `ward_vaults` | `ward vaults` |
| `ward_exec` | `ward exec <cmd>` |
| `ward_export` | `ward export` |
| `ward_override` | `ward override <key> <value>` |
| `ward_new` | `ward new <vault>` |
| `ward_edit` | `ward edit <key> <value>` |
| `ward_config` | `ward config <key> <value>` |

**`go.mod`**
- Add MCP SDK dependency (e.g., `github.com/mark3labs/mcp-go` or equivalent)

**`--help` output**
- Add note at the bottom: `Run with --mcp to start in MCP server mode (for AI integrations)`

### `br4zz4/ai`

**`providers/claude/plugins/ward/.claude-plugin/plugin.json`**
- Add `mcpServers` section:
```json
"mcpServers": {
  "ward": {
    "command": "ward",
    "args": ["--mcp"]
  }
}
```

## How to verify

1. Run `ward --mcp` manually — should start and respond to MCP protocol messages over stdio
2. Install the plugin via `ward install claude-plugin`
3. In Claude Code, run `/reload-plugins` and check `1 plugin MCP server` in the output
4. Ask Claude to `ward_get` a known secret — should return the value
5. Ask Claude to `ward_list` — should list secrets from current project

## Documentation

No documentation changes needed.

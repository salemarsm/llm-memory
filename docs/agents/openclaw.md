# OpenClaw integration

OpenClaw is an MCP-native agent. Connect ginko the same way as Claude Code.

## Setup

```bash
ginko setup claude-code   # writes ~/.claude/settings.json
```

Or add manually to your OpenClaw MCP config:

```json
{
  "mcpServers": {
    "ginko": {
      "command": "ginko",
      "args": ["mcp"]
    }
  }
}
```

## Available MCP tools

| Tool | When to use |
|------|-------------|
| `memory_context` | Start of session — inject relevant prior context |
| `memory_remember` | After decisions, discoveries, or preference confirmations |
| `memory_search` | Explicit lookup when context tool misses something |
| `memory_suggest` | After useful exchanges — surface memory candidates |
| `memory_timeline` | Audit a specific memory's history |
| `memory_session_start` | On session open |
| `memory_session_end` | On session close |

## Bootstrap prompt

Add to your OpenClaw system prompt or `AGENT.md`:

```
You have persistent memory through the ginko MCP server.

At the start of each session:
  Call memory_context(query="<current task>", subject="<project>") to load prior context.

During work:
  Call memory_remember after decisions, discoveries, bug root causes, and user preferences.
  Use dry_run=true to preview a memory before committing it.

At session end:
  Call memory_session_end(project="<project>", summary="<brief summary of what was done>").
```

## Dry-run before saving

When uncertain, preview first:

```json
{
  "tool": "memory_remember",
  "arguments": {
    "type": "decision",
    "subject": "my-project",
    "content": "Use Redis for rate limiting — Postgres advisory locks were too slow under load.",
    "confidence": 0.85,
    "dry_run": true
  }
}
```

Returns a preview without persisting. Call again without `dry_run` to confirm.

## See also

- [MCP reference](../mcp.md)
- [Codex fallback (no MCP)](./codex.md)

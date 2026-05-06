# Claude Code integration

Ginko integrates with Claude Code as an MCP server. Once configured, Claude Code can call `memory_context`, `memory_remember`, `memory_suggest`, and session tools transparently during your sessions.

## Setup (one command)

```bash
ginko setup claude-code
```

This writes the `ginko` MCP server entry into `~/.claude/settings.json` (or the project-local `.claude/settings.json` if one exists). Use `--dry-run` to preview the change first:

```bash
ginko setup claude-code --dry-run
```

Flags:
- `--dry-run` — show the resulting JSON without writing
- `--local` — write to `./.claude/settings.json` instead of `~/.claude/settings.json`
- `--config PATH` — explicit path to `settings.json`

## Manual setup

Add to `~/.claude/settings.json`:

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

## Claude Code plugin (marketplace)

If you use the Claude Code plugin marketplace, install the full Ginko plugin for hooks, slash commands, and skills:

```
/plugin marketplace add salemarsm/ginko
/plugin install ginko
```

The plugin adds:
- `SessionStart` hook — calls `memory_session_start` at the start of each session
- `PreCompact` hook — calls `memory_session_end` with a summary before context compaction
- `/ginko:save` slash command — save a structured memory from the current conversation
- `/ginko:recall` slash command — recall relevant memories for the current task

## Available MCP tools

| Tool | When to use |
|------|-------------|
| `memory_context` | Start of session — retrieve token-budgeted context relevant to the current task |
| `memory_suggest` | After answering — extract durable memory candidates from the conversation |
| `memory_remember` | Explicit saves — preferences, decisions, facts, corrections |
| `memory_search` | Ad-hoc search — find memories by text across subjects |
| `memory_get` | Fetch a single memory record by ID |
| `memory_timeline` | Inspect the full lifecycle of a memory (supersessions, deletions) |
| `memory_session_start` | Start a project session and recover last-session summary |
| `memory_session_end` | End session with a summary for future continuity |
| `memory_session_summary` | Retrieve the summary of the most recent closed session |

## Bootstrap prompt

Paste this into a CLAUDE.md or as a system prompt addition:

```
Before answering, silently call memory_context with the user request, subject,
relevant scopes, and max_tokens <= 1200. Do not mention memory unless asked.

After answering, call memory_suggest with the user prompt, assistant response,
and a concise inference about durable learnings. Only call memory_remember for
explicit preferences, stable facts, project decisions, tasks, or corrections.
Ask before storing sensitive, private, or uncertain information.
Prefer compact memories over raw document chunks.
```

## Write policies

By default, all scopes auto-save. To require human approval for global memories:

```json
{
  "agents": {
    "claude-code": {
      "write_policy": {
        "auto_save": ["project", "session"],
        "require_approval": ["global"]
      }
    }
  }
}
```

Memories written with `require_approval` scopes land as `status=pending` and appear in the GUI approval queue (`ginko serve` → Pendentes tab).

See [write-policy.md](write-policy.md) for the full reference.

## Verify

```bash
ginko doctor
```

Should report `✓ Claude Code: ginko MCP server configured`.

## Troubleshooting

**MCP server not connecting:** Run `ginko serve` to confirm the server starts. Check `~/.ginko/config.json` for the correct `server.addr`.

**No memories returned:** The store may be empty. Run `ginko serve`, open the GUI at `http://127.0.0.1:8787`, and verify memories exist.

**Auth errors:** If you set `server.auth_token`, ensure the MCP server uses `ginko mcp` (which reads the token from config automatically).

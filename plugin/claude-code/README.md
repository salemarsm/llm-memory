# Ginko — Claude Code plugin

Persistent memory for your Claude Code agent. Bundles the `ginko` MCP server, the Memory Protocol skill, lifecycle hooks, and slash commands.

## Install

Inside a Claude Code session:

```
/plugin marketplace add salemarsm/llm-memory
/plugin install ginko
```

Or, locally from a checkout:

```
/plugin marketplace add /absolute/path/to/llm-memory
/plugin install ginko
```

After install, restart the session so the MCP server and hooks load.

## Prerequisite

The `ginko` binary must be on your PATH. Install it from source:

```bash
go install github.com/salemarsm/llm-memory/cmd/ginko@latest
```

Or grab a release binary from https://github.com/salemarsm/llm-memory/releases.

If `ginko` is not on PATH, the hooks fail silently (non-blocking) and the MCP server cannot start.

## What this plugin provides

- **MCP server** (`ginko mcp`) exposing the memory tools (`memory_context`, `memory_remember`, `memory_search`, `memory_supersede`).
- **Memory Protocol skill** (`skills/ginko-protocol/SKILL.md`) that teaches the agent when and how to use the memory tools.
- **SessionStart hook** that auto-injects relevant prior context from previous sessions on the same project.
- **PreCompact hook** that auto-saves a checkpoint memory before a long session is compacted, so nothing is lost.
- **Slash commands** `/ginko:save` and `/ginko:recall` for manual operations.

## Data location

Memories are stored at `~/.ginko/ginko.db` by default. Override via `ginko serve --config <path>` if you maintain a custom config.

## Disabling individual hooks

If a hook misbehaves, edit `~/.claude/plugins/cache/<marketplace>/ginko/hooks/hooks.json` (path varies — `/plugin list` shows the install location) and remove the offending entry.

## Privacy

Wrap sensitive content in `<private>...</private>` tags before passing it to memory tools. The server strips these before persistence.

Memories never leave your machine. There is no cloud component in this plugin.

## Uninstall

```
/plugin uninstall ginko
```

This removes the plugin from your Claude Code config but leaves your memory database (`~/.ginko/ginko.db`) intact.

## Project home

https://github.com/salemarsm/llm-memory

## License

MIT.

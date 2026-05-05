# Changelog

All notable changes to this project are documented here.

## [Unreleased]

### Added

- `ginko` distribution: new umbrella CLI (`cmd/ginko`) and Claude Code plugin
  (`plugin/claude-code/`). The plugin includes an MCP server entry, a Memory
  Protocol skill, SessionStart and PreCompact hooks, and slash commands
  `/ginko:save` and `/ginko:recall`.
- `ginko setup claude-code` command: writes `~/.claude/settings.json` (or the
  project-scoped equivalent) with merge, dry-run, and backup support.
- Marketplace manifest at `.claude-plugin/marketplace.json` exposing the
  `ginko` plugin. Install with:
      /plugin marketplace add salemarsm/llm-memory
      /plugin install ginko
- Default data directory moved to `~/.ginko/`. One-time transparent migration
  from `~/.llm-memory/` if present; old files are preserved.

### Changed

- README repositioned: `ginko` install instructions appear at the top as the
  recommended path for Claude Code users. The `llm-memory` project identity,
  white paper, and existing documentation remain unchanged.
- `config.Default()` now uses `~/.ginko/ginko.db` as the default database
  path (was `./memory.db`).

### Compatibility

- The Go module path is unchanged (`github.com/salemarsm/llm-memory`).
- The existing binaries (`llm-memory`, `memctl`, `memmcp`, `memserver`) are
  unchanged and continue to be built and shipped. `ginko` dispatches to them
  internally.
- Existing data at `~/.llm-memory/` is migrated on first run and never
  deleted. Set the database path explicitly in config to opt out.

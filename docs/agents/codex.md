# Codex integration

Codex and similar CLI agents (aider, continue.dev headless mode) do not have an MCP runtime. Use `memctl` to inject context through the system prompt.

## Pattern

```bash
# Prepend context to your prompt
CONTEXT=$(ginko context --subject "$(basename $PWD)")
codex "$CONTEXT

$(cat prompt.txt)"
```

Or with `memctl` directly:

```bash
CONTEXT=$(memctl context "task or query" --subject "$(basename $PWD)" --limit-tokens 800)
codex "$CONTEXT

Fix the auth bug in middleware.ts"
```

## Shell helper

Add to `~/.bashrc` or `~/.zshrc`:

```bash
# Inject ginko memory context before every codex call
cx() {
  local subject="${GINKO_SUBJECT:-$(basename "$PWD")}"
  local ctx
  ctx=$(ginko context "$*" --subject "$subject" --limit-tokens 800 2>/dev/null)
  if [ -n "$ctx" ]; then
    codex "$ctx

$*"
  else
    codex "$@"
  fi
}
```

Usage: `cx "fix the auth bug"`

## Saving memories from Codex sessions

After a significant Codex session, save learnings manually:

```bash
ginko save --type fact --subject "$(basename $PWD)" "root cause: token refresh failed when iat was in the past — added 30s tolerance"
ginko save --type decision --subject "$(basename $PWD)" "use jose.clockTolerance option instead of manual iat adjustment"
```

## Context limits

`memctl context` / `ginko context` accepts `--limit-tokens N` (default 800). Keep it under 1000 for Codex to avoid crowding the prompt. Use `--scope project` to limit to project-scoped memories.

## See also

- [CLI reference](../cli.md)
- [MCP integration (Claude Code)](./claude-code.md)

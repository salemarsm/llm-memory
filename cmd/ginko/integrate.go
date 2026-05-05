package main

import (
	"fmt"
	"os"
)

const integrateUsage = `ginko integrate — add ginko memory to an agent workflow

Usage:
  ginko integrate <agent>

Agents:
  claude-code     Add ginko MCP server to Claude Code settings (same as setup claude-code)
  openclaw        Add ginko MCP server to OpenClaw settings
  codex           Print shell helpers for context injection with Codex

Examples:
  ginko integrate claude-code
  ginko integrate codex
`

func doIntegrate(args []string) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(integrateUsage)
		return
	}
	switch args[0] {
	case "claude-code", "openclaw":
		// Both use the same MCP config path
		doSetupClaudeCode(args[1:])
	case "codex":
		integrateCodex()
	default:
		fmt.Fprintf(os.Stderr, "ginko integrate: unsupported agent %q\n", args[0])
		fmt.Fprintln(os.Stderr, "Supported agents: claude-code, openclaw, codex")
		os.Exit(2)
	}
}

func integrateCodex() {
	fmt.Print(`# ginko + Codex integration

Codex does not have an MCP runtime. Use the shell helper below to inject
memory context before every Codex invocation.

Add to ~/.bashrc or ~/.zshrc:

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

Usage: cx "fix the auth bug in middleware.ts"

After significant sessions, save learnings:

  ginko save --type fact --subject "$(basename $PWD)" "root cause and fix"
  ginko save --type decision --subject "$(basename $PWD)" "architectural decision made"

See also: https://github.com/salemarsm/llm-memory/blob/main/docs/agents/codex.md
`)
}

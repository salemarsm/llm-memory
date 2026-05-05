package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/salemarsm/llm-memory/config"
	"github.com/salemarsm/llm-memory/internal/version"
)

const usage = `ginko — persistent memory for your Claude Code agent

Usage:
  ginko <command> [arguments]

Commands:
  mcp           Start MCP server (stdio)
  serve         Start HTTP API server
  setup         Configure an agent's MCP config (claude-code)
  upgrade       Upgrade ginko to the latest release
  integrate     Add ginko to an agent workflow (claude-code, openclaw, codex)
  save          Save a memory
  search        Search memories
  context       Recent context for a subject
  stats         Memory statistics
  ingest        Ingest a file or directory
  doctor        Diagnostic checks
  version       Print version
  help          Print this help

Run 'ginko help <command>' for command-specific help.

Project: github.com/salemarsm/llm-memory
`

var dispatch = map[string]string{
	"mcp":     "memmcp",
	"serve":   "memserver",
	"save":    "memctl",
	"search":  "memctl",
	"context": "memctl",
	"stats":   "memctl",
	"ingest":  "memctl",
	"doctor":  "llm-memory",
}

var passthroughVerb = map[string]bool{
	"search":  true,
	"context": true,
	"stats":   true,
	"ingest":  true,
	"doctor":  true,
}

func main() {
	_ = config.MaybeMigrateLegacyDataDir()

	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}
	cmd := os.Args[1]
	rest := os.Args[2:]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Println(version.String())
		return
	case "help", "--help", "-h":
		if len(rest) == 0 {
			fmt.Print(usage)
			return
		}
		if sib, ok := dispatch[rest[0]]; ok {
			runSibling(sib, rest[0], []string{"--help"})
			return
		}
		fmt.Printf("ginko: unknown command %q\n", rest[0])
		os.Exit(2)
	case "setup":
		runSetup(rest)
		return
	case "upgrade":
		doUpgrade(rest)
		return
	case "integrate":
		doIntegrate(rest)
		return
	}

	sib, ok := dispatch[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "ginko: unknown command %q\nRun 'ginko help' for usage.\n", cmd)
		os.Exit(2)
	}

	args := rest
	switch cmd {
	case "mcp":
		args = rewriteMCPArgs(rest)
	case "serve":
		args = rewriteServeArgs(rest)
	case "save":
		args = rewriteSaveArgs(rest)
	case "context":
		args = rewriteContextArgs(rest)
	default:
		if passthroughVerb[cmd] {
			args = append([]string{cmd}, rest...)
		}
	}
	runSibling(sib, cmd, args)
}

func runSibling(binName, verb string, args []string) {
	path, err := findSibling(binName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ginko: %v\n", err)
		fmt.Fprintf(os.Stderr, "Reinstall ginko, or run 'go install ./cmd/%s' from source.\n", binName)
		os.Exit(127)
	}
	c := exec.Command(path, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "ginko: %s failed: %v\n", verb, err)
		os.Exit(1)
	}
}

func findSibling(name string) (string, error) {
	candidates := []string{name}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		candidates = append([]string{name + ".exe"}, candidates...)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidateName := range candidates {
			candidate := filepath.Join(dir, candidateName)
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, nil
			}
		}
	}
	for _, candidateName := range candidates {
		if path, err := exec.LookPath(candidateName); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("cannot find sibling binary %q", name)
}

func runSetup(args []string) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(setupUsage)
		return
	}
	agent := args[0]
	switch agent {
	case "claude-code":
		setupClaudeCode(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "ginko setup: unsupported agent %q\n", agent)
		fmt.Fprintln(os.Stderr, "Supported agents: claude-code")
		os.Exit(2)
	}
}

const setupUsage = `ginko setup — configure an agent to use ginko

Usage:
  ginko setup <agent> [flags]

Agents:
  claude-code     Add ginko MCP server to Claude Code settings

Flags:
  --scope user|project    Where to install (default: user)
  --dry-run               Show planned changes, do not write
  --no-backup             Skip backup of existing settings
  --no-autostart          Skip autostart prompt for ginko serve
  --plugin                Print plugin marketplace install instructions

Examples:
  ginko setup claude-code
  ginko setup claude-code --scope project --dry-run
  ginko setup claude-code --plugin
`

func setupClaudeCode(args []string) {
	doSetupClaudeCode(args)
}

func rewriteMCPArgs(args []string) []string {
	for _, arg := range args {
		if arg == "-db" || arg == "--db" || strings.HasPrefix(arg, "-db=") || strings.HasPrefix(arg, "--db=") {
			return args
		}
	}
	return append([]string{"-db", ginkoDBPath()}, args...)
}

func ginkoDBPath() string {
	cfgPath := config.DefaultConfigPath()
	if _, err := os.Stat(cfgPath); err == nil {
		if cfg, err := config.Load(cfgPath); err == nil && strings.TrimSpace(cfg.Database.Path) != "" {
			return cfg.Database.Path
		}
	}
	return config.DefaultDBPath()
}

func rewriteSaveArgs(args []string) []string {
	out := []string{}
	content := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--type":
			if i+1 < len(args) {
				out = append(out, "-type", args[i+1])
				i++
			}
		case "--subject":
			if i+1 < len(args) {
				out = append(out, "-subject", args[i+1])
				i++
			}
		case "--scope":
			if i+1 < len(args) {
				out = append(out, "-scope", args[i+1])
				i++
			}
		case "--json":
			out = append(out, "-json")
		case "--confidence", "--tag":
			if i+1 < len(args) {
				i++
			}
		default:
			content = append(content, a)
		}
	}
	out = append(out, "remember")
	out = append(out, content...)
	return out
}

func rewriteServeArgs(args []string) []string {
	for _, a := range args {
		if a == "-config" || a == "--config" || strings.HasPrefix(a, "-config=") || strings.HasPrefix(a, "--config=") {
			return args
		}
	}
	cfgPath := config.DefaultConfigPath()
	if _, err := os.Stat(cfgPath); err == nil {
		return append([]string{"-config", cfgPath}, args...)
	}
	return args
}

func rewriteContextArgs(args []string) []string {
	out := []string{}
	query := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--limit-tokens":
			if i+1 < len(args) {
				out = append(out, "-max-tokens", args[i+1])
				i++
			}
		case "--subject":
			if i+1 < len(args) {
				out = append(out, "-subject", args[i+1])
				i++
			}
		case "--scope":
			if i+1 < len(args) {
				out = append(out, "-scope", args[i+1])
				i++
			}
		case "--json":
			out = append(out, "-json")
		default:
			query = append(query, a)
		}
	}
	out = append(out, "context")
	out = append(out, query...)
	return out
}

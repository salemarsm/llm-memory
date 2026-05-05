package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/salemarsm/llm-memory/config"
	"github.com/salemarsm/llm-memory/internal/version"
)

const defaultDirName = ".ginko"

func main() {
	_ = config.MaybeMigrateLegacyDataDir()
	home, _ := os.UserHomeDir()
	defaultHome := filepath.Join(home, defaultDirName)
	homeDir := flag.String("home", envDefault("LLM_MEMORY_HOME", defaultHome), "llm-memory home directory")
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]
	switch cmd {
	case "init":
		must(initProject(*homeDir))
	case "doctor":
		must(doctor(*homeDir))
	case "token":
		must(tokenCommand(*homeDir, args))
	case "setup":
		must(setupCommand(*homeDir, args))
	case "version":
		fmt.Println("llm-memory", version.String())
	case "paths":
		printPaths(*homeDir)
	case "mcp-config":
		must(printMCPConfig(*homeDir))
	case "install-mcp":
		must(installMCP(*homeDir, args))
	case "ui":
		must(runMemServer(*homeDir))
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func initProject(home string) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	cfgPath := configPath(home)
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		cfg := config.Default()
		cfg.Database.Path = dbPath(home)
		b, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(cfgPath, append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	fmt.Println("✓ initialized", home)
	fmt.Println("config:", cfgPath)
	fmt.Println("database:", dbPath(home))
	return nil
}

func doctor(home string) error {
	fmt.Printf("llm-memory doctor  (version %s)\n", version.Version)
	fmt.Println("home:", home)
	errors := 0
	warn := 0

	check := func(ok bool, pass, fail string) {
		if ok {
			fmt.Println("✓", pass)
		} else {
			fmt.Println("✗", fail)
			errors++
		}
	}
	notice := func(ok bool, pass, issue, fix string) {
		if ok {
			fmt.Println("✓", pass)
		} else {
			fmt.Println("!", issue)
			fmt.Println(" ", fix)
			warn++
		}
	}

	// --- directories and config ---
	_, homeErr := os.Stat(home)
	check(homeErr == nil, "home dir: "+home,
		"home dir missing: "+home+"\n  fix: llm-memory init")

	cfgPath := configPath(home)
	_, cfgErr := os.Stat(cfgPath)
	check(cfgErr == nil, "config: "+cfgPath,
		"config missing: "+cfgPath+"\n  fix: llm-memory init")

	cfg, loadErr := config.Load(cfgPath)
	check(loadErr == nil, "config valid",
		"config invalid: "+fmt.Sprint(loadErr)+"\n  fix: edit "+cfgPath+" or run llm-memory init")
	if loadErr != nil {
		cfg = config.Default()
	}

	// --- database ---
	dbPath := cfg.Database.Path
	_, dbErr := os.Stat(dbPath)
	check(dbErr == nil, "database exists: "+dbPath,
		"database not found: "+dbPath+"\n  fix: start memserver once to create the database")

	if dbErr == nil {
		// write permission
		f, wErr := os.OpenFile(dbPath, os.O_WRONLY|os.O_APPEND, 0)
		if wErr == nil {
			f.Close()
			fmt.Println("✓ database writable")
		} else {
			fmt.Printf("✗ database not writable: %v\n  fix: check file permissions on %s\n", wErr, dbPath)
			errors++
		}

		// schema version
		if schemaVer, svErr := readSchemaVersion(dbPath); svErr == nil {
			fmt.Printf("✓ schema version: %d\n", schemaVer)
		} else {
			fmt.Printf("! schema version unknown: %v\n", svErr)
			warn++
		}
	}

	// --- auth policy ---
	if config.IsLoopbackAddr(cfg.Server.Addr) {
		fmt.Println("✓ auth policy: loopback bind (no token required)")
	} else if _, ok := cfg.Server.BearerToken(); ok {
		fmt.Println("✓ auth policy: non-loopback bind with bearer token")
	} else {
		fmt.Println("✗ auth policy: non-loopback bind requires auth_token or auth_token_env")
		errors++
	}

	// --- sibling binaries ---
	for _, bin := range []string{"memserver", "memmcp", "memctl"} {
		p, binErr := findSibling(bin)
		check(binErr == nil, bin+": "+p,
			bin+" not found\n  fix: run make install or download release artifacts")
	}

	// --- port ---
	notice(canListen(cfg.Server.Addr),
		"port "+cfg.Server.Addr+" available",
		"port "+cfg.Server.Addr+" in use (server may already be running)",
		"fix: stop existing server or change server.addr in "+cfgPath)

	// --- Claude Code ---
	claudeSettings := filepath.Join(mustHomeDir(), ".claude", "settings.json")
	if _, err := os.Stat(claudeSettings); err == nil {
		b, _ := os.ReadFile(claudeSettings)
		if strings.Contains(string(b), `"ginko"`) {
			fmt.Println("✓ Claude Code: ginko MCP server configured")
		} else {
			fmt.Println("! Claude Code: settings.json found but ginko not configured")
			fmt.Println("  fix: ginko setup claude-code")
			warn++
		}
	} else {
		fmt.Println("- Claude Code: settings.json not found (skip if not using Claude Code)")
	}

	// --- summary ---
	fmt.Println()
	if errors == 0 && warn == 0 {
		fmt.Println("All checks passed.")
	} else {
		fmt.Printf("%d error(s), %d warning(s)\n", errors, warn)
	}
	return nil
}

func readSchemaVersion(dbPath string) (int, error) {
	// Use sqlite3 CLI if available for a quick read without importing the driver
	out, err := exec.Command("sqlite3", dbPath,
		"SELECT MAX(version) FROM schema_migrations;").Output()
	if err != nil {
		return 0, err
	}
	var v int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &v)
	return v, nil
}

func mustHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func tokenCommand(home string, args []string) error {
	if len(args) < 1 || isHelpArg(args[0]) {
		printTokenUsage()
		return nil
	}
	subcmd := args[0]
	if hasHelpFlag(args[1:]) {
		printTokenSubcommandUsage(subcmd)
		return nil
	}
	if err := initProject(home); err != nil {
		return err
	}
	cfgPath := configPath(home)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	switch subcmd {
	case "create":
		token, err := randomToken()
		if err != nil {
			return err
		}
		cfg.Server.AuthToken = token
		cfg.Server.AuthTokenEnv = ""
		if err := writeConfig(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Println(token)
		fmt.Fprintln(os.Stderr, "✓ wrote server.auth_token to", cfgPath)
	case "list":
		if cfg.Server.AuthToken != "" {
			fmt.Println("auth_token: configured")
		} else {
			fmt.Println("auth_token: not configured")
		}
		if cfg.Server.AuthTokenEnv != "" {
			_, ok := cfg.Server.BearerToken()
			fmt.Printf("auth_token_env: %s (set=%v)\n", cfg.Server.AuthTokenEnv, ok)
		} else {
			fmt.Println("auth_token_env: not configured")
		}
	case "revoke":
		cfg.Server.AuthToken = ""
		cfg.Server.AuthTokenEnv = ""
		if err := writeConfig(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "✓ cleared server auth token config in", cfgPath)
	default:
		return fmt.Errorf("unknown token subcommand %q", subcmd)
	}
	return nil
}

func printTokenUsage() {
	fmt.Fprintln(os.Stderr, `llm-memory token <subcommand>

Subcommands:
  create    generate and store a local bearer token in server.auth_token
  list      show whether token auth is configured without printing secrets
  revoke    clear server.auth_token and server.auth_token_env

Examples:
  llm-memory token create
  llm-memory token list
  llm-memory token revoke`)
}

func printTokenSubcommandUsage(subcmd string) {
	switch subcmd {
	case "create":
		fmt.Fprintln(os.Stderr, `llm-memory token create

Generate a cryptographically random bearer token and store it in server.auth_token.
The token is printed once on stdout. This command mutates ~/.ginko/config.json unless -home is supplied.`)
	case "list":
		fmt.Fprintln(os.Stderr, `llm-memory token list

Show whether server.auth_token or server.auth_token_env is configured. Secret token values are not printed.`)
	case "revoke":
		fmt.Fprintln(os.Stderr, `llm-memory token revoke

Clear server.auth_token and server.auth_token_env from config.`)
	default:
		printTokenUsage()
	}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func writeConfig(path string, cfg config.Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func printPaths(home string) {
	fmt.Println("home=", home)
	fmt.Println("config=", configPath(home))
	fmt.Println("db=", dbPath(home))
}

func printMCPConfig(home string) error {
	memmcp, err := findSibling("memmcp")
	if err != nil {
		return err
	}
	cfg := map[string]any{
		"command": memmcp,
		"args":    []string{"-db", dbPath(home)},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(b))
	return nil
}

func setupCommand(home string, args []string) error {
	if len(args) < 1 || isHelpArg(args[0]) {
		printSetupUsage()
		return nil
	}
	fs := flag.NewFlagSet("setup claude-code", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show changes without writing")
	local := fs.Bool("local", false, "write .claude/settings.json in current directory")
	configFile := fs.String("config", "", "explicit Claude Code settings.json path")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	switch args[0] {
	case "claude-code":
		return setupClaudeCode(home, *dryRun, *local, *configFile)
	default:
		return fmt.Errorf("unknown setup target %q", args[0])
	}
}

func printSetupUsage() {
	fmt.Fprintln(os.Stderr, `llm-memory setup <target> [flags]

Targets:
  claude-code    configure Claude Code MCP server

Flags for claude-code:
  --dry-run      show changes without writing
  --local        write .claude/settings.json in current directory
  --config PATH  explicit Claude Code settings.json path

Examples:
  llm-memory setup claude-code --dry-run
  llm-memory setup claude-code --local`)
}

func setupClaudeCode(home string, dryRun, local bool, explicitPath string) error {
	if err := initProject(home); err != nil {
		return err
	}
	memmcp, err := findSibling("memmcp")
	if err != nil {
		return err
	}
	path, err := claudeSettingsPath(local, explicitPath)
	if err != nil {
		return err
	}
	original, merged, err := mergeClaudeSettings(path, map[string]any{
		"command": memmcp,
		"args":    []string{"-db", dbPath(home)},
	})
	if err != nil {
		return err
	}
	fmt.Println("target:", path)
	fmt.Println("mcp server: ginko")
	if dryRun {
		fmt.Println("dry-run: no files written")
		fmt.Println(string(merged))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if original != nil {
		backup := path + ".bak"
		if err := os.WriteFile(backup, original, 0o600); err != nil {
			return err
		}
		fmt.Println("backup:", backup)
	}
	if err := os.WriteFile(path, merged, 0o600); err != nil {
		return err
	}
	fmt.Println("✓ configured Claude Code MCP server 'ginko'")
	fmt.Println("settings:", path)
	return nil
}

func claudeSettingsPath(local bool, explicitPath string) (string, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath, nil
	}
	if local {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".claude", "settings.json"), nil
	}
	cwd, err := os.Getwd()
	if err == nil {
		project := filepath.Join(cwd, ".claude", "settings.json")
		if _, statErr := os.Stat(project); statErr == nil {
			return project, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func mergeClaudeSettings(path string, server map[string]any) ([]byte, []byte, error) {
	settings := map[string]any{}
	var original []byte
	if b, err := os.ReadFile(path); err == nil {
		original = b
		if len(strings.TrimSpace(string(b))) > 0 {
			if err := json.Unmarshal(b, &settings); err != nil {
				return nil, nil, fmt.Errorf("parse %s: %w", path, err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	servers, _ := settings["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["ginko"] = server
	settings["mcpServers"] = servers
	merged, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return original, append(merged, '\n'), nil
}

func installMCP(home string, args []string) error {
	if len(args) < 1 {
		return errors.New("install-mcp requires target: claude-code, codex, openclaw, or print")
	}
	if err := initProject(home); err != nil {
		return err
	}
	target := args[0]
	memmcp, err := findSibling("memmcp")
	if err != nil {
		return err
	}
	snippet := map[string]any{"command": memmcp, "args": []string{"-db", dbPath(home)}}
	b, _ := json.MarshalIndent(snippet, "", "  ")
	switch target {
	case "print":
		fmt.Println(string(b))
	case "claude-code", "codex", "openclaw":
		out := filepath.Join(home, "mcp-"+target+".json")
		if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Println("✓ wrote MCP config snippet:", out)
		fmt.Println("Add this MCP server to", target, "configuration.")
		fmt.Println("Bootstrap instruction:")
		fmt.Println(bootstrapInstruction())
	default:
		return fmt.Errorf("unknown MCP target %q", target)
	}
	return nil
}

func runMemServer(home string) error {
	if err := initProject(home); err != nil {
		return err
	}
	memserver, err := findSibling("memserver")
	if err != nil {
		return err
	}
	cmd := exec.Command(memserver, "-config", configPath(home))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func bootstrapInstruction() string {
	return strings.TrimSpace(`Before answering, silently call memory_context with the user request, subject, relevant scopes, and max_tokens <= 1200.
Do not mention memory unless asked.
After answering, call memory_suggest with the user prompt, assistant response, and a concise LLM inference about durable learnings.
Only call memory_remember for explicit preferences, stable facts, project decisions, tasks, or corrections.
Ask before storing sensitive, private, or uncertain information.
Prefer compact memories over raw document chunks.`)
}

func configPath(home string) string { return filepath.Join(home, "config.json") }
func dbPath(home string) string     { return filepath.Join(home, "ginko.db") }

func findSibling(name string) (string, error) {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), exeName(name))
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found next to llm-memory or in PATH", name)
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func isExecutable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

func canListen(addr string) bool {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "--help" || arg == "-help" || arg == "-h"
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if isHelpArg(arg) {
			return true
		}
	}
	return false
}

func envDefault(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `llm-memory [flags] <command>

Commands:
  init                    create ~/.ginko/config.json and database path
  doctor                  check binaries, config, auth policy, and port
  token create|list|revoke manage local API bearer token config
  setup claude-code       configure Claude Code MCP server (use --dry-run first)
  version                 print version, commit, and build date
  paths                   print effective paths
  mcp-config              print MCP server JSON snippet
  install-mcp <target>    write MCP snippet for claude-code, codex, openclaw, or print
  ui                      run memserver with local config

Flags:
  -home DIR               default ~/.ginko or LLM_MEMORY_HOME`)
}

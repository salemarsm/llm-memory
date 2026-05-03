package main

import (
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
)

const defaultDirName = ".llm-memory"

func main() {
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
	fmt.Println("llm-memory doctor")
	fmt.Println("home:", home)
	checks := []struct{ name, path string }{
		{"config", configPath(home)},
		{"database dir", home},
	}
	for _, c := range checks {
		if _, err := os.Stat(c.path); err != nil {
			fmt.Printf("✗ %s: %s\n", c.name, err)
		} else {
			fmt.Printf("✓ %s: %s\n", c.name, c.path)
		}
	}
	for _, bin := range []string{"memserver", "memmcp", "memctl"} {
		p, err := findSibling(bin)
		if err != nil {
			fmt.Printf("✗ %s: %s\n", bin, err)
		} else {
			fmt.Printf("✓ %s: %s\n", bin, p)
		}
	}
	if canListen("127.0.0.1:8787") {
		fmt.Println("✓ port 127.0.0.1:8787 available")
	} else {
		fmt.Println("! port 127.0.0.1:8787 unavailable or already in use")
	}
	return nil
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
func dbPath(home string) string     { return filepath.Join(home, "memory.db") }

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
  init                    create ~/.llm-memory/config.json and database path
  doctor                  check binaries, config, and port
  paths                   print effective paths
  mcp-config              print MCP server JSON snippet
  install-mcp <target>    write MCP snippet for claude-code, codex, openclaw, or print
  ui                      run memserver with local config

Flags:
  -home DIR               default ~/.llm-memory or LLM_MEMORY_HOME`)
}

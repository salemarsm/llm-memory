package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/salemarsm/llm-memory/config"
)

type agentSetup struct {
	id          string
	label       string
	configPath  func() (string, error)
	install     func(dryRun, noBackup bool) error
	detected    bool
}

func buildAgents() []agentSetup {
	home, _ := os.UserHomeDir()
	agents := []agentSetup{
		{
			id:    "claude-code",
			label: "Claude Code",
			configPath: func() (string, error) { return resolveSettingsPath("user") },
			install: func(dryRun, noBackup bool) error {
				args := []string{}
				if dryRun {
					args = append(args, "--dry-run")
				}
				if noBackup {
					args = append(args, "--no-backup")
				}
				args = append(args, "--no-autostart")
				doSetupClaudeCode(args)
				return nil
			},
		},
		{
			id:    "openclaw",
			label: "OpenClaw",
			configPath: openclawConfigPath,
			install: func(dryRun, noBackup bool) error {
				return setupOpenClaw(dryRun, noBackup)
			},
		},
	}

	// Mark detected agents (config file or install dir exists)
	for i := range agents {
		if p, err := agents[i].configPath(); err == nil {
			if _, err := os.Stat(p); err == nil {
				agents[i].detected = true
				continue
			}
		}
		// Also check well-known install dirs
		switch agents[i].id {
		case "claude-code":
			if _, err := os.Stat(home + "/.claude"); err == nil {
				agents[i].detected = true
			}
		case "openclaw":
			if _, err := os.Stat(home + "/.openclaw"); err == nil {
				agents[i].detected = true
			}
		}
	}
	return agents
}

func doSetupUnified(args []string) {
	dryRun := hasFlag(args, "--dry-run")
	noBackup := hasFlag(args, "--no-backup")

	agents := buildAgents()
	dbPath := config.DefaultDBPath()

	fmt.Println("ginko setup — shared memory for AI agents")
	fmt.Println()
	fmt.Printf("Canonical database: %s\n", dbPath)
	fmt.Println("All configured agents will share this database.")
	fmt.Println()

	// Show detected agents and let user choose
	selected := selectAgents(agents)
	if len(selected) == 0 {
		fmt.Println("No agents selected. Nothing to do.")
		return
	}

	fmt.Println()

	// Configure each selected agent
	ok := 0
	for _, a := range selected {
		fmt.Printf("Configuring %s...\n", a.label)
		if err := a.install(dryRun, noBackup); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		} else {
			ok++
		}
	}

	if dryRun || ok == 0 {
		return
	}

	fmt.Println()
	fmt.Printf("Done. %d agent(s) configured.\n", ok)
	fmt.Println()
	fmt.Println("Next steps:")
	for _, a := range selected {
		switch a.id {
		case "claude-code":
			fmt.Println("  Claude Code: restart Claude Code to load the MCP server")
		case "openclaw":
			fmt.Println("  OpenClaw:    restart OpenClaw to load the MCP server")
		}
	}

	if !stdinIsTerminal() {
		return
	}
	fmt.Println()
	fmt.Print("Would you like ginko serve (web GUI) to start automatically at login? [y/N] ")
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	if answer := strings.TrimSpace(strings.ToLower(sc.Text())); answer == "y" || answer == "yes" {
		if err := installAutostart(); err != nil {
			fmt.Fprintf(os.Stderr, "autostart setup failed: %v\n", err)
			fmt.Println("Start manually with: ginko serve")
		}
	}
}

func selectAgents(agents []agentSetup) []agentSetup {
	if !stdinIsTerminal() {
		// Non-interactive: install all detected agents
		var out []agentSetup
		for _, a := range agents {
			if a.detected {
				out = append(out, a)
			}
		}
		if len(out) == 0 {
			out = agents // fallback: install all
		}
		return out
	}

	fmt.Println("Detected agents (press Enter to accept, or type custom selection):")
	fmt.Println()
	for i, a := range agents {
		status := "  "
		if a.detected {
			status = "✓ "
		}
		p, _ := a.configPath()
		fmt.Printf("  [%d] %s%s  (%s)\n", i+1, status, a.label, p)
	}
	fmt.Println()
	fmt.Println("  ✓ = detected on this machine")
	fmt.Println()

	// Build default selection from detected
	var defaults []string
	for i, a := range agents {
		if a.detected {
			defaults = append(defaults, fmt.Sprintf("%d", i+1))
		}
	}
	defaultStr := strings.Join(defaults, ",")
	if defaultStr == "" {
		defaultStr = "all"
	}

	fmt.Printf("Select agents [%s]: ", defaultStr)
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	input := strings.TrimSpace(sc.Text())
	if input == "" {
		input = defaultStr
	}

	if input == "all" || input == "a" {
		return agents
	}
	if input == "none" || input == "0" {
		return nil
	}

	var selected []agentSetup
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err == nil && idx >= 1 && idx <= len(agents) {
			selected = append(selected, agents[idx-1])
		}
	}
	return selected
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

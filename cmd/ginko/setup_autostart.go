package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

func askAndInstallAutostart() {
	if !stdinIsTerminal() {
		return
	}
	fmt.Print("\nWould you like ginko serve (web GUI) to start automatically at login? [y/N] ")
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	answer := strings.TrimSpace(strings.ToLower(sc.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Println("You can start it manually any time with: ginko serve")
		return
	}
	if err := installAutostart(); err != nil {
		fmt.Fprintf(os.Stderr, "autostart setup failed: %v\n", err)
		fmt.Println("You can start it manually with: ginko serve")
		return
	}
}

func installAutostart() error {
	bin, err := resolveGinkoBin()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "linux":
		return installSystemdUserService(bin)
	case "darwin":
		return installLaunchdAgent(bin)
	default:
		fmt.Printf("Autostart not supported on %s. Start manually with: ginko serve\n", runtime.GOOS)
		return nil
	}
}

func resolveGinkoBin() (string, error) {
	if exe, err := os.Executable(); err == nil {
		abs, err := filepath.Abs(exe)
		if err == nil {
			return abs, nil
		}
	}
	if path, err := exec.LookPath("ginko"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("cannot locate ginko binary")
}

const systemdUnit = `[Unit]
Description=ginko memory server (web GUI)
After=network.target

[Service]
ExecStart={{.Bin}} serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

func installSystemdUserService(bin string) error {
	dir := filepath.Join(mustHomeDir(), ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	unitPath := filepath.Join(dir, "ginko-serve.service")
	f, err := os.Create(unitPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := template.Must(template.New("").Parse(systemdUnit)).Execute(f, map[string]string{"Bin": bin}); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", "--now", "ginko-serve").Run(); err != nil {
		fmt.Printf("Service file written to %s\n", unitPath)
		fmt.Println("Enable manually with: systemctl --user enable --now ginko-serve")
		return nil
	}
	fmt.Printf("ginko serve enabled as systemd user service — listening on http://127.0.0.1:8787\n")
	return nil
}

const launchdPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.ginko.serve</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Bin}}</string>
		<string>serve</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.Home}}/Library/Logs/ginko-serve.log</string>
	<key>StandardErrorPath</key>
	<string>{{.Home}}/Library/Logs/ginko-serve.log</string>
</dict>
</plist>
`

func installLaunchdAgent(bin string) error {
	home := mustHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	plistPath := filepath.Join(dir, "com.ginko.serve.plist")
	f, err := os.Create(plistPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := template.Must(template.New("").Parse(launchdPlist)).Execute(f, map[string]string{
		"Bin":  bin,
		"Home": home,
	}); err != nil {
		return err
	}
	if err := exec.Command("launchctl", "load", "-w", plistPath).Run(); err != nil {
		fmt.Printf("Agent plist written to %s\n", plistPath)
		fmt.Printf("Enable manually with: launchctl load -w %s\n", plistPath)
		return nil
	}
	fmt.Printf("ginko serve enabled as launchd agent — listening on http://127.0.0.1:8787\n")
	return nil
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func mustHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return home
}

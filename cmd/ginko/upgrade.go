package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/salemarsm/llm-memory/internal/version"
)

const upgradeRepo = "salemarsm/llm-memory"

func doUpgrade(args []string) {
	fmt.Println("Checking for updates...")

	latest, err := fetchLatestTag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: could not fetch latest release: %v\n", err)
		printManualUpgrade()
		os.Exit(1)
	}

	current := strings.TrimPrefix(version.Version, "v")
	latestClean := strings.TrimPrefix(latest, "v")

	if current == "dev" {
		fmt.Printf("Current: dev build\nLatest:  %s\n\n", latest)
	} else if current == latestClean {
		fmt.Printf("Already up to date (%s).\n", latest)
		return
	} else {
		fmt.Printf("Current: %s\nLatest:  %s\n\n", version.Version, latest)
	}

	if err := runInstallScript(latest); err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
		printManualUpgrade()
		os.Exit(1)
	}
}

func fetchLatestTag() (string, error) {
	url := "https://api.github.com/repos/" + upgradeRepo + "/releases/latest"
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("no releases published yet")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API status %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("empty tag_name in response")
	}
	return release.TagName, nil
}

func runInstallScript(tag string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("auto-upgrade not supported on Windows; download from https://github.com/%s/releases", upgradeRepo)
	}

	// try curl | bash
	curlPath, err := exec.LookPath("curl")
	if err != nil {
		return fmt.Errorf("curl not found; install manually")
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("bash not found; install manually")
	}

	scriptURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/install.sh", upgradeRepo, tag)
	fmt.Printf("Running install script for %s...\n\n", tag)

	curlCmd := exec.Command(curlPath, "-fsSL", scriptURL)
	bashCmd := exec.Command(bashPath)
	bashCmd.Env = append(os.Environ(), "GINKO_VERSION="+tag)
	bashCmd.Stdin, _ = curlCmd.StdoutPipe()
	bashCmd.Stdout = os.Stdout
	bashCmd.Stderr = os.Stderr

	if err := curlCmd.Start(); err != nil {
		return err
	}
	if err := bashCmd.Start(); err != nil {
		curlCmd.Process.Kill() //nolint:errcheck
		return err
	}
	curlCmd.Wait() //nolint:errcheck
	return bashCmd.Wait()
}

func printManualUpgrade() {
	fmt.Fprintf(os.Stderr, "\nManual upgrade:\n")
	fmt.Fprintf(os.Stderr, "  curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | bash\n", upgradeRepo)
	fmt.Fprintf(os.Stderr, "or visit: https://github.com/%s/releases\n", upgradeRepo)
}

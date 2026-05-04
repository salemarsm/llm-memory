package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var projectSepRE = regexp.MustCompile(`[^a-z0-9._/-]+`)
var projectCollapseRE = regexp.MustCompile(`[-_/]+`)

type projectConfig struct {
	Project string `json:"project"`
	Subject string `json:"subject"`
}

func DetectProject(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	if p := detectProjectConfig(cwd); p != "" {
		return p
	}
	if p := detectGitRemote(cwd); p != "" {
		return p
	}
	base := filepath.Base(cwd)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "default"
	}
	return NormalizeProject(base)
}

func NormalizeProject(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, ".git")
	s = projectSepRE.ReplaceAllString(s, "-")
	s = strings.Trim(projectCollapseRE.ReplaceAllString(s, "-"), "-._/")
	if s == "" {
		return "default"
	}
	return s
}

func detectProjectConfig(cwd string) string {
	for _, dir := range walkParents(cwd) {
		b, err := os.ReadFile(filepath.Join(dir, ".llm-memory", "config.json"))
		if err != nil {
			continue
		}
		var cfg projectConfig
		if json.Unmarshal(b, &cfg) == nil {
			if p := NormalizeProject(firstNonEmpty(cfg.Project, cfg.Subject)); p != "" {
				return p
			}
		}
	}
	return ""
}

func detectGitRemote(cwd string) string {
	for _, dir := range walkParents(cwd) {
		b, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
		if err != nil {
			continue
		}
		if p := parseGitRemote(string(b)); p != "" {
			return p
		}
	}
	return ""
}

func parseGitRemote(config string) string {
	inOrigin := false
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = strings.Contains(trimmed, `remote "origin"`)
			continue
		}
		if !inOrigin || !strings.HasPrefix(trimmed, "url") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		return NormalizeProject(cleanRemoteURL(parts[1]))
	}
	return ""
}

func cleanRemoteURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		rest := s[i+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return rest[slash+1:]
		}
	}
	if strings.HasPrefix(s, "git@") {
		if colon := strings.Index(s, ":"); colon >= 0 {
			return s[colon+1:]
		}
	}
	return s
}

func walkParents(cwd string) []string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	out := []string{}
	for {
		out = append(out, abs)
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

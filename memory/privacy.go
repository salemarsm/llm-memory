package memory

import (
	"regexp"
	"strings"
)

var privateTagRE = regexp.MustCompile(`(?is)<private>.*?</private>`)

// StripPrivateTags removes content explicitly marked as private before it can
// become canonical memory. It is intentionally deterministic and provider-free.
func StripPrivateTags(s string) string {
	return compactWhitespace(privateTagRE.ReplaceAllString(s, ""))
}

// ContainsPrivateTags reports whether text contains a private block marker.
func ContainsPrivateTags(s string) bool {
	return strings.Contains(strings.ToLower(s), "<private>") || strings.Contains(strings.ToLower(s), "</private>")
}

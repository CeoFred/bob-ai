package security

import (
	"regexp"
	"strings"
)

var (
	bearerRegex = regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9_\-\.]{8,})`)
	tokenRegex  = regexp.MustCompile(`(?i)(api[_-]?key|token|password|secret|auth)[=:\s]+(["']?[A-Za-z0-9_\-\.]{8,}["']?)`)
)

// Redactor masks secrets and credentials from logs and audit trails.
type Redactor struct {
	knownTokens []string
}

// NewRedactor creates a redactor with optional known tokens to explicitly mask.
func NewRedactor(knownTokens ...string) *Redactor {
	cleanTokens := make([]string, 0, len(knownTokens))
	for _, t := range knownTokens {
		trimmed := strings.TrimSpace(t)
		if len(trimmed) > 3 {
			cleanTokens = append(cleanTokens, trimmed)
		}
	}
	return &Redactor{knownTokens: cleanTokens}
}

// Redact sanitizes sensitive substrings in arbitrary text.
func (r *Redactor) Redact(input string) string {
	if input == "" {
		return ""
	}

	result := input

	// Redact known tokens
	for _, tok := range r.knownTokens {
		result = strings.ReplaceAll(result, tok, "[REDACTED_TOKEN]")
	}

	// Redact Bearer headers
	result = bearerRegex.ReplaceAllString(result, "${1}[REDACTED]")

	// Redact key=val or password: val
	result = tokenRegex.ReplaceAllString(result, "${1}=[REDACTED]")

	return result
}

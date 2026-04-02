package services

import (
	"regexp"
	"strings"
)

var (
	// pemPattern matches PEM-encoded certificates and private keys.
	pemPattern = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[\s\S]*?-----END [A-Z ]+-----`)

	// sensitiveKeywords matches env var names that likely contain credentials.
	sensitiveKeywords = regexp.MustCompile(`(?i)(password|passwd|secret|token|apikey|api_key|credential|private.?key|access.?key|auth)`)

	// envNameValuePattern matches a K8s env block:
	//   - name: SENSITIVE_VAR\n    value: <anything>
	// Captures: prefix (through "value: "), value
	envNameValuePattern = regexp.MustCompile(`(?m)(- name:\s*\S*(?:password|passwd|secret|token|apikey|api_key|credential|private.?key|access.?key|auth)\S*\s*\n\s+)(value:\s*)(.*)`)
)

// SanitizeK8sContext removes sensitive data from a K8s resource string
// (YAML or JSON serialized text) before sending it to an external AI provider.
//
// Redacts:
//   - PEM certificates and private keys
//   - Secret data / stringData block values (base64 encoded secrets)
//   - Env var values where the var name contains password/token/key/secret
func SanitizeK8sContext(input string) string {
	// 1. Redact PEM certificates / private keys
	result := pemPattern.ReplaceAllString(input, "[REDACTED: certificate]")

	// 2. Redact env var values with sensitive names
	result = envNameValuePattern.ReplaceAllString(result, "${1}${2}[REDACTED]")

	// 3. Redact Secret data / stringData block values line by line
	result = redactSecretDataBlocks(result)

	return result
}

// redactSecretDataBlocks redacts values in K8s Secret data: and stringData: blocks.
func redactSecretDataBlocks(input string) string {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))

	inDataBlock := false
	dataIndent := -1

	for _, line := range lines {
		if line == "" {
			out = append(out, line)
			continue
		}

		// Measure current line indent
		trimmed := strings.TrimLeft(line, " \t")
		indent := len(line) - len(trimmed)

		// Detect data: / stringData: block
		if trimmed == "data:" || trimmed == "stringData:" {
			inDataBlock = true
			dataIndent = indent
			out = append(out, line)
			continue
		}

		if inDataBlock {
			// Exit block when indent goes back to same level or less
			if indent <= dataIndent && trimmed != "" {
				inDataBlock = false
				// fall through to normal append
			} else {
				// Inside data block: redact value portion
				if colonIdx := strings.Index(trimmed, ":"); colonIdx > 0 {
					key := trimmed[:colonIdx]
					spaces := strings.Repeat(" ", indent)
					out = append(out, spaces+key+": [REDACTED]")
				} else {
					out = append(out, line)
				}
				continue
			}
		}

		// Fallback: redact lines that look like "key: value" where key is sensitive
		// and it's NOT in a data block (already handled above).
		if sensitiveKeywords.MatchString(trimmed) && strings.Contains(trimmed, ":") {
			colonIdx := strings.Index(trimmed, ":")
			key := trimmed[:colonIdx]
			// Only redact if value part is non-empty and looks like an inline value
			after := strings.TrimSpace(trimmed[colonIdx+1:])
			if after != "" && !strings.HasPrefix(after, "{") && !strings.HasPrefix(after, "[") {
				spaces := strings.Repeat(" ", indent)
				out = append(out, spaces+key+": [REDACTED]")
				continue
			}
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

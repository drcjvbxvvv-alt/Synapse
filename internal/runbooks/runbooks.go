package runbooks

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed runbooks.json
var data []byte

// Runbook represents an operational runbook for a specific K8s failure scenario.
type Runbook struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Reasons  []string `json:"reasons"`
	Keywords []string `json:"keywords"`
	Summary  string   `json:"summary"`
	Steps    []string `json:"steps"`
}

// All contains all built-in runbooks loaded at startup.
var All []Runbook

func init() {
	if err := json.Unmarshal(data, &All); err != nil {
		panic("failed to parse runbooks.json: " + err.Error())
	}
}

// Search returns runbooks that match the given query (reason code or keyword).
// The match is case-insensitive and checks reasons and keywords fields.
func Search(query string) []Runbook {
	if query == "" {
		return All
	}
	q := strings.ToLower(query)
	var result []Runbook
	for _, rb := range All {
		if matchesRunbook(rb, q) {
			result = append(result, rb)
		}
	}
	return result
}

func matchesRunbook(rb Runbook, q string) bool {
	for _, r := range rb.Reasons {
		if strings.ToLower(r) == q {
			return true
		}
	}
	for _, k := range rb.Keywords {
		if strings.Contains(strings.ToLower(k), q) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(rb.Title), q) {
		return true
	}
	return false
}

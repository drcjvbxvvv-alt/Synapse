package models

import (
	"encoding/json"
	"strings"
)

// jsonMarshal is a local alias kept in a separate test file so that production
// code does not need to import encoding/json for any reason.
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// containsAny returns true if s contains any of the provided substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

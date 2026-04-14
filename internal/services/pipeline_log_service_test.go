package services

import (
	"strings"
	"testing"
)

func TestScrubSecrets_BasicReplacement(t *testing.T) {
	content := "connecting to db with mysecretvalue and mytoken123"
	secrets := []string{"mysecretvalue", "mytoken123"}

	result := ScrubSecrets(content, secrets)

	if result != "connecting to db with ***REDACTED*** and ***REDACTED***" {
		t.Errorf("unexpected scrub result: %s", result)
	}
}

func TestScrubSecrets_NoSecrets(t *testing.T) {
	content := "hello world"
	result := ScrubSecrets(content, nil)
	if result != "hello world" {
		t.Errorf("expected unchanged content, got: %s", result)
	}
}

func TestScrubSecrets_EmptySecretIgnored(t *testing.T) {
	content := "hello world"
	result := ScrubSecrets(content, []string{"", ""})
	if result != "hello world" {
		t.Errorf("expected unchanged content, got: %s", result)
	}
}

func TestScrubSecrets_MultipleOccurrences(t *testing.T) {
	content := "key=SECRET val=SECRET end"
	result := ScrubSecrets(content, []string{"SECRET"})
	if result != "key=***REDACTED*** val=***REDACTED*** end" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestScrubSecrets_OverlappingSecrets(t *testing.T) {
	// If one secret is a substring of another, both should be scrubbed
	content := "value: my-long-secret-value"
	secrets := []string{"my-long-secret-value", "secret"}

	result := ScrubSecrets(content, secrets)
	// First replacement makes it: "value: ***REDACTED***"
	// "secret" substring already gone
	if result != "value: ***REDACTED***" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestScrubSecrets_MultilineContent(t *testing.T) {
	content := "line1\ndb_host=mysecret\nline3"
	result := ScrubSecrets(content, []string{"mysecret"})
	expected := "line1\ndb_host=***REDACTED***\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestScrubSecrets_PatternBasedDetection(t *testing.T) {
	// secretPattern regex should catch password=xxx, token=xxx etc.
	content := "connecting with password=SuperSecret123 and api_key=abcdef"
	result := ScrubSecrets(content, nil) // no explicit secrets, rely on pattern

	if result == content {
		t.Error("expected pattern-based scrubbing to redact password= and api_key= patterns")
	}
	if strings.Contains(result, "SuperSecret123") {
		t.Error("password value should have been scrubbed by pattern")
	}
	if strings.Contains(result, "abcdef") {
		t.Error("api_key value should have been scrubbed by pattern")
	}
}

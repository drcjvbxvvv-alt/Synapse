package services

import (
	"testing"
)

func TestScrubSecrets_BasicReplacement(t *testing.T) {
	content := "connecting to db with password=SuperSecret123 and token=abc-def-ghi"
	secrets := []string{"SuperSecret123", "abc-def-ghi"}

	result := ScrubSecrets(content, secrets)

	if result != "connecting to db with password=***REDACTED*** and token=***REDACTED***" {
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
	content := "token: my-long-secret-value"
	secrets := []string{"my-long-secret-value", "secret"}

	result := ScrubSecrets(content, secrets)
	// First replacement makes it: "token: ***REDACTED***"
	// Second replacement has nothing left to replace
	if result != "token: ***REDACTED***" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestScrubSecrets_MultilineContent(t *testing.T) {
	content := "line1\npassword=mysecret\nline3"
	result := ScrubSecrets(content, []string{"mysecret"})
	expected := "line1\npassword=***REDACTED***\nline3"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

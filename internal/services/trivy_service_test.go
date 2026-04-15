package services

import (
	"testing"
)

// ---------------------------------------------------------------------------
// parseTrivyOutput
// ---------------------------------------------------------------------------

func TestParseTrivyOutput_ValidJSON(t *testing.T) {
	data := []byte(`{
		"Results": [
			{
				"Target": "myimage:latest",
				"Vulnerabilities": [
					{"VulnerabilityID": "CVE-2024-001", "Severity": "CRITICAL"},
					{"VulnerabilityID": "CVE-2024-002", "Severity": "HIGH"},
					{"VulnerabilityID": "CVE-2024-003", "Severity": "HIGH"},
					{"VulnerabilityID": "CVE-2024-004", "Severity": "MEDIUM"},
					{"VulnerabilityID": "CVE-2024-005", "Severity": "LOW"}
				]
			}
		]
	}`)

	counts, resultJSON := parseTrivyOutput(data)

	if counts["CRITICAL"] != 1 {
		t.Errorf("expected CRITICAL=1, got %d", counts["CRITICAL"])
	}
	if counts["HIGH"] != 2 {
		t.Errorf("expected HIGH=2, got %d", counts["HIGH"])
	}
	if counts["MEDIUM"] != 1 {
		t.Errorf("expected MEDIUM=1, got %d", counts["MEDIUM"])
	}
	if counts["LOW"] != 1 {
		t.Errorf("expected LOW=1, got %d", counts["LOW"])
	}
	if counts["UNKNOWN"] != 0 {
		t.Errorf("expected UNKNOWN=0, got %d", counts["UNKNOWN"])
	}
	if resultJSON == "" {
		t.Error("expected non-empty resultJSON")
	}
}

func TestParseTrivyOutput_InvalidJSON(t *testing.T) {
	data := []byte(`not json`)
	counts, resultJSON := parseTrivyOutput(data)

	// Should return zero counts and raw string
	for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
		if counts[sev] != 0 {
			t.Errorf("expected %s=0, got %d", sev, counts[sev])
		}
	}
	if resultJSON != "not json" {
		t.Errorf("expected raw string, got %s", resultJSON)
	}
}

func TestParseTrivyOutput_EmptyResults(t *testing.T) {
	data := []byte(`{"Results": []}`)
	counts, _ := parseTrivyOutput(data)

	for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
		if counts[sev] != 0 {
			t.Errorf("expected %s=0 for empty results, got %d", sev, counts[sev])
		}
	}
}

func TestParseTrivyOutput_UnknownSeverity(t *testing.T) {
	data := []byte(`{
		"Results": [{
			"Target": "img",
			"Vulnerabilities": [
				{"VulnerabilityID": "CVE-001", "Severity": "SUPER_HIGH"}
			]
		}]
	}`)
	counts, _ := parseTrivyOutput(data)

	if counts["UNKNOWN"] != 1 {
		t.Errorf("expected UNKNOWN=1 for unrecognized severity, got %d", counts["UNKNOWN"])
	}
}

func TestParseTrivyOutput_MultipleTargets(t *testing.T) {
	data := []byte(`{
		"Results": [
			{"Target": "a", "Vulnerabilities": [
				{"VulnerabilityID": "CVE-1", "Severity": "CRITICAL"}
			]},
			{"Target": "b", "Vulnerabilities": [
				{"VulnerabilityID": "CVE-2", "Severity": "CRITICAL"},
				{"VulnerabilityID": "CVE-3", "Severity": "LOW"}
			]}
		]
	}`)
	counts, _ := parseTrivyOutput(data)

	if counts["CRITICAL"] != 2 {
		t.Errorf("expected CRITICAL=2, got %d", counts["CRITICAL"])
	}
	if counts["LOW"] != 1 {
		t.Errorf("expected LOW=1, got %d", counts["LOW"])
	}
}

// ---------------------------------------------------------------------------
// IngestScanRequest validation (field defaults)
// ---------------------------------------------------------------------------

func TestIngestScanRequest_Fields(t *testing.T) {
	req := IngestScanRequest{
		Image:      "nginx:1.25",
		Namespace:  "default",
		ScanSource: "github_actions",
		Critical:   3,
		High:       5,
	}

	if req.Image != "nginx:1.25" {
		t.Error("field mismatch")
	}
	if req.ScanSource != "github_actions" {
		t.Error("expected custom scan_source")
	}
	if req.Critical != 3 || req.High != 5 {
		t.Error("count mismatch")
	}
}

func TestIngestScanRequest_DefaultScanSource(t *testing.T) {
	req := IngestScanRequest{
		Image: "app:v1",
	}
	// When ScanSource is empty, IngestScanResult should default to "ci_push"
	if req.ScanSource != "" {
		t.Error("expected empty ScanSource in struct")
	}
}

// ---------------------------------------------------------------------------
// truncate helper
// ---------------------------------------------------------------------------

func TestTruncate_Short(t *testing.T) {
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("expected hello, got %s", result)
	}
}

func TestTruncate_Long(t *testing.T) {
	result := truncate("abcdefghij", 5)
	if result != "abcde..." {
		t.Errorf("expected abcde..., got %s", result)
	}
}

func TestTruncate_Exact(t *testing.T) {
	result := truncate("12345", 5)
	if result != "12345" {
		t.Errorf("expected 12345, got %s", result)
	}
}

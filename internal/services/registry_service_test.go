package services

import (
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// validateRegistryType
// ---------------------------------------------------------------------------

func TestValidateRegistryType_Valid(t *testing.T) {
	for _, rt := range []string{"harbor", "dockerhub", "acr", "ecr", "gcr"} {
		if err := validateRegistryType(rt); err != nil {
			t.Errorf("expected valid for %s: %v", rt, err)
		}
	}
}

func TestValidateRegistryType_Invalid(t *testing.T) {
	if err := validateRegistryType("quay"); err == nil {
		t.Error("expected error for unsupported registry type")
	}
}

// ---------------------------------------------------------------------------
// NewRegistryAdapter factory
// ---------------------------------------------------------------------------

func TestNewRegistryAdapter_Valid(t *testing.T) {
	for _, rt := range []string{"harbor", "dockerhub", "acr", "ecr", "gcr"} {
		reg := &models.Registry{
			Type: rt,
			URL:  "https://registry.example.com",
		}
		adapter, err := NewRegistryAdapter(reg)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", rt, err)
		}
		if adapter == nil {
			t.Errorf("expected non-nil adapter for %s", rt)
		}
	}
}

func TestNewRegistryAdapter_Invalid(t *testing.T) {
	reg := &models.Registry{Type: "quay", URL: "https://quay.io"}
	_, err := NewRegistryAdapter(reg)
	if err == nil {
		t.Error("expected error for unsupported registry type")
	}
}

// ---------------------------------------------------------------------------
// Adapter type assertions
// ---------------------------------------------------------------------------

func TestNewRegistryAdapter_HarborType(t *testing.T) {
	reg := &models.Registry{
		Type:           "harbor",
		URL:            "https://harbor.example.com",
		DefaultProject: "library",
	}
	adapter, err := NewRegistryAdapter(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ha, ok := adapter.(*HarborAdapter)
	if !ok {
		t.Fatal("expected *HarborAdapter")
	}
	if ha.defaultProject != "library" {
		t.Errorf("expected library, got %s", ha.defaultProject)
	}
}

func TestNewRegistryAdapter_DockerHubType(t *testing.T) {
	reg := &models.Registry{Type: "dockerhub", URL: "https://registry-1.docker.io"}
	adapter, _ := NewRegistryAdapter(reg)
	if _, ok := adapter.(*DockerHubAdapter); !ok {
		t.Error("expected *DockerHubAdapter")
	}
}

func TestNewRegistryAdapter_ECRUsesDockerV2(t *testing.T) {
	reg := &models.Registry{Type: "ecr", URL: "https://123456.dkr.ecr.us-east-1.amazonaws.com"}
	adapter, _ := NewRegistryAdapter(reg)
	if _, ok := adapter.(*DockerV2Adapter); !ok {
		t.Error("expected *DockerV2Adapter for ECR")
	}
}

func TestNewRegistryAdapter_GCRUsesDockerV2(t *testing.T) {
	reg := &models.Registry{Type: "gcr", URL: "https://gcr.io"}
	adapter, _ := NewRegistryAdapter(reg)
	if _, ok := adapter.(*DockerV2Adapter); !ok {
		t.Error("expected *DockerV2Adapter for GCR")
	}
}

// ---------------------------------------------------------------------------
// buildRegistryHTTPClient
// ---------------------------------------------------------------------------

func TestBuildRegistryHTTPClient_InsecureTLS(t *testing.T) {
	reg := &models.Registry{
		URL:         "https://registry.example.com",
		InsecureTLS: true,
	}
	client := buildRegistryHTTPClient(reg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout == 0 {
		t.Error("expected non-zero timeout")
	}
}

func TestBuildRegistryHTTPClient_SecureTLS(t *testing.T) {
	reg := &models.Registry{
		URL:         "https://registry.example.com",
		InsecureTLS: false,
	}
	client := buildRegistryHTTPClient(reg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// ---------------------------------------------------------------------------
// NewRegistryService
// ---------------------------------------------------------------------------

func TestNewRegistryService(t *testing.T) {
	svc := NewRegistryService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------------------------------------------------------------------------
// registryBase URL trimming
// ---------------------------------------------------------------------------

func TestRegistryBase_URLTrimming(t *testing.T) {
	reg := &models.Registry{
		Type: "harbor",
		URL:  "https://harbor.example.com/",
	}
	adapter, err := NewRegistryAdapter(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ha := adapter.(*HarborAdapter)
	if ha.url != "https://harbor.example.com" {
		t.Errorf("expected trailing slash trimmed, got %s", ha.url)
	}
}

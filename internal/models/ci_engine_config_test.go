package models

import (
	"testing"
)

func TestCIEngineConfig_TableName(t *testing.T) {
	if got := (CIEngineConfig{}).TableName(); got != "ci_engine_configs" {
		t.Fatalf("TableName = %q, want %q", got, "ci_engine_configs")
	}
}

func TestCIEngineConfig_AuthConstants(t *testing.T) {
	// Locks in the public auth-type constant values — other packages pattern
	// match on these strings, so accidental renames would be breaking.
	cases := map[string]string{
		CIEngineAuthToken:       "token",
		CIEngineAuthBasic:       "basic",
		CIEngineAuthKubeconfig:  "kubeconfig",
		CIEngineAuthServiceAcct: "service_acct",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("auth constant drift: got %q, want %q", got, want)
		}
	}
}

func TestCIEngineConfigRequest_ApplyTo(t *testing.T) {
	tru := true
	req := &CIEngineConfigRequest{
		Name:       "gitlab-new",
		EngineType: "gitlab",
		Enabled:    &tru,
		Endpoint:   "https://gitlab.example.com",
		AuthType:   CIEngineAuthToken,
		Token:      "pat-123",
	}
	m := &CIEngineConfig{}
	req.ApplyTo(m)

	if m.Name != "gitlab-new" {
		t.Fatalf("Name not copied")
	}
	if !m.Enabled {
		t.Fatalf("Enabled not copied")
	}
	if m.Token != "pat-123" {
		t.Fatalf("Token not copied")
	}
	if m.Endpoint != "https://gitlab.example.com" {
		t.Fatalf("Endpoint not copied")
	}
	if m.AuthType != CIEngineAuthToken {
		t.Fatalf("AuthType not copied")
	}
}

func TestCIEngineConfigRequest_ApplyTo_PreservesExistingSecrets(t *testing.T) {
	// Empty credential strings in the request MUST NOT clobber existing
	// stored values — this is how the UI supports "edit without re-entering
	// the password".
	req := &CIEngineConfigRequest{
		Name:       "gitlab-edit",
		EngineType: "gitlab",
		Endpoint:   "https://gitlab.example.com",
		// Token / Password / WebhookSecret intentionally empty.
	}
	m := &CIEngineConfig{
		Token:         "existing-token",
		Password:      "existing-password",
		WebhookSecret: "existing-whsec",
		CABundle:      "existing-ca",
	}
	req.ApplyTo(m)

	if m.Token != "existing-token" {
		t.Fatalf("Token clobbered: got %q", m.Token)
	}
	if m.Password != "existing-password" {
		t.Fatalf("Password clobbered: got %q", m.Password)
	}
	if m.WebhookSecret != "existing-whsec" {
		t.Fatalf("WebhookSecret clobbered: got %q", m.WebhookSecret)
	}
	if m.CABundle != "existing-ca" {
		t.Fatalf("CABundle clobbered: got %q", m.CABundle)
	}
}

func TestCIEngineConfigRequest_ApplyTo_EnabledOptional(t *testing.T) {
	// Absence of Enabled in the request should not flip the stored value.
	req := &CIEngineConfigRequest{Name: "x", EngineType: "gitlab"}
	m := &CIEngineConfig{Enabled: true}
	req.ApplyTo(m)
	if !m.Enabled {
		t.Fatalf("Enabled changed despite nil in request")
	}
}

func TestCIEngineConfigRequest_ApplyTo_RespectsClusterID(t *testing.T) {
	id := uint(7)
	req := &CIEngineConfigRequest{
		Name:       "tekton",
		EngineType: "tekton",
		ClusterID:  &id,
	}
	m := &CIEngineConfig{}
	req.ApplyTo(m)
	if m.ClusterID == nil || *m.ClusterID != 7 {
		t.Fatalf("ClusterID not propagated: %v", m.ClusterID)
	}
}

func TestCIEngineConfig_SensitiveFieldsAreJSONHidden(t *testing.T) {
	// Regression guard: credentials must never appear in JSON responses.
	cfg := &CIEngineConfig{
		Name:          "x",
		EngineType:    "gitlab",
		Token:         "SECRET",
		Password:      "SECRET",
		WebhookSecret: "SECRET",
		CABundle:      "SECRET",
	}
	buf, err := jsonMarshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if containsAny(string(buf), []string{`"token"`, `"password"`, `"webhook_secret"`, `"ca_bundle"`}) {
		t.Fatalf("sensitive field key leaked in JSON: %s", buf)
	}
	if containsAny(string(buf), []string{"SECRET"}) {
		t.Fatalf("sensitive value leaked in JSON: %s", buf)
	}
}

func TestCIEngineConfig_PublicFieldsAreJSONExposed(t *testing.T) {
	cfg := &CIEngineConfig{
		Name:       "ui-visible",
		EngineType: "gitlab",
		Endpoint:   "https://gitlab.example.com",
		AuthType:   CIEngineAuthToken,
		Enabled:    true,
	}
	buf, err := jsonMarshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, needle := range []string{
		`"name"`, `"engine_type"`, `"endpoint"`, `"auth_type"`, `"enabled"`,
	} {
		if !containsAny(string(buf), []string{needle}) {
			t.Fatalf("public key %q missing from JSON: %s", needle, buf)
		}
	}
}

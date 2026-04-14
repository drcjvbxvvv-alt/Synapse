package services

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ValidateStepDef tests
// ---------------------------------------------------------------------------

func TestValidateStepDef_RunScript_Valid(t *testing.T) {
	step := &StepDef{
		Name:    "test-script",
		Type:    "run-script",
		Image:   "alpine:3.20",
		Command: "echo hello",
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_RunScript_MissingCommand(t *testing.T) {
	step := &StepDef{
		Name:  "test-script",
		Type:  "run-script",
		Image: "alpine:3.20",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStepDef_BuildImage_Valid(t *testing.T) {
	cfg := BuildImageConfig{
		Context:     ".",
		Dockerfile:  "Dockerfile",
		Destination: "harbor.example.com/app:v1",
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "build",
		Type:   "build-image",
		Config: string(cfgJSON),
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_BuildImage_MissingDestination(t *testing.T) {
	cfg := BuildImageConfig{
		Context:    ".",
		Dockerfile: "Dockerfile",
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "build",
		Type:   "build-image",
		Config: string(cfgJSON),
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing destination")
	}
	if !strings.Contains(err.Error(), "destination is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStepDef_BuildImage_NoConfig(t *testing.T) {
	step := &StepDef{
		Name: "build",
		Type: "build-image",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestValidateStepDef_Deploy_WithManifest(t *testing.T) {
	cfg := DeployConfig{
		Manifest:  "k8s/deployment.yaml",
		Namespace: "staging",
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "deploy",
		Type:   "deploy",
		Config: string(cfgJSON),
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_Deploy_WithCommand(t *testing.T) {
	step := &StepDef{
		Name:    "deploy",
		Type:    "deploy",
		Command: "kubectl apply -f /workspace/deploy.yaml",
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_Deploy_NoManifestNoCommand(t *testing.T) {
	step := &StepDef{
		Name: "deploy",
		Type: "deploy",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing manifest and command")
	}
}

func TestValidateStepDef_UnknownType(t *testing.T) {
	step := &StepDef{
		Name: "test",
		Type: "nonexistent-type",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStepDef_MissingName(t *testing.T) {
	step := &StepDef{
		Type:    "run-script",
		Command: "echo hello",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateStepDef_MissingType(t *testing.T) {
	step := &StepDef{
		Name:    "test",
		Command: "echo hello",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestValidateStepDef_CustomType_RequiresImage(t *testing.T) {
	step := &StepDef{
		Name:    "custom-step",
		Type:    "custom",
		Command: "echo hello",
	}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for custom type without image")
	}
	if !strings.Contains(err.Error(), "image is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStepDef_CustomType_WithImage(t *testing.T) {
	step := &StepDef{
		Name:    "custom-step",
		Type:    "custom",
		Image:   "myimage:latest",
		Command: "echo hello",
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GenerateCommand tests
// ---------------------------------------------------------------------------

func TestGenerateCommand_RunScript_UsesUserCommand(t *testing.T) {
	step := &StepDef{
		Name:    "test",
		Type:    "run-script",
		Command: "echo hello && ls -la",
	}
	cmd, args := GenerateCommand(step)
	if len(cmd) != 3 || cmd[0] != "/bin/sh" || cmd[1] != "-c" || cmd[2] != "echo hello && ls -la" {
		t.Errorf("unexpected command: %v", cmd)
	}
	if args != nil {
		t.Errorf("expected nil args, got: %v", args)
	}
}

func TestGenerateCommand_BuildImage_KanikoArgs(t *testing.T) {
	cfg := BuildImageConfig{
		Context:     "/workspace/src",
		Dockerfile:  "build/Dockerfile",
		Destination: "harbor.example.com/app:v1",
		Cache:       true,
		CacheRepo:   "harbor.example.com/cache",
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "build",
		Type:   "build-image",
		Config: string(cfgJSON),
	}
	cmd, args := GenerateCommand(step)
	if len(cmd) != 1 || cmd[0] != "/kaniko/executor" {
		t.Errorf("expected kaniko executor, got: %v", cmd)
	}

	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--context=/workspace/src") {
		t.Error("missing --context arg")
	}
	if !strings.Contains(argsStr, "--dockerfile=build/Dockerfile") {
		t.Error("missing --dockerfile arg")
	}
	if !strings.Contains(argsStr, "--destination=harbor.example.com/app:v1") {
		t.Error("missing --destination arg")
	}
	if !strings.Contains(argsStr, "--cache=true") {
		t.Error("missing --cache arg")
	}
	if !strings.Contains(argsStr, "--cache-repo=harbor.example.com/cache") {
		t.Error("missing --cache-repo arg")
	}
}

func TestGenerateCommand_BuildImage_Defaults(t *testing.T) {
	cfg := BuildImageConfig{
		Destination: "harbor.example.com/app:v1",
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "build",
		Type:   "build-image",
		Config: string(cfgJSON),
	}
	_, args := GenerateCommand(step)

	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--context=/workspace") {
		t.Errorf("expected default context /workspace, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--dockerfile=Dockerfile") {
		t.Errorf("expected default Dockerfile, got: %s", argsStr)
	}
}

func TestGenerateCommand_BuildImage_UserCommandOverride(t *testing.T) {
	cfg := BuildImageConfig{Destination: "harbor.example.com/app:v1"}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:    "build",
		Type:    "build-image",
		Command: "/kaniko/executor --custom-flag",
		Config:  string(cfgJSON),
	}
	cmd, _ := GenerateCommand(step)
	if cmd[2] != "/kaniko/executor --custom-flag" {
		t.Errorf("user command should override: %v", cmd)
	}
}

func TestGenerateCommand_Deploy_WithManifest(t *testing.T) {
	cfg := DeployConfig{
		Manifest:  "k8s/deployment.yaml",
		Namespace: "staging",
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "deploy",
		Type:   "deploy",
		Config: string(cfgJSON),
	}
	cmd, _ := GenerateCommand(step)
	if len(cmd) != 3 {
		t.Fatalf("expected 3-element command, got: %v", cmd)
	}
	if !strings.Contains(cmd[2], "kubectl apply -f k8s/deployment.yaml") {
		t.Errorf("expected kubectl apply command, got: %s", cmd[2])
	}
	if !strings.Contains(cmd[2], "-n staging") {
		t.Errorf("expected namespace flag, got: %s", cmd[2])
	}
}

func TestGenerateCommand_Deploy_DryRun(t *testing.T) {
	cfg := DeployConfig{
		Manifest: "deploy.yaml",
		DryRun:   true,
	}
	cfgJSON, _ := json.Marshal(cfg)

	step := &StepDef{
		Name:   "deploy",
		Type:   "deploy",
		Config: string(cfgJSON),
	}
	cmd, _ := GenerateCommand(step)
	if !strings.Contains(cmd[2], "--dry-run=server") {
		t.Errorf("expected dry-run flag, got: %s", cmd[2])
	}
}

// ---------------------------------------------------------------------------
// ResolveImage tests
// ---------------------------------------------------------------------------

func TestResolveImage_UserSpecified(t *testing.T) {
	step := &StepDef{
		Name:  "test",
		Type:  "run-script",
		Image: "custom-image:v2",
	}
	img := ResolveImage(step)
	if img != "custom-image:v2" {
		t.Errorf("expected user image, got: %s", img)
	}
}

func TestResolveImage_Default(t *testing.T) {
	step := &StepDef{
		Name: "test",
		Type: "run-script",
	}
	img := ResolveImage(step)
	if img != "alpine:3.20" {
		t.Errorf("expected default alpine image, got: %s", img)
	}
}

func TestResolveImage_BuildImage_Default(t *testing.T) {
	step := &StepDef{
		Name: "build",
		Type: "build-image",
	}
	img := ResolveImage(step)
	if !strings.Contains(img, "kaniko") {
		t.Errorf("expected kaniko default image, got: %s", img)
	}
}

func TestResolveImage_Deploy_Default(t *testing.T) {
	step := &StepDef{
		Name: "deploy",
		Type: "deploy",
	}
	img := ResolveImage(step)
	if !strings.Contains(img, "kubectl") {
		t.Errorf("expected kubectl default image, got: %s", img)
	}
}

// ---------------------------------------------------------------------------
// ListStepTypes / GetStepTypeInfo tests
// ---------------------------------------------------------------------------

func TestListStepTypes_NotEmpty(t *testing.T) {
	types := ListStepTypes()
	if len(types) == 0 {
		t.Error("expected non-empty step types list")
	}

	// Verify core types exist
	coreTypes := map[string]bool{"build-image": false, "deploy": false, "run-script": false}
	for _, st := range types {
		if _, ok := coreTypes[st.Name]; ok {
			coreTypes[st.Name] = true
		}
	}
	for name, found := range coreTypes {
		if !found {
			t.Errorf("core type %q not found in registry", name)
		}
	}
}

func TestGetStepTypeInfo_Exists(t *testing.T) {
	info, ok := GetStepTypeInfo("build-image")
	if !ok {
		t.Fatal("expected build-image to exist")
	}
	if info.DefaultImage == "" {
		t.Error("expected non-empty default image")
	}
}

func TestGetStepTypeInfo_NotExists(t *testing.T) {
	_, ok := GetStepTypeInfo("nonexistent")
	if ok {
		t.Error("expected nonexistent type to not be found")
	}
}

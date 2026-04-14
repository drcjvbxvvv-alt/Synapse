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

// ---------------------------------------------------------------------------
// trivy-scan tests
// ---------------------------------------------------------------------------

func TestValidateStepDef_TrivyScan_Valid(t *testing.T) {
	cfg := TrivyScanConfig{Image: "harbor.example.com/app:v1", Severity: "HIGH,CRITICAL"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "scan", Type: "trivy-scan", Config: string(cfgJSON)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_TrivyScan_MissingImage(t *testing.T) {
	cfg := TrivyScanConfig{Severity: "HIGH"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "scan", Type: "trivy-scan", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil {
		t.Error("expected error for missing image")
	}
	if !strings.Contains(err.Error(), "config.image is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStepDef_TrivyScan_NoConfig(t *testing.T) {
	step := &StepDef{Name: "scan", Type: "trivy-scan"}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for missing config")
	}
}

func TestGenerateCommand_TrivyScan(t *testing.T) {
	cfg := TrivyScanConfig{
		Image:    "harbor.example.com/app:v1",
		Severity: "HIGH,CRITICAL",
		ExitCode: 1,
		Format:   "json",
	}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "scan", Type: "trivy-scan", Config: string(cfgJSON)}
	cmd, args := GenerateCommand(step)

	if len(cmd) != 1 || cmd[0] != "trivy" {
		t.Errorf("expected trivy command, got: %v", cmd)
	}
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "image") {
		t.Error("missing 'image' subcommand")
	}
	if !strings.Contains(argsStr, "--severity=HIGH,CRITICAL") {
		t.Error("missing --severity arg")
	}
	if !strings.Contains(argsStr, "--exit-code=1") {
		t.Error("missing --exit-code arg")
	}
	if !strings.Contains(argsStr, "--format=json") {
		t.Error("missing --format arg")
	}
	if !strings.Contains(argsStr, "harbor.example.com/app:v1") {
		t.Error("missing target image in args")
	}
}

func TestGenerateCommand_TrivyScan_Defaults(t *testing.T) {
	cfg := TrivyScanConfig{Image: "myapp:latest"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "scan", Type: "trivy-scan", Config: string(cfgJSON)}
	_, args := GenerateCommand(step)

	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--exit-code=1") {
		t.Error("expected default exit-code=1")
	}
}

// ---------------------------------------------------------------------------
// push-image tests
// ---------------------------------------------------------------------------

func TestValidateStepDef_PushImage_Valid(t *testing.T) {
	cfg := PushImageConfig{Source: "app:build-123", Destination: "harbor.example.com/app:v1"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "push", Type: "push-image", Config: string(cfgJSON)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_PushImage_MissingSource(t *testing.T) {
	cfg := PushImageConfig{Destination: "harbor.example.com/app:v1"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "push", Type: "push-image", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil || !strings.Contains(err.Error(), "source is required") {
		t.Errorf("expected source required error, got: %v", err)
	}
}

func TestValidateStepDef_PushImage_MissingDestination(t *testing.T) {
	cfg := PushImageConfig{Source: "app:build-123"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "push", Type: "push-image", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil || !strings.Contains(err.Error(), "destination is required") {
		t.Errorf("expected destination required error, got: %v", err)
	}
}

func TestGenerateCommand_PushImage(t *testing.T) {
	cfg := PushImageConfig{Source: "app:build-123", Destination: "harbor.example.com/app:v1"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "push", Type: "push-image", Config: string(cfgJSON)}
	cmd, args := GenerateCommand(step)

	if len(cmd) != 1 || cmd[0] != "crane" {
		t.Errorf("expected crane command, got: %v", cmd)
	}
	if len(args) != 3 || args[0] != "copy" || args[1] != "app:build-123" || args[2] != "harbor.example.com/app:v1" {
		t.Errorf("unexpected args: %v", args)
	}
}

// ---------------------------------------------------------------------------
// deploy-helm tests
// ---------------------------------------------------------------------------

func TestValidateStepDef_DeployHelm_Valid(t *testing.T) {
	cfg := HelmDeployConfig{Release: "myapp", Chart: "bitnami/nginx", Namespace: "staging"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "helm", Type: "deploy-helm", Config: string(cfgJSON)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_DeployHelm_MissingRelease(t *testing.T) {
	cfg := HelmDeployConfig{Chart: "bitnami/nginx"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "helm", Type: "deploy-helm", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil || !strings.Contains(err.Error(), "release is required") {
		t.Errorf("expected release required error, got: %v", err)
	}
}

func TestValidateStepDef_DeployHelm_MissingChart(t *testing.T) {
	cfg := HelmDeployConfig{Release: "myapp"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "helm", Type: "deploy-helm", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil || !strings.Contains(err.Error(), "chart is required") {
		t.Errorf("expected chart required error, got: %v", err)
	}
}

func TestValidateStepDef_DeployHelm_WithCommand(t *testing.T) {
	step := &StepDef{Name: "helm", Type: "deploy-helm", Command: "helm upgrade --install myapp ./chart"}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid with user command, got: %v", err)
	}
}

func TestGenerateCommand_DeployHelm(t *testing.T) {
	cfg := HelmDeployConfig{
		Release:   "myapp",
		Chart:     "bitnami/nginx",
		Namespace: "staging",
		Values:    "values-staging.yaml",
		Version:   "18.1.0",
		Wait:      true,
		Timeout:   "5m",
		SetValues: map[string]string{"image.tag": "v1.2.3"},
	}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "helm", Type: "deploy-helm", Config: string(cfgJSON)}
	cmd, _ := GenerateCommand(step)

	if len(cmd) != 3 || cmd[0] != "/bin/sh" {
		t.Fatalf("expected shell command, got: %v", cmd)
	}
	helmCmd := cmd[2]
	if !strings.Contains(helmCmd, "helm upgrade --install myapp bitnami/nginx") {
		t.Errorf("missing base helm command, got: %s", helmCmd)
	}
	if !strings.Contains(helmCmd, "-n staging") {
		t.Errorf("missing namespace, got: %s", helmCmd)
	}
	if !strings.Contains(helmCmd, "-f values-staging.yaml") {
		t.Errorf("missing values file, got: %s", helmCmd)
	}
	if !strings.Contains(helmCmd, "--version 18.1.0") {
		t.Errorf("missing version, got: %s", helmCmd)
	}
	if !strings.Contains(helmCmd, "--wait") {
		t.Errorf("missing --wait, got: %s", helmCmd)
	}
	if !strings.Contains(helmCmd, "--timeout 5m") {
		t.Errorf("missing --timeout, got: %s", helmCmd)
	}
	if !strings.Contains(helmCmd, "--set image.tag=v1.2.3") {
		t.Errorf("missing --set, got: %s", helmCmd)
	}
}

func TestGenerateCommand_DeployHelm_DryRun(t *testing.T) {
	cfg := HelmDeployConfig{Release: "myapp", Chart: "./chart", DryRun: true}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "helm", Type: "deploy-helm", Config: string(cfgJSON)}
	cmd, _ := GenerateCommand(step)
	if !strings.Contains(cmd[2], "--dry-run") {
		t.Errorf("expected --dry-run, got: %s", cmd[2])
	}
}

// ---------------------------------------------------------------------------
// deploy-argocd-sync tests
// ---------------------------------------------------------------------------

func TestValidateStepDef_ArgoCDSync_Valid(t *testing.T) {
	cfg := ArgoCDSyncConfig{AppName: "my-app", Server: "argocd.example.com"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "sync", Type: "deploy-argocd-sync", Config: string(cfgJSON)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_ArgoCDSync_MissingAppName(t *testing.T) {
	cfg := ArgoCDSyncConfig{Server: "argocd.example.com"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "sync", Type: "deploy-argocd-sync", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil || !strings.Contains(err.Error(), "app_name is required") {
		t.Errorf("expected app_name required error, got: %v", err)
	}
}

func TestValidateStepDef_ArgoCDSync_WithCommand(t *testing.T) {
	step := &StepDef{Name: "sync", Type: "deploy-argocd-sync", Command: "argocd app sync my-app --prune"}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid with user command, got: %v", err)
	}
}

func TestGenerateCommand_ArgoCDSync(t *testing.T) {
	cfg := ArgoCDSyncConfig{
		AppName:  "my-app",
		Server:   "argocd.internal",
		Revision: "main",
		Prune:    true,
		Insecure: true,
	}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "sync", Type: "deploy-argocd-sync", Config: string(cfgJSON)}
	cmd, args := GenerateCommand(step)

	if len(cmd) != 1 || cmd[0] != "argocd" {
		t.Errorf("expected argocd command, got: %v", cmd)
	}
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "app sync my-app") {
		t.Errorf("missing app sync subcommand, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--server=argocd.internal") {
		t.Errorf("missing --server, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--revision=main") {
		t.Errorf("missing --revision, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--prune") {
		t.Errorf("missing --prune, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--plaintext") {
		t.Errorf("missing --plaintext for insecure, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--grpc-web") {
		t.Errorf("missing --grpc-web, got: %s", argsStr)
	}
}

func TestGenerateCommand_ArgoCDSync_DryRun(t *testing.T) {
	cfg := ArgoCDSyncConfig{AppName: "my-app", DryRun: true}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "sync", Type: "deploy-argocd-sync", Config: string(cfgJSON)}
	_, args := GenerateCommand(step)
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--dry-run") {
		t.Errorf("expected --dry-run, got: %s", argsStr)
	}
}

// ---------------------------------------------------------------------------
// notify tests
// ---------------------------------------------------------------------------

func TestValidateStepDef_Notify_Valid(t *testing.T) {
	cfg := NotifyConfig{URL: "https://hooks.slack.com/services/xxx"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "notify", Type: "notify", Config: string(cfgJSON)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_Notify_MissingURL(t *testing.T) {
	cfg := NotifyConfig{Method: "POST"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "notify", Type: "notify", Config: string(cfgJSON)}
	err := ValidateStepDef(step)
	if err == nil || !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected url required error, got: %v", err)
	}
}

func TestValidateStepDef_Notify_NoConfig(t *testing.T) {
	step := &StepDef{Name: "notify", Type: "notify"}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for missing config")
	}
}

func TestGenerateCommand_Notify(t *testing.T) {
	cfg := NotifyConfig{
		URL:     "https://hooks.slack.com/xxx",
		Headers: map[string]string{"Authorization": "Bearer token"},
		Body:    `{"text":"Deploy done"}`,
	}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "notify", Type: "notify", Config: string(cfgJSON)}
	cmd, _ := GenerateCommand(step)

	if len(cmd) != 3 || cmd[0] != "/bin/sh" {
		t.Fatalf("expected shell command, got: %v", cmd)
	}
	curlCmd := cmd[2]
	if !strings.Contains(curlCmd, "curl") {
		t.Errorf("expected curl in command, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, "-X POST") {
		t.Errorf("expected POST method, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, "hooks.slack.com") {
		t.Errorf("missing URL, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, "Deploy done") {
		t.Errorf("missing body, got: %s", curlCmd)
	}
}

func TestGenerateCommand_Notify_DefaultBody(t *testing.T) {
	cfg := NotifyConfig{URL: "https://example.com/webhook"}
	cfgJSON, _ := json.Marshal(cfg)
	step := &StepDef{Name: "notify", Type: "notify", Config: string(cfgJSON)}
	cmd, _ := GenerateCommand(step)
	if !strings.Contains(cmd[2], "Pipeline step completed") {
		t.Errorf("expected default body, got: %s", cmd[2])
	}
}

// ---------------------------------------------------------------------------
// ResolveImage for new types
// ---------------------------------------------------------------------------

func TestResolveImage_TrivyScan_Default(t *testing.T) {
	step := &StepDef{Name: "scan", Type: "trivy-scan"}
	img := ResolveImage(step)
	if !strings.Contains(img, "trivy") {
		t.Errorf("expected trivy default image, got: %s", img)
	}
}

func TestResolveImage_PushImage_Default(t *testing.T) {
	step := &StepDef{Name: "push", Type: "push-image"}
	img := ResolveImage(step)
	if !strings.Contains(img, "crane") {
		t.Errorf("expected crane default image, got: %s", img)
	}
}

func TestResolveImage_DeployHelm_Default(t *testing.T) {
	step := &StepDef{Name: "helm", Type: "deploy-helm"}
	img := ResolveImage(step)
	if !strings.Contains(img, "helm") {
		t.Errorf("expected helm default image, got: %s", img)
	}
}

func TestResolveImage_ArgoCDSync_Default(t *testing.T) {
	step := &StepDef{Name: "sync", Type: "deploy-argocd-sync"}
	img := ResolveImage(step)
	if !strings.Contains(img, "argocd") {
		t.Errorf("expected argocd default image, got: %s", img)
	}
}

func TestResolveImage_Notify_Default(t *testing.T) {
	step := &StepDef{Name: "notify", Type: "notify"}
	img := ResolveImage(step)
	if !strings.Contains(img, "curl") {
		t.Errorf("expected curl default image, got: %s", img)
	}
}

// ---------------------------------------------------------------------------
// Rollout step types (P2-7)
// ---------------------------------------------------------------------------

func TestValidateDeployRolloutStep_Valid(t *testing.T) {
	cfg, _ := json.Marshal(DeployRolloutConfig{
		RolloutName: "backend", Namespace: "prod", Image: "app:v2",
	})
	step := &StepDef{Name: "canary", Type: "deploy-rollout", Config: string(cfg)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDeployRolloutStep_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  DeployRolloutConfig
	}{
		{"no rollout_name", DeployRolloutConfig{Namespace: "ns", Image: "img"}},
		{"no namespace", DeployRolloutConfig{RolloutName: "r", Image: "img"}},
		{"no image", DeployRolloutConfig{RolloutName: "r", Namespace: "ns"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := json.Marshal(tt.cfg)
			step := &StepDef{Name: "s", Type: "deploy-rollout", Config: string(cfg)}
			if err := ValidateStepDef(step); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestValidateDeployRolloutStep_NoConfig(t *testing.T) {
	step := &StepDef{Name: "s", Type: "deploy-rollout"}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for missing config")
	}
}

func TestValidateRolloutPromoteStep_Valid(t *testing.T) {
	cfg, _ := json.Marshal(RolloutPromoteConfig{RolloutName: "backend", Namespace: "prod"})
	step := &StepDef{Name: "promote", Type: "rollout-promote", Config: string(cfg)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRolloutPromoteStep_MissingFields(t *testing.T) {
	cfg, _ := json.Marshal(RolloutPromoteConfig{Namespace: "prod"})
	step := &StepDef{Name: "s", Type: "rollout-promote", Config: string(cfg)}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for missing rollout_name")
	}
}

func TestValidateRolloutAbortStep_Valid(t *testing.T) {
	cfg, _ := json.Marshal(RolloutAbortConfig{RolloutName: "backend", Namespace: "prod"})
	step := &StepDef{Name: "abort", Type: "rollout-abort", Config: string(cfg)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRolloutAbortStep_MissingFields(t *testing.T) {
	cfg, _ := json.Marshal(RolloutAbortConfig{RolloutName: "backend"})
	step := &StepDef{Name: "s", Type: "rollout-abort", Config: string(cfg)}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for missing namespace")
	}
}

func TestValidateRolloutStatusStep_Valid(t *testing.T) {
	cfg, _ := json.Marshal(RolloutStatusConfig{
		RolloutName: "backend", Namespace: "prod",
		ExpectedStatus: "healthy", Timeout: "30m", OnTimeout: "abort",
	})
	step := &StepDef{Name: "wait", Type: "rollout-status", Config: string(cfg)}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRolloutStatusStep_InvalidExpectedStatus(t *testing.T) {
	cfg, _ := json.Marshal(RolloutStatusConfig{
		RolloutName: "backend", Namespace: "prod", ExpectedStatus: "invalid",
	})
	step := &StepDef{Name: "s", Type: "rollout-status", Config: string(cfg)}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for invalid expected_status")
	}
}

func TestValidateRolloutStatusStep_InvalidOnTimeout(t *testing.T) {
	cfg, _ := json.Marshal(RolloutStatusConfig{
		RolloutName: "backend", Namespace: "prod", OnTimeout: "retry",
	})
	step := &StepDef{Name: "s", Type: "rollout-status", Config: string(cfg)}
	if err := ValidateStepDef(step); err == nil {
		t.Error("expected error for invalid on_timeout")
	}
}

// --- Command generation ---

func TestGenerateDeployRolloutCommand(t *testing.T) {
	cfg, _ := json.Marshal(DeployRolloutConfig{
		RolloutName: "backend", Namespace: "prod", Image: "app:v2",
	})
	step := &StepDef{Name: "s", Type: "deploy-rollout", Config: string(cfg)}
	cmd, args := GenerateCommand(step)
	if cmd[0] != "kubectl-argo-rollouts" {
		t.Errorf("expected kubectl-argo-rollouts, got %v", cmd)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "set image") {
		t.Error("expected 'set image' in args")
	}
	if !strings.Contains(joined, "backend") {
		t.Error("expected rollout name in args")
	}
	if !strings.Contains(joined, "app:v2") {
		t.Error("expected image in args")
	}
}

func TestGenerateDeployRolloutCommand_WaitForReady(t *testing.T) {
	cfg, _ := json.Marshal(DeployRolloutConfig{
		RolloutName: "backend", Namespace: "prod", Image: "app:v2",
		WaitForReady: true, Timeout: "10m",
	})
	step := &StepDef{Name: "s", Type: "deploy-rollout", Config: string(cfg)}
	cmd, _ := GenerateCommand(step)
	// Should use /bin/sh -c with chained commands
	if cmd[0] != "/bin/sh" {
		t.Errorf("expected /bin/sh for wait_for_ready, got %v", cmd)
	}
	if !strings.Contains(cmd[2], "set image") || !strings.Contains(cmd[2], "status") {
		t.Error("expected chained set image + status command")
	}
	if !strings.Contains(cmd[2], "10m") {
		t.Error("expected custom timeout in command")
	}
}

func TestGenerateRolloutPromoteCommand(t *testing.T) {
	cfg, _ := json.Marshal(RolloutPromoteConfig{
		RolloutName: "backend", Namespace: "prod",
	})
	step := &StepDef{Name: "s", Type: "rollout-promote", Config: string(cfg)}
	cmd, args := GenerateCommand(step)
	if cmd[0] != "kubectl-argo-rollouts" {
		t.Errorf("expected kubectl-argo-rollouts, got %v", cmd)
	}
	if args[0] != "promote" {
		t.Errorf("expected promote subcommand, got %s", args[0])
	}
}

func TestGenerateRolloutPromoteCommand_Full(t *testing.T) {
	cfg, _ := json.Marshal(RolloutPromoteConfig{
		RolloutName: "backend", Namespace: "prod", Full: true,
	})
	step := &StepDef{Name: "s", Type: "rollout-promote", Config: string(cfg)}
	_, args := GenerateCommand(step)
	found := false
	for _, a := range args {
		if a == "--full" {
			found = true
		}
	}
	if !found {
		t.Error("expected --full flag")
	}
}

func TestGenerateRolloutAbortCommand(t *testing.T) {
	cfg, _ := json.Marshal(RolloutAbortConfig{
		RolloutName: "backend", Namespace: "prod",
	})
	step := &StepDef{Name: "s", Type: "rollout-abort", Config: string(cfg)}
	cmd, args := GenerateCommand(step)
	if cmd[0] != "kubectl-argo-rollouts" {
		t.Errorf("expected kubectl-argo-rollouts, got %v", cmd)
	}
	if args[0] != "abort" {
		t.Errorf("expected abort subcommand, got %s", args[0])
	}
}

func TestGenerateRolloutStatusCommand(t *testing.T) {
	cfg, _ := json.Marshal(RolloutStatusConfig{
		RolloutName: "backend", Namespace: "prod", Timeout: "15m",
	})
	step := &StepDef{Name: "s", Type: "rollout-status", Config: string(cfg)}
	cmd, args := GenerateCommand(step)
	if cmd[0] != "kubectl-argo-rollouts" {
		t.Errorf("expected kubectl-argo-rollouts, got %v", cmd)
	}
	if args[0] != "status" {
		t.Errorf("expected status subcommand, got %s", args[0])
	}
	if !strings.Contains(strings.Join(args, " "), "15m") {
		t.Error("expected custom timeout")
	}
}

func TestGenerateRolloutStatusCommand_OnTimeoutAbort(t *testing.T) {
	cfg, _ := json.Marshal(RolloutStatusConfig{
		RolloutName: "backend", Namespace: "prod",
		Timeout: "20m", OnTimeout: "abort",
	})
	step := &StepDef{Name: "s", Type: "rollout-status", Config: string(cfg)}
	cmd, _ := GenerateCommand(step)
	// Should use /bin/sh -c with chained status || abort
	if cmd[0] != "/bin/sh" {
		t.Errorf("expected /bin/sh for on_timeout=abort, got %v", cmd)
	}
	if !strings.Contains(cmd[2], "status") || !strings.Contains(cmd[2], "abort") {
		t.Error("expected status || abort in command")
	}
}

func TestResolveImage_DeployRollout_Default(t *testing.T) {
	step := &StepDef{Name: "s", Type: "deploy-rollout"}
	img := ResolveImage(step)
	if !strings.Contains(img, "argo-rollouts") {
		t.Errorf("expected argo-rollouts image, got: %s", img)
	}
}

func TestResolveImage_RolloutPromote_Default(t *testing.T) {
	step := &StepDef{Name: "s", Type: "rollout-promote"}
	img := ResolveImage(step)
	if !strings.Contains(img, "argo-rollouts") {
		t.Errorf("expected argo-rollouts image, got: %s", img)
	}
}

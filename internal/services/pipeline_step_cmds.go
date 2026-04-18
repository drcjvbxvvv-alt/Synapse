package services

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Command generation — produce container command+args for each Step type.
//
// All generate* functions are called by GenerateCommand (the dispatch entry
// point). Helper functions defaultString and parseJSON live in
// pipeline_step_types.go (same package).
// ---------------------------------------------------------------------------

// GenerateCommand 為特定 Step 類型產生容器執行指令。
// 若使用者已提供 command，則直接使用。
func GenerateCommand(step *StepDef) (command []string, args []string) {
	// 使用者自訂 command 優先
	if step.Command != "" {
		return []string{"/bin/sh", "-c", step.Command}, nil
	}

	switch step.Type {
	case "build-image":
		return generateBuildImageCommand(step)
	case "deploy":
		return generateDeployCommand(step)
	case "build-jar":
		return generateBuildJarCommand(step)
	case "trivy-scan":
		return generateTrivyScanCommand(step)
	case "push-image":
		return generatePushImageCommand(step)
	case "deploy-helm":
		return generateHelmDeployCommand(step)
	case "deploy-argocd-sync":
		return generateArgoCDSyncCommand(step)
	case "notify":
		return generateNotifyCommand(step)
	case "deploy-rollout":
		return generateDeployRolloutCommand(step)
	case "rollout-promote":
		return generateRolloutPromoteCommand(step)
	case "rollout-abort":
		return generateRolloutAbortCommand(step)
	case "rollout-status":
		return generateRolloutStatusCommand(step)
	case "smoke-test":
		return generateSmokeTestCommand(step)
	case "run-script", "shell":
		// run-script 必須有 command（已在 validation 檢查）
		return []string{"/bin/sh", "-c", "echo 'no command provided'"}, nil
	default:
		return nil, nil
	}
}

func generateBuildImageCommand(step *StepDef) ([]string, []string) {
	var cfg BuildImageConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"/kaniko/executor"}, []string{"--help"}
	}

	args := []string{
		"--context=" + defaultString(cfg.Context, "/workspace"),
		"--dockerfile=" + defaultString(cfg.Dockerfile, "Dockerfile"),
		"--destination=" + cfg.Destination,
		"--snapshot-mode=redo",
		"--push-retry=3",
	}

	if cfg.Insecure {
		// 從 destination 擷取 registry host 給 --insecure-registry
		registryHost := strings.SplitN(cfg.Destination, "/", 2)[0]
		args = append(args, "--insecure-registry="+registryHost)
	}

	if cfg.Cache {
		args = append(args, "--cache=true")
		if cfg.CacheRepo != "" {
			args = append(args, "--cache-repo="+cfg.CacheRepo)
		}
	}

	for k, v := range cfg.BuildArgs {
		args = append(args, fmt.Sprintf("--build-arg=%s=%s", k, v))
	}

	// Kaniko 使用自己的 entrypoint，不需要 /bin/sh -c
	return []string{"/kaniko/executor"}, args
}

func generateDeployCommand(step *StepDef) ([]string, []string) {
	var cfg DeployConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"/bin/sh", "-c", "kubectl apply -f /workspace"}, nil
	}

	manifests := cfg.GetManifests()
	if len(manifests) == 0 {
		manifests = []string{"/workspace/deployment.yaml"}
	}

	// Build: kubectl apply -f f1 -f f2 ...
	cmd := "kubectl apply"
	for _, m := range manifests {
		cmd += " -f " + m
	}

	if cfg.Namespace != "" {
		cmd += " -n " + cfg.Namespace
	}
	if cfg.DryRun {
		cmd += " --dry-run=server"
	}

	return []string{"/bin/sh", "-c", cmd}, nil
}

func generateTrivyScanCommand(step *StepDef) ([]string, []string) {
	var cfg TrivyScanConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"trivy"}, []string{"--help"}
	}

	args := []string{"image"}

	if cfg.Severity != "" {
		args = append(args, "--severity="+cfg.Severity)
	}
	if cfg.ExitCode > 0 {
		args = append(args, fmt.Sprintf("--exit-code=%d", cfg.ExitCode))
	} else {
		args = append(args, "--exit-code=1") // 預設：發現漏洞時失敗
	}
	if cfg.IgnoreFile != "" {
		args = append(args, "--ignorefile="+cfg.IgnoreFile)
	}
	if cfg.Format != "" {
		args = append(args, "--format="+cfg.Format)
	}

	args = append(args, cfg.Image)

	return []string{"trivy"}, args
}

func generatePushImageCommand(step *StepDef) ([]string, []string) {
	var cfg PushImageConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"crane"}, []string{"--help"}
	}

	// crane copy = pull + push (retag)
	return []string{"crane"}, []string{"copy", cfg.Source, cfg.Destination}
}

func generateHelmDeployCommand(step *StepDef) ([]string, []string) {
	var cfg HelmDeployConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"/bin/sh", "-c", "helm version"}, nil
	}

	cmd := "helm upgrade --install " + cfg.Release + " " + cfg.Chart

	if cfg.Namespace != "" {
		cmd += " -n " + cfg.Namespace + " --create-namespace"
	}
	if cfg.Values != "" {
		cmd += " -f " + cfg.Values
	}
	for k, v := range cfg.SetValues {
		cmd += fmt.Sprintf(" --set %s=%s", k, v)
	}
	if cfg.Version != "" {
		cmd += " --version " + cfg.Version
	}
	if cfg.Wait {
		cmd += " --wait"
	}
	if cfg.Timeout != "" {
		cmd += " --timeout " + cfg.Timeout
	}
	if cfg.DryRun {
		cmd += " --dry-run"
	}

	return []string{"/bin/sh", "-c", cmd}, nil
}

func generateArgoCDSyncCommand(step *StepDef) ([]string, []string) {
	var cfg ArgoCDSyncConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"argocd"}, []string{"version"}
	}

	args := []string{"app", "sync", cfg.AppName}

	if cfg.Server != "" {
		args = append(args, "--server="+cfg.Server)
	}
	if cfg.Revision != "" {
		args = append(args, "--revision="+cfg.Revision)
	}
	if cfg.Prune {
		args = append(args, "--prune")
	}
	if cfg.DryRun {
		args = append(args, "--dry-run")
	}
	if cfg.Insecure {
		args = append(args, "--plaintext")
	}

	// 使用 argocd CLI + grpc-web（非互動式）
	args = append(args, "--grpc-web")

	return []string{"argocd"}, args
}

func generateNotifyCommand(step *StepDef) ([]string, []string) {
	var cfg NotifyConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"curl"}, []string{"--help"}
	}

	method := defaultString(cfg.Method, "POST")
	body := defaultString(cfg.Body, `{"text":"Pipeline step completed"}`)

	cmd := fmt.Sprintf("curl -sfS -X %s", method)
	for k, v := range cfg.Headers {
		cmd += fmt.Sprintf(" -H '%s: %s'", k, v)
	}
	cmd += fmt.Sprintf(" -H 'Content-Type: application/json' -d '%s' '%s'", body, cfg.URL)

	return []string{"/bin/sh", "-c", cmd}, nil
}

func generateBuildJarCommand(step *StepDef) ([]string, []string) {
	var cfg BuildJarConfig
	if len(step.Config) > 0 {
		if err := parseJSON(step.Config, &cfg); err != nil {
			// fallback: maven clean package
			return []string{"/bin/sh", "-c", "mvn clean package -DskipTests -B"}, nil
		}
	}

	tool := defaultString(cfg.BuildTool, "maven")

	if tool == "gradle" {
		return generateGradleCommand(&cfg), nil
	}
	return generateMavenCommand(&cfg), nil
}

func generateMavenCommand(cfg *BuildJarConfig) []string {
	cmd := "mvn"
	goals := defaultString(cfg.Goals, "clean package -DskipTests")
	cmd += " " + goals
	cmd += " -B" // batch mode (non-interactive)

	if cfg.PomFile != "" {
		cmd += " -f " + cfg.PomFile
	}
	for _, profile := range cfg.Profiles {
		cmd += " -P" + profile
	}
	for k, v := range cfg.Properties {
		cmd += fmt.Sprintf(" -D%s=%s", k, v)
	}
	if cfg.CacheDir != "" {
		cmd += " -Dmaven.repo.local=" + cfg.CacheDir
	}

	return []string{"/bin/sh", "-c", cmd}
}

func generateGradleCommand(cfg *BuildJarConfig) []string {
	// 使用 gradle wrapper 如果存在，否則 fallback gradle
	cmd := "if [ -f ./gradlew ]; then GRADLE=./gradlew; else GRADLE=gradle; fi && $GRADLE"
	tasks := defaultString(cfg.Tasks, "clean build -x test")
	cmd += " " + tasks

	if cfg.BuildFile != "" {
		cmd += " -b " + cfg.BuildFile
	}
	for k, v := range cfg.Properties {
		cmd += fmt.Sprintf(" -D%s=%s", k, v)
	}
	if cfg.CacheDir != "" {
		cmd += " --project-cache-dir " + cfg.CacheDir
	}
	cmd += " --no-daemon" // CI 環境不使用 daemon

	return []string{"/bin/sh", "-c", cmd}
}

func generateDeployRolloutCommand(step *StepDef) ([]string, []string) {
	var cfg DeployRolloutConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"kubectl-argo-rollouts"}, []string{"--help"}
	}

	// kubectl-argo-rollouts set image ROLLOUT_NAME CONTAINER=IMAGE -n NAMESPACE
	args := []string{
		"set", "image", cfg.RolloutName,
		"*=" + cfg.Image, // *=image updates all containers
		"-n", cfg.Namespace,
	}

	if cfg.WaitForReady {
		// Chain: set image + wait for status
		timeout := defaultString(cfg.Timeout, "30m")
		cmd := fmt.Sprintf(
			"kubectl-argo-rollouts set image %s '*=%s' -n %s && kubectl-argo-rollouts status %s -n %s --timeout %s",
			cfg.RolloutName, cfg.Image, cfg.Namespace,
			cfg.RolloutName, cfg.Namespace, timeout,
		)
		return []string{"/bin/sh", "-c", cmd}, nil
	}

	return []string{"kubectl-argo-rollouts"}, args
}

func generateRolloutPromoteCommand(step *StepDef) ([]string, []string) {
	var cfg RolloutPromoteConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"kubectl-argo-rollouts"}, []string{"--help"}
	}

	args := []string{"promote", cfg.RolloutName, "-n", cfg.Namespace}
	if cfg.Full {
		args = append(args, "--full")
	}

	return []string{"kubectl-argo-rollouts"}, args
}

func generateRolloutAbortCommand(step *StepDef) ([]string, []string) {
	var cfg RolloutAbortConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"kubectl-argo-rollouts"}, []string{"--help"}
	}

	return []string{"kubectl-argo-rollouts"}, []string{
		"abort", cfg.RolloutName, "-n", cfg.Namespace,
	}
}

func generateRolloutStatusCommand(step *StepDef) ([]string, []string) {
	var cfg RolloutStatusConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"kubectl-argo-rollouts"}, []string{"--help"}
	}

	timeout := defaultString(cfg.Timeout, "30m")
	onTimeout := defaultString(cfg.OnTimeout, "fail")

	// kubectl-argo-rollouts status ROLLOUT -n NS --timeout TIMEOUT
	// If on_timeout=abort, chain with abort on failure
	if onTimeout == "abort" {
		cmd := fmt.Sprintf(
			"kubectl-argo-rollouts status %s -n %s --timeout %s || kubectl-argo-rollouts abort %s -n %s",
			cfg.RolloutName, cfg.Namespace, timeout,
			cfg.RolloutName, cfg.Namespace,
		)
		return []string{"/bin/sh", "-c", cmd}, nil
	}

	return []string{"kubectl-argo-rollouts"}, []string{
		"status", cfg.RolloutName, "-n", cfg.Namespace,
		"--timeout", timeout,
	}
}

func generateSmokeTestCommand(step *StepDef) ([]string, []string) {
	var cfg SmokeTestConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"curl"}, []string{"--help"}
	}

	method := defaultString(cfg.Method, "GET")
	expectedStatus := cfg.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10
	}
	retries := cfg.Retries
	if retries <= 0 {
		retries = 3
	}
	retryInterval := cfg.RetryInterval
	if retryInterval <= 0 {
		retryInterval = 5
	}

	// 建構 curl 指令
	curlCmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' -X %s", method)
	curlCmd += fmt.Sprintf(" --max-time %d", timeout)

	if cfg.Insecure {
		curlCmd += " -k"
	}

	for k, v := range cfg.Headers {
		curlCmd += fmt.Sprintf(" -H '%s: %s'", k, v)
	}

	if cfg.Body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		curlCmd += fmt.Sprintf(" -d '%s'", cfg.Body)
	}

	curlCmd += fmt.Sprintf(" '%s'", cfg.URL)

	// 包裝為帶重試的 shell 腳本
	script := fmt.Sprintf(`#!/bin/sh
set -e

URL=%q
EXPECTED=%d
RETRIES=%d
INTERVAL=%d

echo "[synapse] smoke-test: %s $URL (expect HTTP $EXPECTED)"

for i in $(seq 1 $RETRIES); do
  STATUS=$(%s)
  echo "[synapse] attempt $i/$RETRIES: HTTP $STATUS"
  if [ "$STATUS" = "$EXPECTED" ]; then
    echo "[synapse] smoke-test PASSED"
    exit 0
  fi
  if [ "$i" -lt "$RETRIES" ]; then
    echo "[synapse] retrying in ${INTERVAL}s..."
    sleep $INTERVAL
  fi
done

echo "[synapse] smoke-test FAILED: expected HTTP $EXPECTED, last got HTTP $STATUS"
exit 1
`, cfg.URL, expectedStatus, retries, retryInterval, method, curlCmd)

	return []string{"/bin/sh", "-c", script}, nil
}

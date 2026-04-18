package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Secret 解析
// ---------------------------------------------------------------------------

// resolveSecrets 解析 Step 設定中的 ${{ secrets.* }} 引用，
// 從 PipelineSecretService 查詢實際值。
func (s *PipelineScheduler) resolveSecrets(ctx context.Context, pipelineID uint, configJSON string) (map[string]string, error) {
	if configJSON == "" || s.secretSvc == nil {
		return nil, nil
	}

	var cfg StepConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, nil // 無法解析時不報錯，由 JobBuilder 處理
	}

	resolved := make(map[string]string)
	for k, v := range cfg.Env {
		secretName, ok := parseSecretRef(v)
		if !ok {
			resolved[k] = v
			continue
		}
		secretVal, err := s.lookupSecret(ctx, pipelineID, secretName)
		if err != nil {
			return nil, fmt.Errorf("secret %q: %w", secretName, err)
		}
		resolved[k] = secretVal
	}
	return resolved, nil
}

// parseSecretRef 解析 ${{ secrets.NAME }} 格式，回傳 secret name 和是否匹配。
func parseSecretRef(value string) (string, bool) {
	v := strings.TrimSpace(value)
	if !strings.HasPrefix(v, "${{") || !strings.HasSuffix(v, "}}") {
		return "", false
	}
	inner := strings.TrimSpace(v[3 : len(v)-2])
	if !strings.HasPrefix(inner, "secrets.") {
		return "", false
	}
	name := strings.TrimSpace(inner[8:])
	if name == "" {
		return "", false
	}
	return name, true
}

// lookupSecret 依優先順序查詢 secret：pipeline → global。
func (s *PipelineScheduler) lookupSecret(ctx context.Context, pipelineID uint, name string) (string, error) {
	secrets, err := s.secretSvc.ListSecrets(ctx, "pipeline", &pipelineID)
	if err != nil {
		return "", fmt.Errorf("list pipeline secrets: %w", err)
	}
	for _, sec := range secrets {
		if sec.Name == name {
			full, err := s.secretSvc.GetSecret(ctx, sec.ID)
			if err != nil {
				return "", err
			}
			return full.ValueEnc, nil
		}
	}

	secrets, err = s.secretSvc.ListSecrets(ctx, "global", nil)
	if err != nil {
		return "", fmt.Errorf("list global secrets: %w", err)
	}
	for _, sec := range secrets {
		if sec.Name == name {
			full, err := s.secretSvc.GetSecret(ctx, sec.ID)
			if err != nil {
				return "", err
			}
			return full.ValueEnc, nil
		}
	}

	return "", fmt.Errorf("secret %q not found in pipeline, environment, or global scope", name)
}

// ---------------------------------------------------------------------------
// Registry 認證注入
// ---------------------------------------------------------------------------

// injectRegistryCredentials 為 push-image / build-image Step 自動注入 Registry 認證。
func (s *PipelineScheduler) injectRegistryCredentials(ctx context.Context, stepType, configJSON string, secrets map[string]string) {
	if s.registrySvc == nil || configJSON == "" {
		return
	}
	if stepType != "push-image" && stepType != "build-image" {
		return
	}

	var cfg PushImageConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil || cfg.Registry == "" {
		logger.Debug("registry credential injection skipped",
			"step_type", stepType,
			"registry_field", cfg.Registry,
			"config", configJSON,
		)
		return
	}

	registry, err := s.registrySvc.GetRegistryByName(ctx, cfg.Registry)
	if err != nil {
		logger.Warn("registry not found for push-image credential injection",
			"registry", cfg.Registry,
			"error", err,
		)
		return
	}

	if secrets == nil {
		return
	}
	if registry.Username != "" {
		if _, exists := secrets["DOCKER_USERNAME"]; !exists {
			secrets["DOCKER_USERNAME"] = registry.Username
		}
	}
	if registry.PasswordEnc != "" {
		if _, exists := secrets["DOCKER_PASSWORD"]; !exists {
			secrets["DOCKER_PASSWORD"] = registry.PasswordEnc
		}
	}
	if registry.URL != "" {
		if _, exists := secrets["REGISTRY_URL"]; !exists {
			secrets["REGISTRY_URL"] = registry.URL
		}
	}
	if registry.InsecureTLS {
		secrets["REGISTRY_INSECURE"] = "true"
	}

	logger.Debug("registry credentials injected for step",
		"step_type", stepType,
		"registry", cfg.Registry,
	)
}

// buildDockerConfigJSON 從已注入的 secrets 建構 Kaniko 所需的 docker config.json。
// Kaniko 不讀取 DOCKER_USERNAME/DOCKER_PASSWORD 環境變數，需要 /kaniko/.docker/config.json。
func (s *PipelineScheduler) buildDockerConfigJSON(secrets map[string]string) string {
	registryURL := secrets["REGISTRY_URL"]
	username := secrets["DOCKER_USERNAME"]
	password := secrets["DOCKER_PASSWORD"]
	if registryURL == "" || username == "" {
		return ""
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	dockerCfg := fmt.Sprintf(
		`{"auths":{%q:{"username":%q,"password":%q,"auth":%q}}}`,
		registryURL, username, password, auth,
	)
	return dockerCfg
}

// ---------------------------------------------------------------------------
// ImagePullSecret 解析
// ---------------------------------------------------------------------------

// resolveImagePullSecret 檢查 step image 是否來自已註冊的私有 registry，
// 如果是，則自動建立 imagePullSecret 供 K8s 拉取 image。
func (s *PipelineScheduler) resolveImagePullSecret(ctx context.Context, run *models.PipelineRun, sr *models.StepRun) string {
	if s.registrySvc == nil {
		return ""
	}

	image := sr.Image
	if image == "" {
		return ""
	}

	registries, err := s.registrySvc.ListRegistries(ctx)
	if err != nil {
		return ""
	}

	for _, reg := range registries {
		if !reg.Enabled || reg.Username == "" {
			continue
		}
		registryHost := extractHost(reg.URL)
		if registryHost != "" && strings.HasPrefix(image, registryHost+"/") {
			fullReg, err := s.registrySvc.GetRegistry(ctx, reg.ID)
			if err != nil || fullReg.PasswordEnc == "" {
				continue
			}

			k8sClient := s.k8sProvider.GetK8sClientByID(run.ClusterID)
			if k8sClient == nil {
				continue
			}

			secretName, err := s.jobBuilder.EnsureImagePullSecret(
				ctx, k8sClient, run, sr, run.Namespace,
				registryHost, fullReg.Username, fullReg.PasswordEnc,
			)
			if err != nil {
				logger.Warn("failed to create imagePullSecret",
					"registry", reg.Name,
					"step_run_id", sr.ID,
					"error", err,
				)
				continue
			}
			if secretName != "" {
				logger.Debug("imagePullSecret created for step",
					"secret_name", secretName,
					"registry", reg.Name,
					"step_run_id", sr.ID,
				)
				return secretName
			}
		}
	}

	return ""
}

// resolveGitInfo 從 Pipeline → Project → GitProvider 解析 git clone 所需的資訊。
// 回傳 (repoURL, branch, token)。任何步驟失敗回傳空字串（不阻塞 Pipeline 執行）。
func (s *PipelineScheduler) resolveGitInfo(ctx context.Context, pipelineID uint) (string, string, string) {
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).First(&pipeline, pipelineID).Error; err != nil || pipeline.ProjectID == nil {
		return "", "", ""
	}

	var project models.Project
	if err := s.db.WithContext(ctx).First(&project, *pipeline.ProjectID).Error; err != nil {
		return "", "", ""
	}

	var provider models.GitProvider
	if err := s.db.WithContext(ctx).First(&provider, project.GitProviderID).Error; err != nil {
		return "", "", ""
	}

	token := provider.AccessTokenEnc // GORM AfterFind hook decrypts
	logger.Debug("resolved git info for pipeline",
		"pipeline_id", pipelineID,
		"repo_url", project.RepoURL,
		"branch", project.DefaultBranch,
	)
	return project.RepoURL, project.DefaultBranch, token
}

// extractHost 從 URL 提取 host（去除 scheme 和 path）。
func extractHost(rawURL string) string {
	host := rawURL
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	return strings.TrimSpace(host)
}

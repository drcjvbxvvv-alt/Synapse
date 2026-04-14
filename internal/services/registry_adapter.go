package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// Registry Adapter — 統一 Registry API 介面（CICD_ARCHITECTURE §11, P2-3）
//
// 設計原則：
//   - RegistryAdapter 定義統一 API，每個 Registry 類型一個實作
//   - 支援 Docker Registry API v2 作為基礎協定
//   - Harbor / ECR / GCR 各有擴充欄位
//   - 純網路呼叫，不依賴 DB
// ---------------------------------------------------------------------------

// RegistryAdapter 統一 Registry 操作介面。
type RegistryAdapter interface {
	// Ping 測試 Registry 連線是否正常。
	Ping(ctx context.Context) error
	// ListRepositories 列出 Registry 中的 Repository。
	ListRepositories(ctx context.Context, project string) ([]RegistryRepository, error)
	// ListTags 列出指定 Repository 的 Tags。
	ListTags(ctx context.Context, repository string) ([]RegistryTag, error)
	// GetManifest 取得指定 Tag 的 Manifest 資訊。
	GetManifest(ctx context.Context, repository, reference string) (*RegistryManifest, error)
}

// RegistryRepository 代表 Registry 中的一個 Repository。
type RegistryRepository struct {
	Name      string `json:"name"`
	TagCount  int    `json:"tag_count,omitempty"`
	PullCount int64  `json:"pull_count,omitempty"`
}

// RegistryTag 代表 Repository 中的一個 Tag。
type RegistryTag struct {
	Name      string    `json:"name"`
	Digest    string    `json:"digest,omitempty"`
	Size      int64     `json:"size,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// RegistryManifest 代表映像的 Manifest 資訊。
type RegistryManifest struct {
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

// NewRegistryAdapter 根據 Registry 類型建立對應的 adapter。
func NewRegistryAdapter(registry *models.Registry) (RegistryAdapter, error) {
	client := buildRegistryHTTPClient(registry)
	base := registryBase{
		url:      strings.TrimRight(registry.URL, "/"),
		username: registry.Username,
		password: registry.PasswordEnc, // already decrypted by AfterFind hook
		client:   client,
	}

	switch registry.Type {
	case models.RegistryTypeHarbor:
		return &HarborAdapter{registryBase: base, defaultProject: registry.DefaultProject}, nil
	case models.RegistryTypeDockerHub:
		return &DockerHubAdapter{registryBase: base}, nil
	case models.RegistryTypeACR:
		return &DockerV2Adapter{registryBase: base}, nil
	case models.RegistryTypeECR:
		return &DockerV2Adapter{registryBase: base}, nil
	case models.RegistryTypeGCR:
		return &DockerV2Adapter{registryBase: base}, nil
	default:
		return nil, fmt.Errorf("unsupported registry type: %s", registry.Type)
	}
}

// ---------------------------------------------------------------------------
// Shared base for Docker Registry API v2
// ---------------------------------------------------------------------------

type registryBase struct {
	url      string
	username string
	password string
	client   *http.Client
}

func (b *registryBase) doRequest(ctx context.Context, method, path string) (*http.Response, error) {
	url := b.url + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if b.username != "" && b.password != "" {
		req.SetBasicAuth(b.username, b.password)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request %s: %w", path, err)
	}

	return resp, nil
}

func (b *registryBase) ping(ctx context.Context) error {
	resp, err := b.doRequest(ctx, http.MethodGet, "/v2/")
	if err != nil {
		return fmt.Errorf("ping registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		// 401 means the registry exists but needs auth — still a valid connection
		return nil
	}
	return fmt.Errorf("ping registry: unexpected status %d", resp.StatusCode)
}

// ---------------------------------------------------------------------------
// DockerV2Adapter — Generic Docker Registry API v2（ACR / ECR / GCR 共用）
// ---------------------------------------------------------------------------

// DockerV2Adapter 實作 Docker Registry API v2 通用操作。
type DockerV2Adapter struct {
	registryBase
}

func (a *DockerV2Adapter) Ping(ctx context.Context) error {
	return a.registryBase.ping(ctx)
}

func (a *DockerV2Adapter) ListRepositories(ctx context.Context, _ string) ([]RegistryRepository, error) {
	resp, err := a.doRequest(ctx, http.MethodGet, "/v2/_catalog")
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list repositories: status %d", resp.StatusCode)
	}

	var result struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}

	repos := make([]RegistryRepository, 0, len(result.Repositories))
	for _, name := range result.Repositories {
		repos = append(repos, RegistryRepository{Name: name})
	}
	return repos, nil
}

func (a *DockerV2Adapter) ListTags(ctx context.Context, repository string) ([]RegistryTag, error) {
	path := fmt.Sprintf("/v2/%s/tags/list", repository)
	resp, err := a.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("list tags for %s: %w", repository, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list tags: status %d", resp.StatusCode)
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}

	tags := make([]RegistryTag, 0, len(result.Tags))
	for _, name := range result.Tags {
		tags = append(tags, RegistryTag{Name: name})
	}
	return tags, nil
}

func (a *DockerV2Adapter) GetManifest(ctx context.Context, repository, reference string) (*RegistryManifest, error) {
	path := fmt.Sprintf("/v2/%s/manifests/%s", repository, reference)
	resp, err := a.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("get manifest for %s:%s: %w", repository, reference, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get manifest: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}

	return &RegistryManifest{
		Digest:    resp.Header.Get("Docker-Content-Digest"),
		MediaType: resp.Header.Get("Content-Type"),
		Size:      int64(len(body)),
	}, nil
}

// ---------------------------------------------------------------------------
// HarborAdapter — Harbor 專屬 API 擴充
// ---------------------------------------------------------------------------

// HarborAdapter 實作 Harbor Registry API（Docker v2 + Harbor 專屬）。
type HarborAdapter struct {
	registryBase
	defaultProject string
}

func (a *HarborAdapter) Ping(ctx context.Context) error {
	// Harbor 有專屬 health endpoint
	resp, err := a.doRequest(ctx, http.MethodGet, "/api/v2.0/health")
	if err != nil {
		// fallback to v2 ping
		return a.registryBase.ping(ctx)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	// fallback
	return a.registryBase.ping(ctx)
}

func (a *HarborAdapter) ListRepositories(ctx context.Context, project string) ([]RegistryRepository, error) {
	if project == "" {
		project = a.defaultProject
	}
	if project == "" {
		return nil, fmt.Errorf("harbor requires a project name")
	}

	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories?page_size=100", project)
	resp, err := a.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("list harbor repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list harbor repositories: status %d", resp.StatusCode)
	}

	var harborRepos []struct {
		Name         string `json:"name"`
		ArtifactCount int   `json:"artifact_count"`
		PullCount    int64  `json:"pull_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&harborRepos); err != nil {
		return nil, fmt.Errorf("decode harbor repositories: %w", err)
	}

	repos := make([]RegistryRepository, 0, len(harborRepos))
	for _, r := range harborRepos {
		repos = append(repos, RegistryRepository{
			Name:      r.Name,
			TagCount:  r.ArtifactCount,
			PullCount: r.PullCount,
		})
	}
	return repos, nil
}

func (a *HarborAdapter) ListTags(ctx context.Context, repository string) ([]RegistryTag, error) {
	// Harbor uses artifacts API for tags
	path := fmt.Sprintf("/api/v2.0/projects/%s/repositories/%s/artifacts?page_size=100&with_tag=true",
		a.defaultProject, repository)
	resp, err := a.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("list harbor tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list harbor tags: status %d", resp.StatusCode)
	}

	var artifacts []struct {
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
		Tags   []struct {
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"push_time"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&artifacts); err != nil {
		return nil, fmt.Errorf("decode harbor artifacts: %w", err)
	}

	var tags []RegistryTag
	for _, a := range artifacts {
		for _, t := range a.Tags {
			tags = append(tags, RegistryTag{
				Name:      t.Name,
				Digest:    a.Digest,
				Size:      a.Size,
				CreatedAt: t.CreatedAt,
			})
		}
	}
	return tags, nil
}

func (a *HarborAdapter) GetManifest(ctx context.Context, repository, reference string) (*RegistryManifest, error) {
	// Use Docker v2 API for manifest
	path := fmt.Sprintf("/v2/%s/%s/manifests/%s", a.defaultProject, repository, reference)
	resp, err := a.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("get harbor manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get harbor manifest: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}

	return &RegistryManifest{
		Digest:    resp.Header.Get("Docker-Content-Digest"),
		MediaType: resp.Header.Get("Content-Type"),
		Size:      int64(len(body)),
	}, nil
}

// ---------------------------------------------------------------------------
// DockerHubAdapter — Docker Hub 擴充（含 Docker Hub API）
// ---------------------------------------------------------------------------

// DockerHubAdapter 實作 Docker Hub Registry API。
type DockerHubAdapter struct {
	registryBase
}

func (a *DockerHubAdapter) Ping(ctx context.Context) error {
	return a.registryBase.ping(ctx)
}

func (a *DockerHubAdapter) ListRepositories(ctx context.Context, namespace string) ([]RegistryRepository, error) {
	if namespace == "" {
		namespace = a.username
	}
	if namespace == "" {
		return nil, fmt.Errorf("docker hub requires a namespace (username or organization)")
	}

	// Docker Hub v2 API for user repos
	path := fmt.Sprintf("/v2/repositories/%s/?page_size=100", namespace)
	resp, err := a.doRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("list docker hub repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list docker hub repositories: status %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name      string `json:"name"`
			PullCount int64  `json:"pull_count"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode docker hub repositories: %w", err)
	}

	repos := make([]RegistryRepository, 0, len(result.Results))
	for _, r := range result.Results {
		repos = append(repos, RegistryRepository{
			Name:      r.Name,
			PullCount: r.PullCount,
		})
	}
	return repos, nil
}

func (a *DockerHubAdapter) ListTags(ctx context.Context, repository string) ([]RegistryTag, error) {
	// Use standard v2 API
	adapter := &DockerV2Adapter{registryBase: a.registryBase}
	return adapter.ListTags(ctx, repository)
}

func (a *DockerHubAdapter) GetManifest(ctx context.Context, repository, reference string) (*RegistryManifest, error) {
	adapter := &DockerV2Adapter{registryBase: a.registryBase}
	return adapter.GetManifest(ctx, repository, reference)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildRegistryHTTPClient(registry *models.Registry) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: registry.InsecureTLS, //nolint:gosec // user-controlled per registry config
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

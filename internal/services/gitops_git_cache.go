package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ---------------------------------------------------------------------------
// GitOps Git Cache — PVC-backed Git clone 快取（CICD_ARCHITECTURE §12.3）
//
// 設計原則：
//   - 同一 Repo URL 的多個 App 共享同一個 PVC（hash-based 命名）
//   - PVC 使用 ReadWriteOnce（單 Node Diff Worker）
//   - Git clone 結果持久化，每次 Diff 只需 git pull
//   - PVC 大小預設 5Gi（可透過 config 調整）
//   - Namespace 使用 synapse-system（或 pipeline namespace）
// ---------------------------------------------------------------------------

const (
	// GitCachePVCPrefix is the prefix for git clone cache PVC names.
	GitCachePVCPrefix = "gitops-clone-"

	// GitCachePVCDefaultSize is the default storage size for clone cache PVC.
	GitCachePVCDefaultSize = "5Gi"

	// GitCacheCloneImage is the default image for git operations.
	GitCacheCloneImage = "bitnami/git:2.47"

	// GitCacheMountPath is the mount path inside the git clone container.
	GitCacheMountPath = "/workspace/repo"

	// GitCacheJobTTL is the TTL for completed clone/pull jobs.
	GitCacheJobTTL = 300 // 5 minutes
)

// GitCacheService 管理 GitOps 的 Git clone 快取 PVC。
type GitCacheService struct {
	gitopsSvc *GitOpsService
}

// NewGitCacheService 建立 GitCacheService。
func NewGitCacheService(gitopsSvc *GitOpsService) *GitCacheService {
	return &GitCacheService{gitopsSvc: gitopsSvc}
}

// CloneCacheKey 從 repo URL 產生一致的快取 key（PVC 名稱後綴）。
// 同一 repo 的多個 App（不同 path/branch）共享同一個 PVC。
func CloneCacheKey(repoURL string) string {
	normalized := strings.TrimSuffix(strings.TrimSpace(repoURL), ".git")
	normalized = strings.ToLower(normalized)
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h[:8]) // 16 chars hex
}

// PVCName 產生 clone cache PVC 的名稱。
func PVCName(repoURL string) string {
	name := GitCachePVCPrefix + CloneCacheKey(repoURL)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// EnsureClonePVC 確保指定 repo 的 clone cache PVC 存在。
// 如果 PVC 已存在則為 no-op，否則建立新的 PVC。
func (s *GitCacheService) EnsureClonePVC(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	repoURL string,
) (string, error) {
	pvcName := PVCName(repoURL)

	// 檢查是否已存在
	_, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		// 已存在
		return pvcName, nil
	}
	if !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("check clone PVC %s: %w", pvcName, err)
	}

	// 建立 PVC
	pvc := BuildCloneCachePVC(pvcName, namespace, repoURL)
	created, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return pvcName, nil // 競爭條件：其他 worker 先建立了
		}
		return "", fmt.Errorf("create clone PVC %s: %w", pvcName, err)
	}

	logger.Info("gitops clone PVC created",
		"pvc_name", created.Name,
		"namespace", namespace,
		"repo_url", repoURL,
	)
	return pvcName, nil
}

// BuildCloneCachePVC 建立 clone cache PVC 的 spec。
func BuildCloneCachePVC(name, namespace, repoURL string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "synapse",
				"app.kubernetes.io/component":  "gitops-clone-cache",
				"synapse.io/repo-hash":         CloneCacheKey(repoURL),
			},
			Annotations: map[string]string{
				"synapse.io/repo-url": repoURL,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(GitCachePVCDefaultSize),
				},
			},
		},
	}
}

// BuildGitCloneJob 建立 git clone/pull Job，將 repo clone 到 PVC 中。
// 如果 PVC 已有 clone（.git 目錄存在），執行 git pull 而非重新 clone。
func BuildGitCloneJob(
	app *models.GitOpsApp,
	pvcName, namespace string,
	gitToken string, // Git Provider token（可為空）
) *GitCloneJobSpec {
	cacheKey := CloneCacheKey(app.RepoURL)
	jobName := fmt.Sprintf("gitops-clone-%d-%s", app.ID, cacheKey[:8])
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	// repo 在 PVC 中的子目錄（同 PVC 可存多 branch）
	repoDir := fmt.Sprintf("%s/%s", GitCacheMountPath, app.Branch)

	// 建構 git clone 或 pull 的 shell 腳本
	script := buildGitScript(app.RepoURL, app.Branch, repoDir, gitToken)

	return &GitCloneJobSpec{
		JobName:   jobName,
		Namespace: namespace,
		PVCName:   pvcName,
		Image:     GitCacheCloneImage,
		Script:    script,
		RepoDir:   repoDir,
		AppPath:   app.Path, // repo 中的子路徑
		Labels: map[string]string{
			"synapse.io/managed-by":   "synapse-gitops",
			"synapse.io/gitops-app":   fmt.Sprintf("%d", app.ID),
			"synapse.io/clone-cache":  cacheKey,
		},
	}
}

// GitCloneJobSpec 描述 git clone Job 的規格。
type GitCloneJobSpec struct {
	JobName   string
	Namespace string
	PVCName   string
	Image     string
	Script    string
	RepoDir   string            // clone 的完整路徑（PVC 內）
	AppPath   string            // repo 中的子路徑
	Labels    map[string]string
}

// ManifestDir 回傳 manifest 在 PVC 中的完整路徑。
func (s *GitCloneJobSpec) ManifestDir() string {
	if s.AppPath == "" || s.AppPath == "." || s.AppPath == "/" {
		return s.RepoDir
	}
	return fmt.Sprintf("%s/%s", s.RepoDir, strings.TrimPrefix(s.AppPath, "/"))
}

// buildGitScript 建構 clone/pull shell 腳本。
// 如果已有 .git 目錄，執行 fetch + reset；否則 clone。
func buildGitScript(repoURL, branch, repoDir, token string) string {
	// 注入 token 到 URL（若有）
	authURL := repoURL
	if token != "" {
		authURL = injectGitToken(repoURL, token)
	}

	return fmt.Sprintf(`#!/bin/sh
set -e

REPO_DIR=%q
BRANCH=%q
REPO_URL=%q

if [ -d "$REPO_DIR/.git" ]; then
  echo "[synapse] git pull — cached clone exists"
  cd "$REPO_DIR"
  git remote set-url origin "$REPO_URL"
  git fetch origin "$BRANCH" --depth=1
  git reset --hard "origin/$BRANCH"
else
  echo "[synapse] git clone — first time"
  mkdir -p "$(dirname "$REPO_DIR")"
  git clone --depth=1 --single-branch --branch "$BRANCH" "$REPO_URL" "$REPO_DIR"
fi

echo "[synapse] git clone/pull complete"
git -C "$REPO_DIR" log --oneline -1
`, repoDir, branch, authURL)
}

// injectGitToken 將 token 注入 HTTPS git URL。
// https://github.com/org/repo → https://x-access-token:TOKEN@github.com/org/repo
func injectGitToken(repoURL, token string) string {
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "http://") {
		return repoURL // SSH URL 不注入 token
	}

	prefix := "https://"
	if strings.HasPrefix(repoURL, "http://") {
		prefix = "http://"
	}
	rest := strings.TrimPrefix(repoURL, prefix)
	return fmt.Sprintf("%sx-access-token:%s@%s", prefix, token, rest)
}

// DeleteClonePVC 刪除指定 repo 的 clone cache PVC。
// 用於 App 刪除後清理（如果沒有其他 App 使用同一 repo）。
func (s *GitCacheService) DeleteClonePVC(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	repoURL string,
) error {
	pvcName := PVCName(repoURL)

	// 檢查是否還有其他 App 使用同一 repo
	apps, err := s.gitopsSvc.ListAllApps(ctx)
	if err != nil {
		return fmt.Errorf("list apps for PVC cleanup: %w", err)
	}

	refCount := 0
	for _, app := range apps {
		if app.Source == models.GitOpsSourceNative && CloneCacheKey(app.RepoURL) == CloneCacheKey(repoURL) {
			refCount++
		}
	}

	if refCount > 0 {
		logger.Debug("clone PVC still in use, skipping delete",
			"pvc_name", pvcName,
			"ref_count", refCount,
		)
		return nil
	}

	err = clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete clone PVC %s: %w", pvcName, err)
	}

	logger.Info("gitops clone PVC deleted",
		"pvc_name", pvcName,
		"namespace", namespace,
	)
	return nil
}

// ListClonePVCs 列出所有 gitops clone cache PVC。
func ListClonePVCs(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
) ([]corev1.PersistentVolumeClaim, error) {
	list, err := clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=gitops-clone-cache,app.kubernetes.io/managed-by=synapse",
	})
	if err != nil {
		return nil, fmt.Errorf("list clone PVCs: %w", err)
	}
	return list.Items, nil
}

// ClonePVCInfo 快取 PVC 的前端回傳格式。
type ClonePVCInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	RepoURL   string    `json:"repo_url"`
	RepoHash  string    `json:"repo_hash"`
	Size      string    `json:"size"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ToClonePVCInfo 轉換 K8s PVC 為前端 DTO。
func ToClonePVCInfo(pvc *corev1.PersistentVolumeClaim) ClonePVCInfo {
	info := ClonePVCInfo{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
		RepoURL:   pvc.Annotations["synapse.io/repo-url"],
		RepoHash:  pvc.Labels["synapse.io/repo-hash"],
		Status:    string(pvc.Status.Phase),
		CreatedAt: pvc.CreationTimestamp.Time,
	}

	if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		info.Size = req.String()
	}
	return info
}

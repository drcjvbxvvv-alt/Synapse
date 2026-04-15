package services

import (
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// CloneCacheKey
// ---------------------------------------------------------------------------

func TestCloneCacheKey_Deterministic(t *testing.T) {
	key1 := CloneCacheKey("https://github.com/org/repo.git")
	key2 := CloneCacheKey("https://github.com/org/repo.git")
	if key1 != key2 {
		t.Errorf("expected deterministic key, got %q vs %q", key1, key2)
	}
}

func TestCloneCacheKey_NormalizesGitSuffix(t *testing.T) {
	key1 := CloneCacheKey("https://github.com/org/repo.git")
	key2 := CloneCacheKey("https://github.com/org/repo")
	if key1 != key2 {
		t.Errorf("expected same key with/without .git, got %q vs %q", key1, key2)
	}
}

func TestCloneCacheKey_CaseInsensitive(t *testing.T) {
	key1 := CloneCacheKey("https://GitHub.com/ORG/Repo")
	key2 := CloneCacheKey("https://github.com/org/repo")
	if key1 != key2 {
		t.Errorf("expected case-insensitive key, got %q vs %q", key1, key2)
	}
}

func TestCloneCacheKey_DifferentRepos(t *testing.T) {
	key1 := CloneCacheKey("https://github.com/org/repo-a")
	key2 := CloneCacheKey("https://github.com/org/repo-b")
	if key1 == key2 {
		t.Errorf("expected different keys for different repos")
	}
}

func TestCloneCacheKey_Length(t *testing.T) {
	key := CloneCacheKey("https://github.com/org/repo")
	if len(key) != 16 {
		t.Errorf("expected 16-char hex key, got %d: %q", len(key), key)
	}
}

// ---------------------------------------------------------------------------
// PVCName
// ---------------------------------------------------------------------------

func TestPVCName_HasPrefix(t *testing.T) {
	name := PVCName("https://github.com/org/repo")
	if len(name) == 0 {
		t.Fatal("expected non-empty PVC name")
	}
	if name[:len(GitCachePVCPrefix)] != GitCachePVCPrefix {
		t.Errorf("expected prefix %q, got %q", GitCachePVCPrefix, name)
	}
}

func TestPVCName_MaxLength(t *testing.T) {
	name := PVCName("https://very-long-host.example.com/organization/super-long-repository-name-that-could-be-problematic")
	if len(name) > 63 {
		t.Errorf("PVC name exceeds 63 chars: %d", len(name))
	}
}

func TestPVCName_SameRepoSameName(t *testing.T) {
	name1 := PVCName("https://github.com/org/repo.git")
	name2 := PVCName("https://github.com/org/repo")
	if name1 != name2 {
		t.Errorf("expected same PVC name, got %q vs %q", name1, name2)
	}
}

// ---------------------------------------------------------------------------
// BuildCloneCachePVC
// ---------------------------------------------------------------------------

func TestBuildCloneCachePVC_Labels(t *testing.T) {
	pvc := BuildCloneCachePVC("test-pvc", "default", "https://github.com/org/repo")

	if pvc.Name != "test-pvc" {
		t.Errorf("expected name 'test-pvc', got %q", pvc.Name)
	}
	if pvc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", pvc.Namespace)
	}
	if pvc.Labels["app.kubernetes.io/managed-by"] != "synapse" {
		t.Error("expected managed-by label = synapse")
	}
	if pvc.Labels["app.kubernetes.io/component"] != "gitops-clone-cache" {
		t.Error("expected component label = gitops-clone-cache")
	}
	if pvc.Annotations["synapse.io/repo-url"] != "https://github.com/org/repo" {
		t.Error("expected repo-url annotation")
	}
}

func TestBuildCloneCachePVC_AccessMode(t *testing.T) {
	pvc := BuildCloneCachePVC("test-pvc", "ns", "https://github.com/org/repo")

	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != "ReadWriteOnce" {
		t.Errorf("expected ReadWriteOnce, got %v", pvc.Spec.AccessModes)
	}
}

func TestBuildCloneCachePVC_StorageSize(t *testing.T) {
	pvc := BuildCloneCachePVC("test-pvc", "ns", "https://github.com/org/repo")

	req := pvc.Spec.Resources.Requests["storage"]
	if req.String() != GitCachePVCDefaultSize {
		t.Errorf("expected %s storage, got %s", GitCachePVCDefaultSize, req.String())
	}
}

// ---------------------------------------------------------------------------
// BuildGitCloneJob
// ---------------------------------------------------------------------------

func TestBuildGitCloneJob_Basic(t *testing.T) {
	app := &models.GitOpsApp{
		ID:      1,
		RepoURL: "https://github.com/org/repo",
		Branch:  "main",
		Path:    "k8s/overlays/prod",
	}

	spec := BuildGitCloneJob(app, "pvc-abc", "default", "")

	if spec.PVCName != "pvc-abc" {
		t.Errorf("expected PVC name 'pvc-abc', got %q", spec.PVCName)
	}
	if spec.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", spec.Namespace)
	}
	if spec.Image != GitCacheCloneImage {
		t.Errorf("expected image %q, got %q", GitCacheCloneImage, spec.Image)
	}
	if len(spec.JobName) > 63 {
		t.Errorf("job name exceeds 63 chars: %d", len(spec.JobName))
	}
}

func TestBuildGitCloneJob_ManifestDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"k8s/overlays/prod", "/workspace/repo/main/k8s/overlays/prod"},
		{"", "/workspace/repo/main"},
		{".", "/workspace/repo/main"},
		{"/", "/workspace/repo/main"},
		{"/deploy", "/workspace/repo/main/deploy"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			app := &models.GitOpsApp{
				ID:      1,
				RepoURL: "https://github.com/org/repo",
				Branch:  "main",
				Path:    tt.path,
			}
			spec := BuildGitCloneJob(app, "pvc", "ns", "")
			got := spec.ManifestDir()
			if got != tt.want {
				t.Errorf("ManifestDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildGitCloneJob_ScriptContainsClone(t *testing.T) {
	app := &models.GitOpsApp{
		ID:      2,
		RepoURL: "https://github.com/org/repo",
		Branch:  "develop",
		Path:    ".",
	}

	spec := BuildGitCloneJob(app, "pvc", "ns", "")

	if spec.Script == "" {
		t.Fatal("expected non-empty script")
	}
	// Script should contain both clone and pull logic
	if !containsStr(spec.Script, "git clone") {
		t.Error("script should contain 'git clone'")
	}
	if !containsStr(spec.Script, "git fetch") {
		t.Error("script should contain 'git fetch'")
	}
}

func TestBuildGitCloneJob_WithToken(t *testing.T) {
	app := &models.GitOpsApp{
		ID:      3,
		RepoURL: "https://github.com/org/private-repo",
		Branch:  "main",
		Path:    ".",
	}

	spec := BuildGitCloneJob(app, "pvc", "ns", "ghp_testtoken123")

	// Token should be injected into the URL in the script
	if !containsStr(spec.Script, "x-access-token:ghp_testtoken123@github.com") {
		t.Error("script should contain token-injected URL")
	}
}

// ---------------------------------------------------------------------------
// injectGitToken
// ---------------------------------------------------------------------------

func TestInjectGitToken_HTTPS(t *testing.T) {
	tests := []struct {
		url   string
		token string
		want  string
	}{
		{
			"https://github.com/org/repo",
			"token123",
			"https://x-access-token:token123@github.com/org/repo",
		},
		{
			"http://gitlab.local/group/project",
			"glpat-abc",
			"http://x-access-token:glpat-abc@gitlab.local/group/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := injectGitToken(tt.url, tt.token)
			if got != tt.want {
				t.Errorf("injectGitToken(%q, %q) = %q, want %q", tt.url, tt.token, got, tt.want)
			}
		})
	}
}

func TestInjectGitToken_SSHUnchanged(t *testing.T) {
	url := "git@github.com:org/repo.git"
	got := injectGitToken(url, "token123")
	if got != url {
		t.Errorf("expected SSH URL unchanged, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// ToClonePVCInfo
// ---------------------------------------------------------------------------

func TestToClonePVCInfo_Basic(t *testing.T) {
	pvc := BuildCloneCachePVC("test-pvc", "ns", "https://github.com/org/repo")
	info := ToClonePVCInfo(pvc)

	if info.Name != "test-pvc" {
		t.Errorf("expected name 'test-pvc', got %q", info.Name)
	}
	if info.Namespace != "ns" {
		t.Errorf("expected namespace 'ns', got %q", info.Namespace)
	}
	if info.RepoURL != "https://github.com/org/repo" {
		t.Errorf("expected repo URL, got %q", info.RepoURL)
	}
	if info.Size != GitCachePVCDefaultSize {
		t.Errorf("expected size %s, got %q", GitCachePVCDefaultSize, info.Size)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

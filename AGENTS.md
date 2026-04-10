- Think before acting. Read existing files before writing code.
- Be concise in output but thorough in reasoning.
- Prefer editing over rewriting whole files.
- Do not re-read files you have already read.
- Test your code before declaring done.
- No sycophantic openers or closing fluff.
- Keep solutions simple and direct.
- User instructions always override this file.

# Project Brain

Project Brain is installed at `/Users/ahern/Documents/AI-tools/OpenScoure/Synapse`.

## Memory System Instructions

At the start of every task, call the `get_context` MCP tool
with the task description and current file path.

If Brain returns nudges or warnings, treat them as **hard constraints**.

When you discover any of the following, call `add_knowledge` immediately
— do not wait until the end of the task:

- A bug and the reason it happened (kind: Pitfall)
- An architectural decision and why (kind: Decision)
- A rule that must always be followed (kind: Rule)
- Something that does not work as expected (kind: Pitfall)

Use confidence=0.9 for verified facts, 0.7 for reasonable inferences.

## Task Start Protocol

Before beginning **any** task:

1. Call `get_context` with the task description and current file path.
2. If the result contains **Pitfall** entries, read each one carefully and
   explicitly state how you will avoid that mistake before writing code.
3. If the result contains **Rule** entries, those rules are mandatory for
   this task — do not deviate from them.
4. If the result contains **Decision** entries, treat them as established
   architecture — do not reverse them without discussion.

## Task Complete Protocol

After completing **any** non-trivial task:

1. Call `complete_task` with:
   - `task_description`: one-sentence summary of what was done
   - `decisions`: list of architectural or design choices made (can be empty)
   - `lessons`: list of things learned that would help future work (can be empty)
   - `pitfalls`: list of mistakes encountered or near-misses (can be empty)
2. If a **new bug pattern** was discovered during the task, also call
   `add_knowledge(kind="Pitfall", ...)` immediately — do not rely solely on
   `complete_task` for Pitfall recording.
3. If an **important architectural decision** was made, call
   `add_knowledge(kind="Decision", ...)` as well.

## Knowledge Feedback Protocol

After a task that used knowledge retrieved from Brain:

- If a retrieved knowledge node **directly helped** complete the task correctly,
  call `report_knowledge_outcome(node_id=..., was_useful=True)`.
- If a retrieved knowledge node was **outdated, incorrect, or irrelevant**,
  call `report_knowledge_outcome(node_id=..., was_useful=False, notes="reason")`.

This feedback loop keeps confidence scores accurate and prevents stale knowledge
from surfacing in future queries.

# Synapse Backend — Go & Gin Development Guide

# Codex MUST read this file completely before writing any backend code.

> Version: v1.0 | Scope: all files under internal/ cmd/ pkg/
> Violations must be corrected before committing.

---

## Table of Contents

1. [Project Layout & Package Rules](#1-project-layout--package-rules)
2. [Handler Pattern](#2-handler-pattern)
3. [Service Layer Pattern](#3-service-layer-pattern)
4. [Error Handling](#4-error-handling)
5. [Context Usage](#5-context-usage)
6. [Database (GORM)](#6-database-gorm)
7. [Kubernetes Client Usage](#7-kubernetes-client-usage)
8. [Observer Pattern for Optional Components](#8-observer-pattern-for-optional-components)
9. [Logging](#9-logging)
10. [Security Rules](#10-security-rules)
11. [Route Registration](#11-route-registration)
12. [Testing](#12-testing)
13. [Complete Handler Template](#13-complete-handler-template)
14. [Quick Checklist](#14-quick-checklist)

---

## 1. Project Layout & Package Rules

```
internal/
  handlers/      HTTP handlers — parse request, call service, write response
  services/      Business logic, K8s API calls, external integrations
  models/        GORM model structs (DB schema source of truth)
  middleware/    Gin middleware (auth, rate-limit, audit, CORS)
  router/        Route registration only — no business logic here
  k8s/           K8s client lifecycle (ClusterInformerManager)
  config/        App configuration structs
  database/      DB init, migration
  apierrors/     Structured AppError type + error code constants
  response/      Gin response helpers (OK, List, BadRequest, …)
  metrics/       Prometheus metrics registry
pkg/
  logger/        Structured logger (slog wrapper)
  crypto/        AES-256-GCM encryption, KeyProvider interface
cmd/
  admin/         Admin CLI subcommands (rotate-key, …)
```

### Package dependency rules (NEVER violate)

```
handlers  →  services, models, response, apierrors, logger, k8s, middleware
services  →  models, logger, pkg/crypto
models    →  pkg/crypto (BeforeSave / AfterFind hooks only)
router    →  handlers, middleware, services (for dependency injection only)
```

- **handlers MUST NOT import other handlers**
- **services MUST NOT import handlers or response**
- **models MUST NOT import services or handlers**
- A package that is not in the allowed list above must NOT be imported

---

## 2. Handler Pattern

Every handler follows the exact same 5-step flow. Do not invent variations.

```go
// Step 1: Parse & validate path/query params
// Step 2: Resolve cluster → get K8s client
// Step 3: Build context with timeout
// Step 4: Call K8s API or service
// Step 5: Write response

func (h *ExampleHandler) GetResource(c *gin.Context) {
    // ── Step 1: Parse params ──────────────────────────────────────────────
    clusterID, err := parseClusterID(c.Param("clusterID"))
    if err != nil {
        response.BadRequest(c, "invalid cluster ID")
        return
    }
    namespace := c.Param("namespace")
    name      := c.Param("name")

    // ── Step 2: Resolve cluster + K8s client ─────────────────────────────
    cluster, err := h.clusterService.GetCluster(clusterID)
    if err != nil {
        response.NotFound(c, "cluster not found")
        return
    }
    k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
    if err != nil {
        response.InternalError(c, "failed to get K8s client: "+err.Error())
        return
    }

    // ── Step 3: Context with timeout ──────────────────────────────────────
    ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
    defer cancel()

    // ── Step 4: K8s API call ──────────────────────────────────────────────
    obj, err := k8sClient.GetClientset().CoreV1().
        ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        response.NotFound(c, "resource not found: "+err.Error())
        return
    }

    // ── Step 5: Response ──────────────────────────────────────────────────
    response.OK(c, convertToInfo(obj))
}
```

### Handler struct construction

```go
// Handler holds ONLY what it directly needs.
// NEVER inject a dependency the handler does not use itself.
type ExampleHandler struct {
    clusterService *services.ClusterService  // always needed
    k8sMgr         *k8s.ClusterInformerManager  // always needed for K8s ops
    db             *gorm.DB                  // only if handler writes to DB
    cfg            *config.Config            // only if handler reads config
}

func NewExampleHandler(
    clusterService *services.ClusterService,
    k8sMgr *k8s.ClusterInformerManager,
) *ExampleHandler {
    return &ExampleHandler{
        clusterService: clusterService,
        k8sMgr:         k8sMgr,
    }
}
```

### Request binding

```go
// JSON body
var req CreateResourceRequest
if err := c.ShouldBindJSON(&req); err != nil {
    response.BadRequest(c, "invalid request body: "+err.Error())
    return
}

// Query params with defaults
page, _     := strconv.Atoi(c.DefaultQuery("page", "1"))
pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
namespace   := c.DefaultQuery("namespace", "")
search      := c.Query("search")
```

### NEVER do these in a handler

```go
// ❌ Business logic belongs in service
if user.Role == "admin" && cluster.Status == "healthy" { ... }

// ❌ Direct DB queries in handler
h.db.Where("cluster_id = ?", id).Find(&items)

// ❌ Swallowing errors silently
obj, _ := clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})

// ❌ log.Printf — use the project logger
log.Printf("error: %v", err)

// ❌ c.JSON(200, ...) directly — use response helpers
c.JSON(200, gin.H{"data": result})
```

---

## 3. Service Layer Pattern

Services contain all business logic and K8s API calls that require
more than one step. Keep handlers thin.

```go
// Service struct
type ExampleService struct {
    db      *gorm.DB
    // add only what the service directly needs
}

func NewExampleService(db *gorm.DB) *ExampleService {
    return &ExampleService{db: db}
}

// Service method signature rules:
//   - First param is always context.Context
//   - Return (result, error) — never return (result, bool, error)
//   - Use named return values only when it genuinely improves clarity
func (s *ExampleService) GetResource(ctx context.Context, id uint) (*ResourceInfo, error) {
    var model models.Resource
    if err := s.db.WithContext(ctx).First(&model, id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, fmt.Errorf("resource %d not found: %w", id, err)
        }
        return nil, fmt.Errorf("query resource: %w", err)
    }
    return toResourceInfo(&model), nil
}
```

### Service method naming

| Operation             | Name pattern                             |
| --------------------- | ---------------------------------------- |
| Read single           | `Get<Resource>`                          |
| Read list             | `List<Resource>s`                        |
| Create                | `Create<Resource>`                       |
| Update                | `Update<Resource>`                       |
| Delete                | `Delete<Resource>`                       |
| Toggle/state change   | `Enable<Resource>` / `Disable<Resource>` |
| Check capability      | `Is<Feature>Available` / `Has<Feature>`  |
| Detect installed tool | `Detect<Tool>`                           |

---

## 4. Error Handling

### Rule 1 — Always wrap errors with context

```go
// ❌ Bare error return — no context
return nil, err

// ✅ Always wrap with fmt.Errorf + %w
return nil, fmt.Errorf("list deployments in %s: %w", namespace, err)
return nil, fmt.Errorf("encrypt kubeconfig for cluster %d: %w", id, err)
```

### Rule 2 — Use apierrors.AppError for domain errors

```go
import "github.com/shaia/Synapse/internal/apierrors"

// In service layer — return structured error
if !authorized {
    return nil, &apierrors.AppError{
        Code:       apierrors.CodeAuthUnauthorized,
        HTTPStatus: http.StatusForbidden,
        Message:    "user lacks permission for this cluster",
    }
}

// In handler layer — check for AppError first
result, err := h.service.DoSomething(ctx, id)
if err != nil {
    if ae, ok := apierrors.As(err); ok {
        c.JSON(ae.HTTPStatus, response.ErrorBody{
            Error: response.ErrorDetail{Code: ae.Code, Message: ae.Message},
        })
        return
    }
    response.InternalError(c, err.Error())
    return
}
```

### Rule 3 — Map K8s errors correctly

```go
import k8serrors "k8s.io/apimachinery/pkg/api/errors"

obj, err := clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
if err != nil {
    if k8serrors.IsNotFound(err) {
        response.NotFound(c, "pod not found: "+name)
        return
    }
    if k8serrors.IsUnauthorized(err) || k8serrors.IsForbidden(err) {
        response.Forbidden(c, "insufficient cluster permissions")
        return
    }
    response.InternalError(c, "K8s API error: "+err.Error())
    return
}
```

### Rule 4 — Never ignore errors

```go
// ❌ Blank identifier on error
result, _ := doSomethingImportant()

// ✅ Log non-critical errors; return critical ones
result, err := doSomethingBestEffort()
if err != nil {
    logger.Warn("best-effort operation failed, continuing", "error", err)
}
```

---

## 5. Context Usage

### Use c.Request.Context() for request-scoped operations

```go
// ✅ Preferred — inherits request cancellation and tracing
ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
defer cancel()
```

### Use context.Background() only for background goroutines

```go
// ✅ Correct — background worker not tied to any HTTP request
func (w *CertExpiryWorker) Start() {
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
        defer cancel()
        w.scan(ctx)
    }()
}

// ❌ Wrong — handler using Background() loses request cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // in handler
```

### Timeout values

| Operation                               | Timeout |
| --------------------------------------- | ------- |
| K8s API single resource GET             | 30s     |
| K8s API list (large)                    | 60s     |
| Informer sync wait                      | 5s      |
| External HTTP call (Prometheus, ArgoCD) | 15s     |
| Database query                          | 10s     |
| Background worker cycle                 | 2min    |

---

## 6. Database (GORM)

### Model conventions

```go
type ResourceModel struct {
    ID        uint           `json:"id"         gorm:"primaryKey;autoIncrement"`
    Name      string         `json:"name"       gorm:"not null;size:255;uniqueIndex:idx_name_cluster"`
    ClusterID uint           `json:"cluster_id" gorm:"not null;index;uniqueIndex:idx_name_cluster"`
    Status    string         `json:"status"     gorm:"size:20;default:'unknown'"`
    Meta      string         `json:"-"          gorm:"type:json"`  // JSON blob — hide from API
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `json:"-"          gorm:"index"`      // soft delete
}

func (ResourceModel) TableName() string { return "resources" }
```

### Query patterns

```go
// ✅ Always pass context
h.db.WithContext(ctx).Where("cluster_id = ?", clusterID).Find(&items)

// ✅ Single record — use First (returns ErrRecordNotFound when missing)
var model models.Resource
if err := h.db.WithContext(ctx).First(&model, id).Error; err != nil { ... }

// ✅ Multi-record — use Find (returns empty slice, not error, when missing)
var items []models.Resource
h.db.WithContext(ctx).Where("cluster_id = ?", id).Find(&items)

// ✅ Transaction for multi-table writes
err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&parent).Error; err != nil {
        return fmt.Errorf("create parent: %w", err)
    }
    if err := tx.Create(&child).Error; err != nil {
        return fmt.Errorf("create child: %w", err)
    }
    return nil
})
```

### NEVER do these with GORM

```go
// ❌ Raw SQL for things GORM can express
h.db.Raw("SELECT * FROM clusters WHERE id = ?", id).Scan(&result)

// ❌ Query without context
h.db.Where("id = ?", id).Find(&items)

// ❌ Select * on large tables — specify columns
h.db.Find(&items)  // fetches all columns including blobs

// ✅ Specify columns when you don't need everything
h.db.WithContext(ctx).
    Select("id, name, status, created_at").
    Where("cluster_id = ?", id).
    Find(&items)
```

---

## 7. Kubernetes Client Usage

### Two access paths — choose correctly

```go
// Path A: Informer cache (read-only, fast, no API call)
// Use for: LIST operations in handler GET endpoints
deps, err := h.k8sMgr.DeploymentsLister(clusterID).
    Deployments(namespace).List(labels.Everything())

// Path B: Live API client (read-write, slower)
// Use for: GET single resource details, all write operations
k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
clientset := k8sClient.GetClientset()
obj, err := clientset.AppsV1().Deployments(namespace).
    Get(ctx, name, metav1.GetOptions{})
```

### YAML output — always clean ManagedFields

```go
// NEVER return raw K8s object YAML to the frontend
// ALWAYS strip managedFields and set TypeMeta

clean := obj.DeepCopy()
clean.ManagedFields = nil
clean.APIVersion = "apps/v1"   // client-go omits this by default
clean.Kind = "Deployment"

yamlBytes, err := sigsyaml.Marshal(clean)
```

### Dynamic client for CRDs

```go
// For resources without a typed client (CRDs, Gateway API, KEDA, etc.)
import (
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
)

var scaledObjectGVR = schema.GroupVersionResource{
    Group:    "keda.sh",
    Version:  "v1alpha1",
    Resource: "scaledobjects",
}

dynClient := k8sClient.GetDynamicClient()
list, err := dynClient.Resource(scaledObjectGVR).
    Namespace(namespace).
    List(ctx, metav1.ListOptions{})
```

---

## 8. Observer Pattern for Optional Components

**Synapse NEVER forces users to install additional components.**
For every optional component (Istio, KEDA, Cilium, Kyverno, Gateway API, Velero, Flux…):

1. Detect via Discovery API — never assume installed
2. Return graceful degradation when not installed
3. Never return an error that blocks the parent page from loading

```go
// Standard detection template
func (s *FeatureService) IsInstalled(ctx context.Context, clientset kubernetes.Interface) bool {
    // Try multiple API versions for forward/backward compatibility
    for _, version := range []string{"v1", "v1beta1", "v1alpha1"} {
        _, err := clientset.Discovery().ServerResourcesForGroupVersion(
            "feature.example.com/" + version,
        )
        if err == nil {
            return true
        }
        // Only IsNotFound means truly absent; other errors may be transient
        if !k8serrors.IsNotFound(err) {
            logger.Warn("discovery API transient error",
                "group", "feature.example.com",
                "version", version,
                "error", err,
            )
        }
    }
    return false
}

// Handler — graceful degradation
func (h *FeatureHandler) GetStatus(c *gin.Context) {
    // ... resolve cluster ...
    installed := h.featureSvc.IsInstalled(ctx, k8sClient.GetClientset())
    response.OK(c, gin.H{
        "installed": installed,
        "version":   "", // populate only when installed
    })
    // Do NOT return 404 or 500 when the component is absent
}
```

---

## 9. Logging

### Use only the project logger — never fmt.Println or log.Printf

```go
import "github.com/shaia/Synapse/pkg/logger"

// Structured key-value pairs (preferred)
logger.Info("deployment scaled",
    "cluster_id", clusterID,
    "namespace", namespace,
    "name", name,
    "replicas", replicas,
)

logger.Error("K8s API call failed",
    "error", err,
    "cluster_id", clusterID,
)

logger.Warn("optional component not installed, skipping",
    "component", "cilium",
    "cluster_id", clusterID,
)
```

### What to log at each level

| Level   | When                                                                       |
| ------- | -------------------------------------------------------------------------- |
| `Debug` | Detailed flow tracing (disabled in production)                             |
| `Info`  | Every state-changing operation entry point (create, update, delete, scale) |
| `Warn`  | Expected-but-notable conditions (component absent, degraded mode, retry)   |
| `Error` | Actual failures that affect the response or system state                   |

### NEVER log sensitive data

```go
// ❌ Never log these fields
logger.Info("cluster imported", "kubeconfig", kubeconfig)
logger.Info("user login", "password", password)
logger.Info("token created", "token", token)
```

---

## 10. Security Rules

### TLS verification

```go
// ❌ NEVER hardcode InsecureSkipVerify: true
TLSClientConfig: &tls.Config{InsecureSkipVerify: true}

// ✅ Make it configurable via the model / config
TLSClientConfig: &tls.Config{
    InsecureSkipVerify: config.SkipTLSVerify, // user-controlled
    RootCAs:            caPool,               // use CA when provided
}

// Exception: one-time certificate inspection only (use nolint comment)
//nolint:gosec // used only to read cert expiry, not to trust the connection
conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
```

### Sensitive fields in models

```go
// Fields that must NEVER appear in JSON responses
KubeconfigEnc string `json:"-" gorm:"type:text"`
SATokenEnc    string `json:"-" gorm:"type:text"`
CAEnc         string `json:"-" gorm:"type:text"`
PasswordHash  string `json:"-" gorm:"size:255"`

// Encrypted via BeforeSave hook — see models/cluster.go for reference
```

### Permission checks — NEVER use username string comparison

```go
// ❌ Hardcoded superuser check — bypassed by DB manipulation
if username == "admin" { ... }

// ✅ Role-based check
if user.SystemRole == models.RoleSuperAdmin { ... }
// or via middleware: middleware.PlatformAdminRequired(d.db)
```

### User input in K8s API calls

```go
// ❌ Never use raw user input as a label selector
selector := c.Query("selector") // could be: "a=b,..malicious"
clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
    LabelSelector: selector,
})

// ✅ Validate and sanitize before use
selector, err := labels.Parse(c.Query("selector"))
if err != nil {
    response.BadRequest(c, "invalid selector: "+err.Error())
    return
}
```

---

## 11. Route Registration

All routes live in `internal/router/routes_*.go`. Do NOT define routes in handlers.

```go
// Pattern for a new resource group
func registerExampleRoutes(cluster *gin.RouterGroup, d *routeDeps) {
    handler := handlers.NewExampleHandler(d.clusterSvc, d.k8sMgr)

    examples := cluster.Group("/examples")
    {
        examples.GET("",                   handler.List)
        examples.GET("/:namespace",        handler.ListByNamespace)
        examples.GET("/:namespace/:name",  handler.Get)
        examples.POST("",                  handler.Create)
        examples.PUT("/:namespace/:name",  handler.Update)
        examples.DELETE("/:namespace/:name", handler.Delete)

        // Sub-resources
        examples.GET("/:namespace/:name/yaml",     handler.GetYAML)
        examples.POST("/:namespace/:name/restart", handler.Restart)
    }
}
```

### Middleware application rules

```go
// Applied to ALL cluster routes already (in routes_cluster.go):
//   - ClusterAccessRequired() — verifies user has access to this cluster
//   - AutoWriteCheck()        — POST/PUT/DELETE requires non-readonly role

// Add to specific routes only when needed:
clusters.POST("/import", middleware.PlatformAdminRequired(d.db), handler.Import)
```

---

## 12. Testing

### File naming

```
internal/handlers/example_test.go   ← handler test
internal/services/example_test.go   ← service test
```

### Handler test template

```go
func TestExampleHandler_Get(t *testing.T) {
    gin.SetMode(gin.TestMode)

    t.Run("returns 200 with valid cluster and resource", func(t *testing.T) {
        // Arrange
        w   := httptest.NewRecorder()
        ctx, _ := gin.CreateTestContext(w)
        ctx.Params = gin.Params{
            {Key: "clusterID", Value: "1"},
            {Key: "namespace", Value: "default"},
            {Key: "name",      Value: "my-resource"},
        }

        // Act
        handler.Get(ctx)

        // Assert
        assert.Equal(t, http.StatusOK, w.Code)
        var body map[string]any
        require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
        assert.Equal(t, "my-resource", body["name"])
    })

    t.Run("returns 400 for invalid cluster ID", func(t *testing.T) {
        w   := httptest.NewRecorder()
        ctx, _ := gin.CreateTestContext(w)
        ctx.Params = gin.Params{{Key: "clusterID", Value: "not-a-number"}}

        handler.Get(ctx)

        assert.Equal(t, http.StatusBadRequest, w.Code)
    })
}
```

---

## 13. Complete Handler Template

Copy this template verbatim when adding a new resource handler.

```go
package handlers

import (
    "context"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/shaia/Synapse/internal/k8s"
    "github.com/shaia/Synapse/internal/response"
    "github.com/shaia/Synapse/internal/services"
    "github.com/shaia/Synapse/pkg/logger"

    k8serrors "k8s.io/apimachinery/pkg/api/errors"
    metav1    "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ─── Struct ────────────────────────────────────────────────────────────────

// ExampleHandler manages Example resources.
type ExampleHandler struct {
    clusterService *services.ClusterService
    k8sMgr         *k8s.ClusterInformerManager
}

// NewExampleHandler wires dependencies for ExampleHandler.
func NewExampleHandler(
    clusterSvc *services.ClusterService,
    k8sMgr *k8s.ClusterInformerManager,
) *ExampleHandler {
    return &ExampleHandler{
        clusterService: clusterSvc,
        k8sMgr:         k8sMgr,
    }
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// ExampleInfo is the API response shape for an Example resource.
type ExampleInfo struct {
    Name      string `json:"name"`
    Namespace string `json:"namespace"`
    Status    string `json:"status"`
}

// CreateExampleRequest is the request body for POST /examples.
type CreateExampleRequest struct {
    Name      string `json:"name"      binding:"required,max=253"`
    Namespace string `json:"namespace" binding:"required"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// List returns all Examples in the given namespace.
// GET /clusters/:clusterID/examples?namespace=default
func (h *ExampleHandler) List(c *gin.Context) {
    clusterID, err := parseClusterID(c.Param("clusterID"))
    if err != nil {
        response.BadRequest(c, "invalid cluster ID")
        return
    }
    namespace := c.DefaultQuery("namespace", "")

    cluster, err := h.clusterService.GetCluster(clusterID)
    if err != nil {
        response.NotFound(c, "cluster not found")
        return
    }
    k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
    if err != nil {
        response.InternalError(c, "failed to get K8s client: "+err.Error())
        return
    }

    ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
    defer cancel()

    list, err := k8sClient.GetClientset().CoreV1().
        ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        logger.Error("failed to list examples", "error", err, "cluster_id", clusterID)
        response.InternalError(c, "failed to list examples: "+err.Error())
        return
    }

    items := make([]ExampleInfo, 0, len(list.Items))
    for i := range list.Items {
        items = append(items, convertToExampleInfo(&list.Items[i]))
    }
    response.List(c, items, int64(len(items)))
}

// Get returns a single Example by name.
// GET /clusters/:clusterID/examples/:namespace/:name
func (h *ExampleHandler) Get(c *gin.Context) {
    clusterID, err := parseClusterID(c.Param("clusterID"))
    if err != nil {
        response.BadRequest(c, "invalid cluster ID")
        return
    }
    namespace := c.Param("namespace")
    name      := c.Param("name")

    cluster, err := h.clusterService.GetCluster(clusterID)
    if err != nil {
        response.NotFound(c, "cluster not found")
        return
    }
    k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
    if err != nil {
        response.InternalError(c, "failed to get K8s client: "+err.Error())
        return
    }

    ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
    defer cancel()

    obj, err := k8sClient.GetClientset().CoreV1().
        ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
    if err != nil {
        if k8serrors.IsNotFound(err) {
            response.NotFound(c, "example not found: "+name)
            return
        }
        response.InternalError(c, "K8s API error: "+err.Error())
        return
    }

    response.OK(c, convertToExampleInfo(obj))
}

// Create creates a new Example resource.
// POST /clusters/:clusterID/examples
func (h *ExampleHandler) Create(c *gin.Context) {
    clusterID, err := parseClusterID(c.Param("clusterID"))
    if err != nil {
        response.BadRequest(c, "invalid cluster ID")
        return
    }

    var req CreateExampleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "invalid request body: "+err.Error())
        return
    }

    cluster, err := h.clusterService.GetCluster(clusterID)
    if err != nil {
        response.NotFound(c, "cluster not found")
        return
    }
    k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
    if err != nil {
        response.InternalError(c, "failed to get K8s client: "+err.Error())
        return
    }

    ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
    defer cancel()

    logger.Info("creating example",
        "cluster_id", clusterID,
        "namespace", req.Namespace,
        "name", req.Name,
    )

    // Build K8s object from request
    // obj := &corev1.ConfigMap{ ... }

    // created, err := k8sClient.GetClientset().CoreV1().
    //     ConfigMaps(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
    // if err != nil {
    //     if k8serrors.IsAlreadyExists(err) {
    //         response.Conflict(c, "example already exists: "+req.Name)
    //         return
    //     }
    //     response.InternalError(c, "failed to create example: "+err.Error())
    //     return
    // }

    _ = ctx
    response.OK(c, gin.H{"message": "created"})
}

// Delete removes an Example resource.
// DELETE /clusters/:clusterID/examples/:namespace/:name
func (h *ExampleHandler) Delete(c *gin.Context) {
    clusterID, err := parseClusterID(c.Param("clusterID"))
    if err != nil {
        response.BadRequest(c, "invalid cluster ID")
        return
    }
    namespace := c.Param("namespace")
    name      := c.Param("name")

    cluster, err := h.clusterService.GetCluster(clusterID)
    if err != nil {
        response.NotFound(c, "cluster not found")
        return
    }
    k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
    if err != nil {
        response.InternalError(c, "failed to get K8s client: "+err.Error())
        return
    }

    ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
    defer cancel()

    logger.Info("deleting example",
        "cluster_id", clusterID,
        "namespace", namespace,
        "name", name,
    )

    err = k8sClient.GetClientset().CoreV1().
        ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
    if err != nil {
        if k8serrors.IsNotFound(err) {
            response.NotFound(c, "example not found: "+name)
            return
        }
        response.InternalError(c, "failed to delete example: "+err.Error())
        return
    }

    response.OK(c, gin.H{"message": "deleted"})
}

// ─── Converters ────────────────────────────────────────────────────────────

func convertToExampleInfo(obj any) ExampleInfo {
    // Convert K8s object to API response shape
    return ExampleInfo{}
}
```

---

## 14. Quick Checklist

Before committing ANY backend code, verify all 12 points:

```
□  1. Handler follows 5-step flow: parse → cluster → ctx → K8s → response
□  2. All errors wrapped with fmt.Errorf("operation: %w", err)
□  3. K8s errors mapped with k8serrors.IsNotFound / IsForbidden / etc.
□  4. context.WithTimeout uses c.Request.Context() in handlers
□  5. All DB queries have .WithContext(ctx)
□  6. YAML output has ManagedFields stripped and TypeMeta set
□  7. Optional components use IsInstalled() detection, never assume present
□  8. No InsecureSkipVerify: true without nolint comment + justification
□  9. No username == "admin" hardcoded superuser logic
□ 10. No sensitive fields (token, kubeconfig, password) in logs or JSON
□ 11. New routes registered in internal/router/routes_*.go, not in handler
□ 12. logger.Info called at every state-changing entry point
```

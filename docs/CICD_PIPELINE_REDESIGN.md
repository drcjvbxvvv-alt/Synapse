# CI/CD Pipeline Architecture Redesign: Top-Level Pipeline + Environment-per-Cluster

> Version: v1.0 | Status: DRAFT — Pending Review
> Date: 2026-04-15

---

## 1. Problem Statement

### Current Architecture (Wrong)

```
Cluster-1
  └── Pipeline-A (bound to Cluster-1)
       └── Environment: dev  → Cluster-1
       └── Environment: prod → Cluster-3  ← contradiction: pipeline is on Cluster-1 but env points elsewhere
```

- `Pipeline.ClusterID` ties the pipeline to a single cluster
- All routes are under `/clusters/:clusterID/pipelines/...`
- `Pipeline.Namespace` is meaningless when environments span multiple clusters
- PipelineRun executes in the pipeline's cluster, not the target environment's cluster
- No natural way to express "deploy same artifact to dev first, then staging, then prod across different clusters"

### Target Architecture (Correct)

```
Pipeline-A (top-level, no cluster binding)
  ├── Environment: dev     → Cluster-1 / namespace: app-dev
  ├── Environment: staging → Cluster-2 / namespace: app-staging
  └── Environment: prod    → Cluster-3 / namespace: app-prod

Pipeline-B (top-level)
  ├── Environment: dev     → Cluster-1 / namespace: svc-b-dev
  └── Environment: prod    → Cluster-1 / namespace: svc-b-prod   ← same cluster, different NS
```

- Pipeline is a **definition** — it defines _what_ to build and deploy
- Environment is the **execution target** — it defines _where_ to deploy
- A PipelineRun always targets a specific Environment
- Promotion = trigger the same pipeline version in the next Environment

---

## 2. Data Model Changes

### 2.1 Pipeline Model — Remove Cluster Binding

```go
// BEFORE
type Pipeline struct {
    ID                 uint   `gorm:"primaryKey;autoIncrement"`
    Name               string `gorm:"not null;size:255;uniqueIndex:idx_pipeline_name_cluster"`
    Description        string `gorm:"size:1000"`
    ClusterID          uint   `gorm:"not null;index;uniqueIndex:idx_pipeline_name_cluster"`  // ← REMOVE
    Namespace          string `gorm:"not null;size:255"`                                      // ← REMOVE
    CurrentVersionID   *uint
    // ... concurrency, notifications, timestamps
}

// AFTER
type Pipeline struct {
    ID                 uint   `gorm:"primaryKey;autoIncrement"`
    Name               string `gorm:"not null;size:255;uniqueIndex:idx_pipeline_name"`  // globally unique
    Description        string `gorm:"size:1000"`
    CurrentVersionID   *uint
    ConcurrencyGroup   string `gorm:"size:255"`
    ConcurrencyPolicy  string `gorm:"size:20;default:'queue'"`
    MaxConcurrentRuns  int    `gorm:"default:1"`
    NotifyOnSuccess    string `gorm:"type:json"`
    NotifyOnFailure    string `gorm:"type:json"`
    NotifyOnScan       string `gorm:"type:json"`
    CreatedBy          string `gorm:"size:255"`
    CreatedAt          time.Time
    UpdatedAt          time.Time
    DeletedAt          gorm.DeletedAt `gorm:"index"`
}
```

**Changes:**
- Remove `ClusterID` — pipeline is no longer cluster-scoped
- Remove `Namespace` — namespace is per-environment
- Change unique index from `(name, cluster_id)` to just `(name)` — pipeline names are globally unique

### 2.2 Environment Model — Becomes the Cluster Bridge

```go
// BEFORE
type Environment struct {
    ID                uint   `gorm:"primaryKey;autoIncrement"`
    Name              string `gorm:"not null;size:100"`
    PipelineID        uint   `gorm:"not null;index"`
    ClusterID         uint   `gorm:"not null;index"`     // already correct
    Namespace         string `gorm:"not null;size:255"`   // already correct
    OrderIndex        int    `gorm:"not null;default:0"`
    // ...
}

// AFTER — add unique constraint and variables
type Environment struct {
    ID                 uint   `gorm:"primaryKey;autoIncrement"`
    Name               string `gorm:"not null;size:100;uniqueIndex:idx_env_pipeline_name"`
    PipelineID         uint   `gorm:"not null;index;uniqueIndex:idx_env_pipeline_name"`
    ClusterID          uint   `gorm:"not null;index"`
    Namespace          string `gorm:"not null;size:255"`
    OrderIndex         int    `gorm:"not null;default:0"`
    AutoPromote        bool   `gorm:"default:false"`
    ApprovalRequired   bool   `gorm:"default:false"`
    ApproverIDs        string `gorm:"type:json"`
    SmokeTestStepName  string `gorm:"size:255"`
    NotifyChannelIDs   string `gorm:"type:json"`
    VariablesJSON      string `gorm:"type:json"`           // NEW: env-specific variables (e.g. replicas, image tag overrides)
    CreatedAt          time.Time
    UpdatedAt          time.Time
    DeletedAt          gorm.DeletedAt `gorm:"index"`
}
```

**Changes:**
- Add unique constraint `(pipeline_id, name)` — no duplicate env names per pipeline
- Add `VariablesJSON` — environment-specific variable overrides (replicas, feature flags, etc.)
- `ClusterID` + `Namespace` remain — this is now the **sole** place that maps to a cluster

### 2.3 PipelineRun Model — Add Environment Reference

```go
// BEFORE
type PipelineRun struct {
    ID               uint   `gorm:"primaryKey;autoIncrement"`
    PipelineID       uint   `gorm:"not null;index"`
    SnapshotID       uint   `gorm:"not null"`
    Status           string
    TriggerType      string
    // ... no environment reference
}

// AFTER
type PipelineRun struct {
    ID               uint   `gorm:"primaryKey;autoIncrement"`
    PipelineID       uint   `gorm:"not null;index"`
    EnvironmentID    uint   `gorm:"not null;index"`           // NEW: which env this run targets
    SnapshotID       uint   `gorm:"not null"`
    Status           string `gorm:"size:20;default:'queued'"`
    TriggerType      string `gorm:"size:20;not null"`
    TriggeredByUser  string `gorm:"size:255"`
    TriggerPayload   string `gorm:"type:json"`
    ConcurrencyGroup string `gorm:"size:255;index"`
    RerunFromID      *uint
    RerunFromStep    *string
    QueuedAt         time.Time
    StartedAt        *time.Time
    FinishedAt       *time.Time
    BoundNodeName    string `gorm:"size:255"`
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

**Changes:**
- Add `EnvironmentID` — every run executes in the context of a specific environment
- The environment provides `ClusterID` and `Namespace` at runtime
- Promotion = create a new PipelineRun with the same SnapshotID but next EnvironmentID

### 2.4 PipelineSecret Model — Add Pipeline Scope

```go
// BEFORE — scope: global, cluster, pipeline
// "cluster" scope used ScopeRef as cluster_id

// AFTER — scope: global, pipeline, environment
type PipelineSecret struct {
    ID          uint   `gorm:"primaryKey;autoIncrement"`
    Scope       string `gorm:"not null;size:20;index"`     // "global" | "pipeline" | "environment"
    ScopeRef    uint   `gorm:"default:0;index"`            // pipeline_id or environment_id
    Name        string `gorm:"not null;size:255"`
    ValueEnc    string `gorm:"-"`
    // ...
}
```

**Changes:**
- Replace `cluster` scope with `environment` scope — secrets scoped to a specific environment
- `ScopeRef` for `environment` scope = `environment_id`

---

## 3. API Route Changes

### 3.1 Before (Cluster-Scoped)

```
/api/v1/clusters/:clusterID/pipelines                         [GET, POST]
/api/v1/clusters/:clusterID/pipelines/:pipelineID             [GET, PUT, DELETE]
/api/v1/clusters/:clusterID/pipelines/:pipelineID/versions    [GET, POST]
/api/v1/clusters/:clusterID/pipelines/:pipelineID/runs        [GET, POST]
/api/v1/clusters/:clusterID/pipelines/:pipelineID/environments [GET, POST]
/api/v1/clusters/:clusterID/pipeline-secrets                  [GET, POST]
```

### 3.2 After (Top-Level)

```
# ── Pipeline CRUD (top-level) ──────────────────────────────────────────
/api/v1/pipelines                                              [GET, POST]
/api/v1/pipelines/:pipelineID                                  [GET, PUT, DELETE]

# ── Versions (immutable snapshots) ─────────────────────────────────────
/api/v1/pipelines/:pipelineID/versions                         [GET, POST]
/api/v1/pipelines/:pipelineID/versions/:version                [GET]

# ── Environments (cluster binding) ─────────────────────────────────────
/api/v1/pipelines/:pipelineID/environments                     [GET, POST]
/api/v1/pipelines/:pipelineID/environments/:envID              [GET, PUT, DELETE]

# ── Runs (always in context of an environment) ─────────────────────────
/api/v1/pipelines/:pipelineID/runs                             [GET]        # list all runs (filterable by env)
/api/v1/pipelines/:pipelineID/environments/:envID/runs         [POST]       # trigger run in specific env
/api/v1/pipelines/:pipelineID/runs/:runID                      [GET]
/api/v1/pipelines/:pipelineID/runs/:runID/cancel               [POST]
/api/v1/pipelines/:pipelineID/runs/:runID/rerun                [POST]
/api/v1/pipelines/:pipelineID/runs/:runID/promote              [POST]       # NEW: promote to next env

# ── Step operations ────────────────────────────────────────────────────
/api/v1/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/approve  [POST]
/api/v1/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/reject   [POST]
/api/v1/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/logs     [GET]

# ── Secrets ────────────────────────────────────────────────────────────
/api/v1/pipeline-secrets                                       [GET, POST]      # global
/api/v1/pipelines/:pipelineID/secrets                          [GET, POST]      # pipeline-scoped
/api/v1/pipelines/:pipelineID/environments/:envID/secrets      [GET, POST]      # env-scoped
/api/v1/pipeline-secrets/:secretID                             [GET, PUT, DELETE]

# ── Step type registry ─────────────────────────────────────────────────
/api/v1/pipeline-step-types                                    [GET]
```

### 3.3 Key API Design Decisions

| Decision | Rationale |
|----------|-----------|
| Trigger run under `environments/:envID/runs` | Makes it explicit which environment the run targets |
| List runs under `pipelines/:pipelineID/runs` | Cross-environment view for the pipeline owner |
| `POST .../runs/:runID/promote` | Creates a new run in the next environment with the same snapshot |
| Secrets at 3 levels | Global → Pipeline → Environment, resolved with cascading precedence |
| No `clusterID` in pipeline routes | Pipeline is cluster-agnostic; cluster is resolved via environment |

---

## 4. Router Registration

### 4.1 New File: `internal/router/routes_pipeline.go`

Replace `routes_cluster_pipeline.go` entirely.

```go
func registerPipelineRoutes(api *gin.RouterGroup, d *routeDeps) {
    // ── Pipeline CRUD ──
    pipelines := api.Group("/pipelines")
    pipelines.Use(middleware.AuthRequired(d.db))  // user must be authenticated
    {
        pipelines.GET("",    pipelineHandler.List)
        pipelines.POST("",   pipelineHandler.Create)

        single := pipelines.Group("/:pipelineID")
        single.Use(middleware.PipelineAccessRequired(d.db))  // NEW middleware
        {
            single.GET("",    pipelineHandler.Get)
            single.PUT("",    pipelineHandler.Update)
            single.DELETE("", pipelineHandler.Delete)

            // Versions
            versions := single.Group("/versions")
            {
                versions.GET("",          pipelineHandler.ListVersions)
                versions.POST("",         pipelineHandler.CreateVersion)
                versions.GET("/:version", pipelineHandler.GetVersion)
            }

            // Environments
            envs := single.Group("/environments")
            {
                envs.GET("",         envHandler.List)
                envs.POST("",        envHandler.Create)
                envs.GET("/:envID",  envHandler.Get)
                envs.PUT("/:envID",  envHandler.Update)
                envs.DELETE("/:envID", envHandler.Delete)

                // Trigger run in specific environment
                envs.POST("/:envID/runs", runHandler.TriggerRun)

                // Environment-scoped secrets
                envs.GET("/:envID/secrets",  secretHandler.ListByEnvironment)
                envs.POST("/:envID/secrets", secretHandler.CreateForEnvironment)
            }

            // Runs (cross-environment view)
            runs := single.Group("/runs")
            {
                runs.GET("",         runHandler.ListRuns)
                runs.GET("/:runID",  runHandler.GetRun)
                runs.POST("/:runID/cancel",  runHandler.CancelRun)
                runs.POST("/:runID/rerun",   runHandler.RerunPipeline)
                runs.POST("/:runID/promote", runHandler.PromoteRun)  // NEW

                // Step operations
                steps := runs.Group("/:runID/steps/:stepRunID")
                {
                    steps.POST("/approve", runHandler.ApproveStep)
                    steps.POST("/reject",  runHandler.RejectStep)
                    steps.GET("/logs",     logHandler.GetStepLogs)
                }
            }

            // Pipeline-scoped secrets
            single.GET("/secrets",  secretHandler.ListByPipeline)
            single.POST("/secrets", secretHandler.CreateForPipeline)
        }
    }

    // ── Global secrets ──
    globalSecrets := api.Group("/pipeline-secrets")
    globalSecrets.Use(middleware.AuthRequired(d.db))
    {
        globalSecrets.GET("",              secretHandler.ListGlobal)
        globalSecrets.POST("",             secretHandler.CreateGlobal)
        globalSecrets.GET("/:secretID",    secretHandler.Get)
        globalSecrets.PUT("/:secretID",    secretHandler.Update)
        globalSecrets.DELETE("/:secretID", secretHandler.Delete)
    }

    // ── Step type registry ──
    api.GET("/pipeline-step-types", runHandler.ListStepTypes)
}
```

### 4.2 New Middleware: `PipelineAccessRequired`

```go
// Checks if the authenticated user has access to this pipeline.
// Initially: any authenticated user can access any pipeline.
// Future: RBAC per pipeline (owner, contributor, viewer roles).
func PipelineAccessRequired(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        pipelineID, err := strconv.ParseUint(c.Param("pipelineID"), 10, 64)
        if err != nil {
            response.BadRequest(c, "invalid pipeline ID")
            c.Abort()
            return
        }
        // Verify pipeline exists
        var count int64
        db.Model(&models.Pipeline{}).Where("id = ?", pipelineID).Count(&count)
        if count == 0 {
            response.NotFound(c, "pipeline not found")
            c.Abort()
            return
        }
        c.Set("pipelineID", uint(pipelineID))
        c.Next()
    }
}
```

---

## 5. Handler Changes

### 5.1 PipelineHandler — Remove Cluster Resolution

```go
// BEFORE: handler resolves cluster first, then does pipeline CRUD
func (h *PipelineHandler) List(c *gin.Context) {
    clusterID, _ := parseClusterID(c.Param("clusterID"))  // ← REMOVE
    // ... list pipelines filtered by clusterID
}

// AFTER: handler does pipeline CRUD directly
func (h *PipelineHandler) List(c *gin.Context) {
    page, _     := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
    search      := c.Query("search")

    pipelines, total, err := h.pipelineService.ListPipelines(c.Request.Context(), ListPipelinesParams{
        Page:     page,
        PageSize: pageSize,
        Search:   search,
    })
    if err != nil {
        response.InternalError(c, "failed to list pipelines: "+err.Error())
        return
    }
    response.List(c, pipelines, total)
}
```

**Handler struct changes:**
```go
// BEFORE
type PipelineHandler struct {
    clusterService  *services.ClusterService
    k8sMgr          *k8s.ClusterInformerManager
    pipelineService *services.PipelineService
    auditService    *services.AuditService
    db              *gorm.DB
}

// AFTER — no more clusterService/k8sMgr for basic CRUD
type PipelineHandler struct {
    pipelineService *services.PipelineService
    auditService    *services.AuditService
    db              *gorm.DB
}
```

### 5.2 PipelineRunHandler — Cluster Resolved via Environment

```go
// TriggerRun now takes environmentID from URL
func (h *PipelineRunHandler) TriggerRun(c *gin.Context) {
    pipelineID, _ := parseID(c.Param("pipelineID"))
    envID, err := parseID(c.Param("envID"))
    if err != nil {
        response.BadRequest(c, "invalid environment ID")
        return
    }

    var req TriggerRunRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "invalid request: "+err.Error())
        return
    }

    // Environment provides the cluster + namespace
    env, err := h.envService.GetEnvironment(c.Request.Context(), envID)
    if err != nil {
        response.NotFound(c, "environment not found")
        return
    }
    if env.PipelineID != pipelineID {
        response.BadRequest(c, "environment does not belong to this pipeline")
        return
    }

    run, err := h.scheduler.TriggerRun(c.Request.Context(), TriggerRunInput{
        PipelineID:    pipelineID,
        EnvironmentID: envID,
        ClusterID:     env.ClusterID,
        Namespace:     env.Namespace,
        VersionID:     req.VersionID,
        TriggeredBy:   getCurrentUser(c),
        Variables:     mergeVariables(env.VariablesJSON, req.Variables),
    })
    // ...
}
```

### 5.3 NEW: PromoteRun Handler

```go
// PromoteRun creates a new run in the next environment using the same snapshot
func (h *PipelineRunHandler) PromoteRun(c *gin.Context) {
    pipelineID, _ := parseID(c.Param("pipelineID"))
    runID, _ := parseID(c.Param("runID"))

    // Get the current run
    run, err := h.pipelineService.GetPipelineRun(c.Request.Context(), runID)
    if err != nil {
        response.NotFound(c, "run not found")
        return
    }
    if run.Status != models.PipelineRunStatusSuccess {
        response.BadRequest(c, "can only promote successful runs")
        return
    }

    // Find the next environment
    currentEnv, _ := h.envService.GetEnvironment(c.Request.Context(), run.EnvironmentID)
    nextEnv, err := h.envService.GetNextEnvironment(c.Request.Context(), pipelineID, currentEnv.OrderIndex)
    if err != nil {
        response.BadRequest(c, "no next environment to promote to")
        return
    }

    // Check approval requirement
    if nextEnv.ApprovalRequired {
        // Record pending promotion, return 202 Accepted
        history := &models.PromotionHistory{
            PipelineID:      pipelineID,
            PipelineRunID:   runID,
            FromEnvironment: currentEnv.Name,
            ToEnvironment:   nextEnv.Name,
            Status:          "pending",
        }
        h.envService.RecordPromotion(c.Request.Context(), history)
        response.Accepted(c, gin.H{
            "message":      "promotion pending approval",
            "promotion_id": history.ID,
            "from":         currentEnv.Name,
            "to":           nextEnv.Name,
        })
        return
    }

    // Auto-promote: trigger new run in next environment
    newRun, err := h.scheduler.TriggerRun(c.Request.Context(), TriggerRunInput{
        PipelineID:    pipelineID,
        EnvironmentID: nextEnv.ID,
        ClusterID:     nextEnv.ClusterID,
        Namespace:     nextEnv.Namespace,
        SnapshotID:    run.SnapshotID,  // same version
        TriggeredBy:   getCurrentUser(c),
        TriggerType:   "promotion",
    })
    // ...
}
```

---

## 6. Service Layer Changes

### 6.1 PipelineService — Remove ClusterID Dependencies

```go
// BEFORE
type ListPipelinesParams struct {
    ClusterID uint
    Namespace string
    Search    string
    Page      int
    PageSize  int
}

// AFTER
type ListPipelinesParams struct {
    Search    string
    Page      int
    PageSize  int
    CreatedBy string  // optional: filter by creator
}

func (s *PipelineService) ListPipelines(ctx context.Context, params ListPipelinesParams) ([]PipelineInfo, int64, error) {
    query := s.db.WithContext(ctx).Model(&models.Pipeline{})

    if params.Search != "" {
        query = query.Where("name LIKE ? OR description LIKE ?",
            "%"+params.Search+"%", "%"+params.Search+"%")
    }
    if params.CreatedBy != "" {
        query = query.Where("created_by = ?", params.CreatedBy)
    }

    var total int64
    query.Count(&total)

    var pipelines []models.Pipeline
    query.Offset((params.Page - 1) * params.PageSize).
        Limit(params.PageSize).
        Order("updated_at DESC").
        Find(&pipelines)

    // Enrich with environment summary
    items := make([]PipelineInfo, 0, len(pipelines))
    for _, p := range pipelines {
        info := toPipelineInfo(&p)
        info.Environments = s.getEnvironmentSummary(ctx, p.ID)
        items = append(items, info)
    }
    return items, total, nil
}
```

### 6.2 PipelineScheduler — Resolve Cluster from Environment

```go
// BEFORE: scheduler receives clusterID directly from handler (from URL)
// AFTER: scheduler receives TriggerRunInput which includes env-derived cluster info

type TriggerRunInput struct {
    PipelineID    uint
    EnvironmentID uint
    ClusterID     uint     // resolved from environment
    Namespace     string   // resolved from environment
    SnapshotID    uint     // optional: specific version (0 = current)
    VersionID     *uint    // optional: version number
    TriggeredBy   string
    TriggerType   string   // manual, webhook, cron, promotion, rerun
    Variables     map[string]string
}
```

### 6.3 JobBuilder — Use Environment Context

```go
// Key change: JobBuilder gets cluster client from environment's ClusterID
func (b *JobBuilder) BuildJob(ctx context.Context, input JobBuildInput) (*batchv1.Job, error) {
    // Resolve cluster from environment
    cluster, err := b.clusterService.GetCluster(input.ClusterID)
    if err != nil {
        return nil, fmt.Errorf("resolve cluster for environment: %w", err)
    }
    k8sClient, err := b.k8sMgr.GetK8sClient(cluster)
    if err != nil {
        return nil, fmt.Errorf("get k8s client for cluster %d: %w", input.ClusterID, err)
    }

    // Build job in environment's namespace
    job := &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:      input.JobName,
            Namespace: input.Namespace,  // from environment
            // ...
        },
        // ...
    }
    return job, nil
}
```

### 6.4 EnvironmentService — Enrichments

```go
// New method: get environment with cluster info for display
func (s *EnvironmentService) GetEnvironmentWithCluster(ctx context.Context, id uint) (*EnvironmentInfo, error) {
    var env models.Environment
    if err := s.db.WithContext(ctx).First(&env, id).Error; err != nil {
        return nil, fmt.Errorf("get environment %d: %w", id, err)
    }

    // Fetch cluster name for display
    var cluster models.Cluster
    if err := s.db.WithContext(ctx).
        Select("id, name, status").
        First(&cluster, env.ClusterID).Error; err != nil {
        return nil, fmt.Errorf("get cluster %d for environment: %w", env.ClusterID, err)
    }

    return &EnvironmentInfo{
        ID:               env.ID,
        Name:             env.Name,
        ClusterID:        env.ClusterID,
        ClusterName:      cluster.Name,
        ClusterStatus:    cluster.Status,
        Namespace:        env.Namespace,
        OrderIndex:       env.OrderIndex,
        AutoPromote:      env.AutoPromote,
        ApprovalRequired: env.ApprovalRequired,
        // ...
    }, nil
}
```

---

## 7. Database Migration

### 7.1 Migration SQL

```sql
-- Step 1: Remove cluster binding from pipelines
ALTER TABLE pipelines DROP COLUMN cluster_id;
ALTER TABLE pipelines DROP COLUMN namespace;

-- Step 2: Update unique index (name only, globally unique)
DROP INDEX IF EXISTS idx_pipeline_name_cluster;
CREATE UNIQUE INDEX idx_pipeline_name ON pipelines(name) WHERE deleted_at IS NULL;

-- Step 3: Add environment_id to pipeline_runs
ALTER TABLE pipeline_runs ADD COLUMN environment_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_pipeline_runs_env ON pipeline_runs(environment_id);

-- Step 4: Add unique constraint on environments (pipeline_id, name)
CREATE UNIQUE INDEX idx_env_pipeline_name ON environments(pipeline_id, name) WHERE deleted_at IS NULL;

-- Step 5: Add variables column to environments
ALTER TABLE environments ADD COLUMN variables_json TEXT DEFAULT '{}';

-- Step 6: Update pipeline_secrets scope values
UPDATE pipeline_secrets SET scope = 'environment' WHERE scope = 'cluster';
```

### 7.2 Data Migration Strategy

Since we're still in development, the migration is straightforward:

1. For existing pipelines: create a default "dev" environment using the pipeline's current `ClusterID` + `Namespace`
2. For existing runs: set `EnvironmentID` to the default dev environment
3. Drop the old columns

```sql
-- Backfill: create default environment for each existing pipeline
INSERT INTO environments (name, pipeline_id, cluster_id, namespace, order_index, created_at, updated_at)
SELECT 'dev', p.id, p.cluster_id, p.namespace, 1, NOW(), NOW()
FROM pipelines p
WHERE p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM environments e WHERE e.pipeline_id = p.id);

-- Backfill: link existing runs to their pipeline's default environment
UPDATE pipeline_runs pr
SET environment_id = (
    SELECT e.id FROM environments e
    WHERE e.pipeline_id = pr.pipeline_id
    AND e.name = 'dev'
    LIMIT 1
)
WHERE pr.environment_id = 0;
```

---

## 8. Frontend Impact

### 8.1 API Service Layer

```typescript
// BEFORE
const pipelineApi = {
  list: (clusterId: number) => api.get(`/clusters/${clusterId}/pipelines`),
  create: (clusterId: number, data) => api.post(`/clusters/${clusterId}/pipelines`, data),
  triggerRun: (clusterId: number, pipelineId: number, data) =>
    api.post(`/clusters/${clusterId}/pipelines/${pipelineId}/runs`, data),
}

// AFTER
const pipelineApi = {
  list: (params?) => api.get('/pipelines', { params }),
  create: (data) => api.post('/pipelines', data),
  triggerRun: (pipelineId: number, envId: number, data) =>
    api.post(`/pipelines/${pipelineId}/environments/${envId}/runs`, data),
  promoteRun: (pipelineId: number, runId: number) =>
    api.post(`/pipelines/${pipelineId}/runs/${runId}/promote`),
}
```

### 8.2 UI Changes

| Component | Change |
|-----------|--------|
| `PipelineList` | Remove cluster selector from pipeline CRUD. Add environment summary column. |
| `PipelineEditor` | Remove namespace field from pipeline form. |
| `PipelineRunDetail` | Show environment name/cluster in run header. Add "Promote" button for successful runs. |
| `PipelineEnvironments` | Elevate from drawer to dedicated tab/page. Add cluster selector per environment. |
| Sidebar/Routes | Move Pipeline menu from under Cluster to top-level CI/CD section. |
| `TriggerRunModal` | Add environment selector (required). |

### 8.3 Route Changes

```typescript
// BEFORE: under cluster context
/clusters/:clusterId/pipelines
/clusters/:clusterId/pipelines/:pipelineId

// AFTER: top-level
/pipelines
/pipelines/:pipelineId
/pipelines/:pipelineId/environments
/pipelines/:pipelineId/runs/:runId
```

---

## 9. Execution Flow (Promotion Lifecycle)

```
Developer pushes code
        │
        ▼
┌─────────────────┐
│  Webhook fires   │
│  Pipeline-A      │
│  → env: dev      │
│  → Cluster-1     │
└────────┬────────┘
         │ build + deploy + smoke test
         ▼
    ✅ Success
         │
         ▼ (auto_promote=true)
┌─────────────────┐
│  New Run created │
│  Pipeline-A      │
│  → env: staging  │
│  → Cluster-2     │
└────────┬────────┘
         │ deploy + integration test
         ▼
    ✅ Success
         │
         ▼ (approval_required=true)
┌─────────────────┐
│  Pending approval│
│  Notify approvers│
└────────┬────────┘
         │ Team lead approves
         ▼
┌─────────────────┐
│  New Run created │
│  Pipeline-A      │
│  → env: prod     │
│  → Cluster-3     │
└────────┬────────┘
         │ deploy + smoke test
         ▼
    ✅ Production deployed
```

---

## 10. Files to Modify

### Backend (Go)

| File | Action | Description |
|------|--------|-------------|
| `internal/models/pipeline.go` | **MODIFY** | Remove `ClusterID`, `Namespace` from Pipeline. Add `EnvironmentID` to PipelineRun. |
| `internal/models/environment.go` | **MODIFY** | Add unique index, `VariablesJSON`. |
| `internal/models/pipeline_secret.go` | **MODIFY** | Change `cluster` scope to `environment`. |
| `internal/handlers/pipeline_handler.go` | **REWRITE** | Remove cluster resolution. Pipeline CRUD is cluster-free. |
| `internal/handlers/pipeline_run_handler.go` | **REWRITE** | TriggerRun takes envID. Add PromoteRun. Cluster resolved via env. |
| `internal/handlers/environment_handler.go` | **MODIFY** | Add Get endpoint. Remove cluster resolution (env provides it). |
| `internal/handlers/pipeline_secret_handler.go` | **MODIFY** | Adapt to new scope hierarchy. |
| `internal/services/pipeline_service.go` | **MODIFY** | Remove ClusterID from queries. Add environment enrichment. |
| `internal/services/pipeline_scheduler.go` | **MODIFY** | Accept `TriggerRunInput` with env-derived cluster info. |
| `internal/services/pipeline_job_builder.go` | **MODIFY** | Resolve cluster from environment context. |
| `internal/services/pipeline_job_watcher.go` | **MODIFY** | Watch jobs across multiple clusters (env-aware). |
| `internal/services/environment_service.go` | **MODIFY** | Add `GetEnvironmentWithCluster`, promotion logic. |
| `internal/router/routes_cluster_pipeline.go` | **DELETE** | Replaced by new file. |
| `internal/router/routes_pipeline.go` | **CREATE** | New top-level pipeline routes. |
| `internal/middleware/pipeline_access.go` | **CREATE** | `PipelineAccessRequired` middleware. |
| `internal/database/migrations.go` | **MODIFY** | Add migration for schema changes. |

### Frontend (TypeScript/React)

| File | Action | Description |
|------|--------|-------------|
| `ui/src/services/pipelineService.ts` | **REWRITE** | Remove clusterId from all API paths. |
| `ui/src/services/environmentService.ts` | **MODIFY** | Update API paths. |
| `ui/src/pages/pipeline/PipelineList.tsx` | **MODIFY** | Remove cluster context. Add env summary. |
| `ui/src/pages/pipeline/PipelineEditor.tsx` | **MODIFY** | Remove namespace from pipeline form. |
| `ui/src/pages/pipeline/PipelineRunDetail.tsx` | **MODIFY** | Show env info. Add promote button. |
| `ui/src/pages/pipeline/components/PipelineEnvironments.tsx` | **MODIFY** | Add cluster selector per env. |
| Route config (`routes.tsx`) | **MODIFY** | Move pipeline routes to top-level. |
| Sidebar config | **MODIFY** | CI/CD as top-level menu. |

---

## 11. Priority Matrix & Implementation Order

### 11.1 Evaluation Dimensions

| Dimension | Weight | Scale | Description |
|-----------|--------|-------|-------------|
| **Impact** | 30% | 1-5 | Value delivered: 5 = unblocks everything, 1 = cosmetic |
| **Dependency** | 30% | 1-5 | Blocking factor: 5 = blocks 5+ items, 1 = blocks nothing |
| **Risk** | 20% | 1-5 (inverse) | 5 = low risk/simple, 1 = high risk/complex |
| **Effort** | 20% | 1-5 (inverse) | 5 = < 30 LOC, 1 = 500+ LOC refactor |

**Composite Score** = Impact×0.3 + Dependency×0.3 + Risk×0.2 + Effort×0.2 — Higher = do first.

### 11.2 Work Items — Layer 0: Data Foundation (Models + Migration)

| ID | Work Item | Impact | Dep | Risk | Effort | **Score** | Blocks |
|----|-----------|--------|-----|------|--------|-----------|--------|
| **M1** | Pipeline model: remove `ClusterID`, `Namespace` | 5 | 5 | 4 | 4 | **4.6** | M2,S1-S4,H1-H4,F1-F4 |
| **M2** | PipelineRun model: add `EnvironmentID` | 5 | 5 | 4 | 5 | **4.8** | S2,S3,H2,H3,F3 |
| **M3** | Environment model: add unique index + `VariablesJSON` | 4 | 4 | 5 | 5 | **4.4** | S5,H4 |
| **M4** | PipelineSecret model: `cluster` scope → `environment` scope | 3 | 2 | 5 | 5 | **3.5** | H5 |
| **M5** | Database migration SQL + data backfill | 5 | 5 | 3 | 3 | **4.2** | ALL |

### 11.3 Work Items — Layer 1: Backend Services

| ID | Work Item | Impact | Dep | Risk | Effort | **Score** | Blocks | Blocked By |
|----|-----------|--------|-----|------|--------|-----------|--------|------------|
| **S1** | PipelineService: remove ClusterID from queries | 4 | 4 | 4 | 4 | **4.0** | H1 | M1 |
| **S2** | PipelineScheduler: accept `TriggerRunInput` (env-based) | 5 | 5 | 2 | 1 | **3.6** | H2,H3 | M1,M2 |
| **S3** | JobBuilder: resolve cluster from env context | 4 | 4 | 3 | 3 | **3.6** | H2 | M2 |
| **S4** | JobWatcher: ClusterID from PipelineRun.EnvironmentID→env.ClusterID | 4 | 3 | 2 | 2 | **2.9** | — | M1,M2 |
| **S5** | EnvironmentService: add `GetEnvironmentWithCluster`, promotion helpers | 4 | 4 | 4 | 3 | **3.8** | H3,H4 | M3 |
| **S6** | Webhook trigger: resolve env from pipeline (default lowest OrderIndex) | 3 | 2 | 3 | 4 | **2.9** | — | S2 |

### 11.4 Work Items — Layer 2: Backend API (Handlers + Router + Middleware)

| ID | Work Item | Impact | Dep | Risk | Effort | **Score** | Blocks | Blocked By |
|----|-----------|--------|-----|------|--------|-----------|--------|------------|
| **H1** | PipelineHandler: rewrite (cluster-free CRUD) | 4 | 3 | 4 | 3 | **3.5** | F1,F2 | S1 |
| **H2** | PipelineRunHandler: env-based TriggerRun + cluster resolution | 5 | 4 | 3 | 2 | **3.7** | F3 | S2,S3 |
| **H3** | PipelineRunHandler: new `PromoteRun` endpoint | 5 | 3 | 3 | 3 | **3.6** | F3 | S2,S5 |
| **H4** | EnvironmentHandler: add `Get` endpoint, remove cluster resolution | 3 | 2 | 4 | 4 | **3.1** | F4 | S5 |
| **H5** | PipelineSecretHandler: adapt to 3-level scope hierarchy | 2 | 1 | 4 | 3 | **2.3** | F5 | M4 |
| **R1** | `routes_pipeline.go`: new top-level route file | 4 | 4 | 4 | 3 | **3.8** | F1-F5 | H1-H5 |
| **R2** | `PipelineAccessRequired` middleware | 3 | 3 | 5 | 5 | **3.8** | R1 | — |
| **R3** | Delete `routes_cluster_pipeline.go` | 2 | 1 | 5 | 5 | **2.9** | — | R1 |

### 11.5 Work Items — Layer 3: Frontend

| ID | Work Item | Impact | Dep | Risk | Effort | **Score** | Blocks | Blocked By |
|----|-----------|--------|-----|------|--------|-----------|--------|------------|
| **F1** | `pipelineService.ts`: remove clusterId from all API paths | 4 | 4 | 4 | 4 | **4.0** | F2-F5 | R1 |
| **F2** | `PipelineList.tsx`: remove cluster context, add env summary column | 4 | 2 | 3 | 3 | **3.0** | — | F1,H1 |
| **F3** | `PipelineRunDetail.tsx`: show env info, add Promote button | 5 | 2 | 3 | 3 | **3.3** | — | F1,H2,H3 |
| **F4** | `PipelineEnvironments.tsx`: cluster selector per env | 3 | 1 | 4 | 4 | **2.8** | — | F1,H4 |
| **F5** | Routes & sidebar: move Pipeline to top-level CI/CD | 3 | 2 | 4 | 4 | **3.1** | — | F1 |
| **F6** | `environmentService.ts`: update API paths | 3 | 2 | 5 | 5 | **3.5** | F4 | R1 |
| **F7** | `PipelineEditor.tsx`: remove namespace field | 2 | 1 | 5 | 5 | **2.9** | — | F1 |

### 11.6 Dependency Graph (DAG)

```
                    ┌─────────────────────────────────────┐
                    │       Layer 0: Data Foundation       │
                    │                                     │
                    │  M5 (Migration SQL)                 │
                    │   ├── M1 (Pipeline: drop ClusterID) │
                    │   ├── M2 (Run: add EnvironmentID)   │
                    │   ├── M3 (Env: unique idx + vars)   │
                    │   └── M4 (Secret: scope change)     │
                    └──────────┬──────────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
     ┌────────────┐   ┌──────────────┐   ┌──────────┐
     │ S1 Pipeline │   │ S2 Scheduler │   │ S5 EnvSvc│
     │    Svc      │   │ S3 JobBuilder│   │          │
     └──────┬─────┘   │ S4 JobWatch  │   └────┬─────┘
            │         │ S6 Webhook   │        │
            ▼         └──────┬───────┘        ▼
     ┌──────────┐            │          ┌──────────┐
     │ H1 Pipe  │            ▼          │ H4 Env   │
     │  Handler │     ┌──────────┐      │  Handler │
     └──────┬───┘     │ H2 Run   │      └────┬─────┘
            │         │  Trigger │            │
            │         │ H3 Prom  │            │
            │         └────┬─────┘            │
            │              │                  │
            ▼              ▼                  ▼
     ┌─────────────────────────────────────────────┐
     │  R2 Middleware → R1 routes_pipeline.go       │
     │                  R3 delete old routes         │
     └──────────────────────┬──────────────────────┘
                            │
              ┌─────────────┼──────────────┐
              ▼             ▼              ▼
     ┌──────────┐   ┌──────────┐   ┌──────────┐
     │ F1 API   │   │ F6 Env   │   │ F5 Route │
     │  Service │   │  Service │   │ +Sidebar │
     └────┬─────┘   └────┬─────┘   └──────────┘
          │               │
    ┌─────┼─────┐         ▼
    ▼     ▼     ▼    ┌──────────┐
  F2    F3    F7     │ F4 Env   │
  List  Run   Editor │  Page    │
              Detail └──────────┘
```

### 11.7 Execution Plan: 6 Sprints

#### Sprint 1 — Data Foundation (blocks everything)

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 1.1 | M1 | Pipeline model: remove `ClusterID`, `Namespace` | ~30 |
| 1.2 | M2 | PipelineRun model: add `EnvironmentID` | ~10 |
| 1.3 | M3 | Environment model: unique index + `VariablesJSON` | ~15 |
| 1.4 | M4 | PipelineSecret model: scope change | ~10 |
| 1.5 | M5 | Migration SQL + backfill script | ~50 |

**Gate**: `go build ./...` passes with model changes. Migration idempotent.

#### Sprint 2 — Core Services (unblocks handlers)

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 2.1 | S1 | PipelineService: remove ClusterID queries | ~80 |
| 2.2 | S5 | EnvironmentService: enrichment + promotion helpers | ~120 |
| 2.3 | R2 | `PipelineAccessRequired` middleware | ~40 |

**Gate**: `go build ./...` passes. Service unit tests pass.

#### Sprint 3 — Execution Engine (highest risk)

| Order | ID | Work Item | Est. LOC | Risk Notes |
|-------|----|-----------|----------|------------|
| 3.1 | S2 | PipelineScheduler: `TriggerRunInput` refactor | ~200 | 10+ ClusterID refs, concurrency logic |
| 3.2 | S3 | JobBuilder: cluster from env context | ~60 | K8s client resolution path |
| 3.3 | S4 | JobWatcher: env-aware cluster lookup | ~80 | 6+ ClusterID refs, polling logic |
| 3.4 | S6 | Webhook trigger: default env resolution | ~40 | Must handle "no env" gracefully |

**Gate**: Can trigger a run via scheduler with env-derived cluster. Watcher picks up job status.

> **This is the riskiest sprint.** Scheduler alone has 10+ ClusterID references across
> concurrency counting, K8s client lookups, and job submission. Plan extra review time.

#### Sprint 4 — API Layer (connects everything)

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 4.1 | H1 | PipelineHandler: cluster-free CRUD | ~150 |
| 4.2 | H2 | PipelineRunHandler: env-based trigger | ~180 |
| 4.3 | H3 | PipelineRunHandler: `PromoteRun` | ~80 |
| 4.4 | H4 | EnvironmentHandler: add Get, remove cluster res. | ~60 |
| 4.5 | H5 | PipelineSecretHandler: 3-level scope | ~60 |
| 4.6 | R1 | `routes_pipeline.go`: new top-level routes | ~90 |
| 4.7 | R3 | Delete `routes_cluster_pipeline.go` | ~5 |

**Gate**: Manual API test — create pipeline → add env → trigger run → promote.

#### Sprint 5 — Frontend Core

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 5.1 | F1 | `pipelineService.ts`: remove clusterId | ~80 |
| 5.2 | F6 | `environmentService.ts`: update paths | ~30 |
| 5.3 | F5 | Routes & sidebar: top-level CI/CD | ~40 |
| 5.4 | F2 | `PipelineList.tsx`: env summary column | ~100 |

**Gate**: Navigate to `/pipelines`, see list with environment badges.

#### Sprint 6 — Frontend Polish

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 6.1 | F3 | `PipelineRunDetail.tsx`: env info + Promote button | ~120 |
| 6.2 | F4 | `PipelineEnvironments.tsx`: cluster selector | ~80 |
| 6.3 | F7 | `PipelineEditor.tsx`: remove namespace field | ~20 |

**Gate**: End-to-end walkthrough in browser.

### 11.8 Critical Path

```
M1 → M2 → M5 → S2 → H2 → R1 → F1 → F3
 8 items, estimated ~750 LOC on critical path
```

Any delay on these items delays the entire project. **Sprint 3 (S2 Scheduler)** is the
bottleneck — it's the most complex single item (~200 LOC, 10+ reference points, concurrency logic).

### 11.9 Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scheduler refactor breaks concurrency logic | Medium | High | Write concurrency unit tests BEFORE refactoring. Keep cluster-level concurrency counting but source ClusterID from env. |
| JobWatcher loses track of jobs during migration | Low | High | Backfill ensures every existing run has an EnvironmentID. Watcher still uses `run.ClusterID` path — just the source changes (via env lookup). |
| Frontend `useClusterId()` hook deeply embedded | Medium | Medium | Search all `useClusterId` / `useParams` references in pipeline pages. May need a `usePipelineContext` replacement hook. |
| Webhook breaks for existing integrations | Medium | High | Keep old webhook URL working as deprecated alias for one release cycle. Log warnings on old-path hits. |
| Pipeline name uniqueness conflict on migration | Low | Low | Pre-check: `SELECT name, COUNT(*) FROM pipelines GROUP BY name HAVING COUNT(*) > 1`. Rename duplicates before migration. |

### 11.10 Summary

```
Sprint 1  ██████████  Models + Migration       → Foundation, blocks everything
Sprint 2  ████████    Core Services             → Unblocks handlers
Sprint 3  ██████████  Execution Engine           → Highest risk, needs most review
Sprint 4  ████████    API Handlers + Routes      → Connects backend
Sprint 5  ██████      Frontend Core              → Makes it usable
Sprint 6  ████        Frontend Polish            → Completes the experience
```

**Total estimated**: ~1,800 LOC changed across 25 work items.
**Rule**: Never start a sprint until the previous sprint's gate passes.
Sprint 3 deserves a dedicated review checkpoint before proceeding to Sprint 4.

---

## 12. Open Questions

1. **Pipeline RBAC**: Should we add per-pipeline roles (owner/contributor/viewer) now, or defer?
   - Recommendation: Defer. Start with "any authenticated user can access any pipeline." Add RBAC later.

2. **Cross-cluster secret injection**: When a pipeline runs in Cluster-3 but the secret was created in global scope, how does the JobBuilder inject it?
   - Answer: Secrets are stored in Synapse DB (encrypted). JobBuilder reads them and injects as environment variables or volume mounts into the K8s Job. The secret never exists as a K8s Secret — it's injected at job creation time.

3. **Webhook routing**: Webhooks currently target `/clusters/:clusterID/webhooks/...`. Should they move to `/pipelines/:pipelineID/webhooks/...`?
   - Recommendation: Yes. Webhooks trigger pipelines, not clusters. The environment selection for webhook-triggered runs could default to the lowest `order_index` environment.

4. **Concurrency across environments**: If Pipeline-A has `max_concurrent_runs=1`, does that apply globally or per-environment?
   - Recommendation: Per-pipeline globally. A pipeline with max=1 can only have one active run across all environments. This prevents conflicting deployments.

---

## Appendix A: DTO Shapes

### PipelineInfo (API Response)

```json
{
  "id": 1,
  "name": "frontend-app",
  "description": "Build and deploy frontend application",
  "current_version_id": 5,
  "concurrency_policy": "queue",
  "max_concurrent_runs": 1,
  "environments": [
    {
      "id": 10,
      "name": "dev",
      "cluster_id": 1,
      "cluster_name": "dev-cluster",
      "cluster_status": "healthy",
      "namespace": "frontend-dev",
      "order_index": 1,
      "auto_promote": true,
      "last_run_status": "success"
    },
    {
      "id": 11,
      "name": "staging",
      "cluster_id": 2,
      "cluster_name": "staging-cluster",
      "cluster_status": "healthy",
      "namespace": "frontend-staging",
      "order_index": 2,
      "auto_promote": false,
      "approval_required": true,
      "last_run_status": "success"
    },
    {
      "id": 12,
      "name": "prod",
      "cluster_id": 3,
      "cluster_name": "prod-cluster",
      "cluster_status": "healthy",
      "namespace": "frontend-prod",
      "order_index": 3,
      "approval_required": true,
      "last_run_status": "running"
    }
  ],
  "created_by": "john",
  "created_at": "2026-04-15T10:00:00Z",
  "updated_at": "2026-04-15T14:30:00Z"
}
```

### TriggerRunRequest

```json
{
  "version_id": 5,
  "variables": {
    "IMAGE_TAG": "v1.2.3",
    "REPLICAS": "3"
  }
}
```

### PipelineRunInfo (API Response)

```json
{
  "id": 42,
  "pipeline_id": 1,
  "pipeline_name": "frontend-app",
  "environment_id": 10,
  "environment_name": "dev",
  "cluster_name": "dev-cluster",
  "namespace": "frontend-dev",
  "snapshot_id": 5,
  "version": 5,
  "status": "running",
  "trigger_type": "manual",
  "triggered_by": "john",
  "queued_at": "2026-04-15T14:30:00Z",
  "started_at": "2026-04-15T14:30:05Z",
  "steps": [
    { "name": "build", "status": "success", "duration": "45s" },
    { "name": "test",  "status": "running", "duration": "..." },
    { "name": "deploy","status": "pending" }
  ],
  "promotable": false
}
```

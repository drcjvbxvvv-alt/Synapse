# CI/CD Pipeline Architecture: Top-Level Pipeline + Environment-per-Cluster

> Version: v2.0 | Status: **Sprint 1 + Frontend UI Complete — In Progress**
> Last Updated: 2026-04-15

---

## 1. Architecture Overview

Pipeline is a **top-level entity** — it defines *what* to build and deploy, with no cluster binding.
Environment is the **execution target** — it defines *where* to deploy (which cluster + namespace).

```
Pipeline-A (top-level, globally unique name)
  ├── Environment: dev     → Cluster-1 / namespace: app-dev     (order_index: 1)
  ├── Environment: staging → Cluster-2 / namespace: app-staging  (order_index: 2)
  └── Environment: prod    → Cluster-3 / namespace: app-prod     (order_index: 3)

Pipeline-B
  ├── Environment: dev     → Cluster-1 / namespace: svc-b-dev
  └── Environment: prod    → Cluster-1 / namespace: svc-b-prod   ← same cluster, different NS
```

**Key design principles:**

- A `Pipeline` holds the step definitions and trigger rules — not a cluster assignment
- An `Environment` is the sole place that maps to a `ClusterID` + `Namespace`
- Every `PipelineRun` targets exactly one `Environment`
- Promotion = trigger the same pipeline `SnapshotID` in the next `Environment`
- Secret resolution order: **pipeline scope → environment scope → global scope**

---

## 2. Data Models

### 2.1 Pipeline

```go
type Pipeline struct {
    ID                uint           `json:"id" gorm:"primaryKey"`
    Name              string         `json:"name" gorm:"not null;size:255;uniqueIndex:idx_pipeline_name"`
    Description       string         `json:"description" gorm:"type:text"`
    CurrentVersionID  *uint          `json:"current_version_id"`
    ConcurrencyGroup  string         `json:"concurrency_group,omitempty" gorm:"size:255"`
    ConcurrencyPolicy string         `json:"concurrency_policy" gorm:"size:30;default:'cancel_previous'"`
    MaxConcurrentRuns int            `json:"max_concurrent_runs" gorm:"default:1"`
    NotifyOnSuccess   string         `json:"notify_on_success,omitempty" gorm:"type:jsonb"`
    NotifyOnFailure   string         `json:"notify_on_failure,omitempty" gorm:"type:jsonb"`
    NotifyOnScan      string         `json:"notify_on_scan,omitempty" gorm:"type:jsonb"`
    CreatedBy         uint           `json:"created_by" gorm:"not null"`
    CreatedAt         time.Time      `json:"created_at"`
    UpdatedAt         time.Time      `json:"updated_at"`
    DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}
```

- Name is **globally unique** (no cluster context needed)
- No `ClusterID`, no `Namespace`

### 2.2 Environment

```go
type Environment struct {
    ID                uint           `json:"id" gorm:"primaryKey"`
    Name              string         `json:"name" gorm:"not null;size:100;uniqueIndex:uq_pipeline_env"`
    PipelineID        uint           `json:"pipeline_id" gorm:"not null;uniqueIndex:uq_pipeline_env"`
    ClusterID         uint           `json:"cluster_id" gorm:"not null;index"`
    Namespace         string         `json:"namespace" gorm:"not null;size:253"`
    OrderIndex        int            `json:"order_index" gorm:"not null;index:idx_env_order"`
    AutoPromote       bool           `json:"auto_promote" gorm:"default:false"`
    ApprovalRequired  bool           `json:"approval_required" gorm:"default:false"`
    ApproverIDs       string         `json:"approver_ids,omitempty" gorm:"type:text"` // JSON array of user IDs
    SmokeTestStepName string         `json:"smoke_test_step_name,omitempty" gorm:"size:255"`
    NotifyChannelIDs  string         `json:"notify_channel_ids,omitempty" gorm:"type:text"` // JSON array
    VariablesJSON     string         `json:"variables_json,omitempty" gorm:"type:text"`     // env-specific overrides
    CreatedAt         time.Time      `json:"created_at"`
    UpdatedAt         time.Time      `json:"updated_at"`
    DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}
```

- Unique constraint: `(pipeline_id, name)` — no duplicate env names per pipeline
- `ClusterID` + `Namespace` — the sole place that maps a pipeline to a cluster
- `VariablesJSON` — env-specific variable overrides (replicas, feature flags, image tags)
- `OrderIndex` — determines promotion order (dev=1 → staging=2 → prod=3)

### 2.3 PipelineRun

```go
type PipelineRun struct {
    ID               uint           `json:"id" gorm:"primaryKey"`
    PipelineID       uint           `json:"pipeline_id" gorm:"not null;index"`
    EnvironmentID    uint           `json:"environment_id" gorm:"not null;index"` // execution target
    SnapshotID       uint           `json:"snapshot_id" gorm:"not null;index"`
    ClusterID        uint           `json:"cluster_id" gorm:"not null;index"`     // denormalized from Environment
    Namespace        string         `json:"namespace" gorm:"not null;size:253"`   // denormalized from Environment
    Status           string         `json:"status" gorm:"size:20;default:'queued'"`
    TriggerType      string         `json:"trigger_type" gorm:"size:20;not null"` // manual/webhook/cron/promotion/rerun
    TriggerPayload   string         `json:"trigger_payload,omitempty" gorm:"type:text"`
    TriggeredByUser  uint           `json:"triggered_by_user" gorm:"not null"`
    ConcurrencyGroup string         `json:"concurrency_group,omitempty" gorm:"size:255;index"`
    RerunFromID      *uint          `json:"rerun_from_id,omitempty"`
    Error            string         `json:"error,omitempty" gorm:"type:text"`
    QueuedAt         time.Time      `json:"queued_at"`
    StartedAt        *time.Time     `json:"started_at,omitempty"`
    FinishedAt       *time.Time     `json:"finished_at,omitempty"`
    BoundNodeName    string         `json:"bound_node_name,omitempty" gorm:"size:255"`
    CreatedAt        time.Time      `json:"created_at"`
    UpdatedAt        time.Time      `json:"updated_at"`
    DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}
```

- `EnvironmentID` — source of truth for deployment target
- `ClusterID` / `Namespace` — denormalized cache for Scheduler/JobBuilder/JobWatcher compatibility

### 2.4 PipelineSecret

```go
type PipelineSecret struct {
    ID          uint           `json:"id" gorm:"primaryKey"`
    Scope       string         `json:"scope" gorm:"not null;size:20;uniqueIndex:uq_scope_name"` // global / environment / pipeline
    ScopeRef    *uint          `json:"scope_ref" gorm:"uniqueIndex:uq_scope_name"`              // environment_id or pipeline_id
    Name        string         `json:"name" gorm:"not null;size:100;uniqueIndex:uq_scope_name"`
    ValueEnc    string         `json:"-" gorm:"type:text;not null"` // AES-256-GCM encrypted
    Description string         `json:"description" gorm:"size:255"`
    CreatedBy   uint           `json:"created_by" gorm:"not null"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
```

Secret resolution order (highest → lowest priority):

```
pipeline scope (scope_ref = pipeline_id)
    → environment scope (scope_ref = environment_id)
        → global scope (scope_ref = NULL)
```

---

## 3. API Routes

### 3.1 Pipeline Routes (top-level)

```
# Pipeline CRUD
GET    /api/v1/pipelines                                              List pipelines
POST   /api/v1/pipelines                                              Create pipeline
GET    /api/v1/pipelines/:pipelineID                                  Get pipeline
PUT    /api/v1/pipelines/:pipelineID                                  Update pipeline
DELETE /api/v1/pipelines/:pipelineID                                  Delete pipeline

# Versions (immutable snapshots)
GET    /api/v1/pipelines/:pipelineID/versions                         List versions
POST   /api/v1/pipelines/:pipelineID/versions                         Create version
GET    /api/v1/pipelines/:pipelineID/versions/:version                Get version

# Environments (cluster binding)
GET    /api/v1/pipelines/:pipelineID/environments                     List environments
POST   /api/v1/pipelines/:pipelineID/environments                     Create environment
GET    /api/v1/pipelines/:pipelineID/environments/:envID              Get environment
PUT    /api/v1/pipelines/:pipelineID/environments/:envID              Update environment
DELETE /api/v1/pipelines/:pipelineID/environments/:envID              Delete environment

# Runs
POST   /api/v1/pipelines/:pipelineID/environments/:envID/runs         Trigger run in specific env
GET    /api/v1/pipelines/:pipelineID/runs                             List runs (filterable by env)
GET    /api/v1/pipelines/:pipelineID/runs/:runID                      Get run
POST   /api/v1/pipelines/:pipelineID/runs/:runID/cancel               Cancel run
POST   /api/v1/pipelines/:pipelineID/runs/:runID/rerun                Rerun
POST   /api/v1/pipelines/:pipelineID/runs/:runID/promote              Promote to next environment

# Step operations
POST   /api/v1/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/approve
POST   /api/v1/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/reject
GET    /api/v1/pipelines/:pipelineID/runs/:runID/steps/:stepRunID/logs

# Secrets (3-level hierarchy)
GET    /api/v1/pipeline-secrets                                       Global secrets
POST   /api/v1/pipeline-secrets                                       Create global secret
GET    /api/v1/pipeline-secrets/:secretID                             Get secret
PUT    /api/v1/pipeline-secrets/:secretID                             Update secret
DELETE /api/v1/pipeline-secrets/:secretID                             Delete secret
GET    /api/v1/pipelines/:pipelineID/secrets                          Pipeline-scoped secrets
POST   /api/v1/pipelines/:pipelineID/secrets                          Create pipeline-scoped secret
GET    /api/v1/pipelines/:pipelineID/environments/:envID/secrets      Env-scoped secrets
POST   /api/v1/pipelines/:pipelineID/environments/:envID/secrets      Create env-scoped secret

# Step type registry
GET    /api/v1/pipeline-step-types                                    List available step types
```

### 3.2 Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Trigger run under `environments/:envID/runs` | Makes it explicit which environment the run targets |
| List runs under `pipelines/:pipelineID/runs` | Cross-environment view for the pipeline owner |
| `POST .../runs/:runID/promote` | Creates a new run in the next environment with the same snapshot |
| Secrets at 3 levels | Global → Environment → Pipeline, resolved with cascading precedence |
| No `clusterID` in pipeline routes | Pipeline is cluster-agnostic; cluster is resolved via environment |

---

## 4. Router Registration

### `internal/router/routes_pipeline.go` (new file, replaces `routes_cluster_pipeline.go`)

```go
func registerPipelineRoutes(api *gin.RouterGroup, d *routeDeps) {
    pipelines := api.Group("/pipelines")
    pipelines.Use(middleware.AuthRequired(d.db))
    {
        pipelines.GET("",  pipelineHandler.List)
        pipelines.POST("", pipelineHandler.Create)

        single := pipelines.Group("/:pipelineID")
        single.Use(middleware.PipelineAccessRequired(d.db))
        {
            single.GET("",    pipelineHandler.Get)
            single.PUT("",    pipelineHandler.Update)
            single.DELETE("", pipelineHandler.Delete)

            versions := single.Group("/versions")
            {
                versions.GET("",          pipelineHandler.ListVersions)
                versions.POST("",         pipelineHandler.CreateVersion)
                versions.GET("/:version", pipelineHandler.GetVersion)
            }

            envs := single.Group("/environments")
            {
                envs.GET("",           envHandler.List)
                envs.POST("",          envHandler.Create)
                envs.GET("/:envID",    envHandler.Get)
                envs.PUT("/:envID",    envHandler.Update)
                envs.DELETE("/:envID", envHandler.Delete)
                envs.POST("/:envID/runs", runHandler.TriggerRun)
                envs.GET("/:envID/secrets",  secretHandler.ListByEnvironment)
                envs.POST("/:envID/secrets", secretHandler.CreateForEnvironment)
            }

            runs := single.Group("/runs")
            {
                runs.GET("",                              runHandler.ListRuns)
                runs.GET("/:runID",                       runHandler.GetRun)
                runs.POST("/:runID/cancel",               runHandler.CancelRun)
                runs.POST("/:runID/rerun",                runHandler.RerunPipeline)
                runs.POST("/:runID/promote",              runHandler.PromoteRun)
                steps := runs.Group("/:runID/steps/:stepRunID")
                {
                    steps.POST("/approve", runHandler.ApproveStep)
                    steps.POST("/reject",  runHandler.RejectStep)
                    steps.GET("/logs",     logHandler.GetStepLogs)
                }
            }

            single.GET("/secrets",  secretHandler.ListByPipeline)
            single.POST("/secrets", secretHandler.CreateForPipeline)
        }
    }

    globalSecrets := api.Group("/pipeline-secrets")
    globalSecrets.Use(middleware.AuthRequired(d.db))
    {
        globalSecrets.GET("",              secretHandler.ListGlobal)
        globalSecrets.POST("",             secretHandler.CreateGlobal)
        globalSecrets.GET("/:secretID",    secretHandler.Get)
        globalSecrets.PUT("/:secretID",    secretHandler.Update)
        globalSecrets.DELETE("/:secretID", secretHandler.Delete)
    }

    api.GET("/pipeline-step-types", runHandler.ListStepTypes)
}
```

### `PipelineAccessRequired` middleware

```go
// Verifies pipeline exists and sets "pipelineID" in context.
// Future: extend to per-pipeline RBAC (owner/contributor/viewer).
func PipelineAccessRequired(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        pipelineID, err := strconv.ParseUint(c.Param("pipelineID"), 10, 64)
        if err != nil {
            response.BadRequest(c, "invalid pipeline ID")
            c.Abort()
            return
        }
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

## 5. Execution Flow (Promotion Lifecycle)

```
Developer pushes code
        │
        ▼
┌─────────────────┐
│  Webhook fires   │
│  Pipeline-A      │
│  → env: dev      │   (lowest order_index, auto-resolved)
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
│  → env: staging  │   (same SnapshotID, next order_index)
│  → Cluster-2     │
└────────┬────────┘
         │ deploy + integration test
         ▼
    ✅ Success
         │
         ▼ (approval_required=true)
┌─────────────────┐
│ Pending approval │
│ Notify approvers │
└────────┬────────┘
         │ Team lead approves
         ▼
┌─────────────────┐
│  New Run created │
│  Pipeline-A      │
│  → env: prod     │   (same SnapshotID, final order_index)
│  → Cluster-3     │
└────────┬────────┘
         │ deploy + smoke test
         ▼
    ✅ Production deployed
```

---

## 6. Database Schema

Managed by versioned SQL migrations in `internal/database/migrations/postgres/`.

### Key tables

| Table | File | Notes |
|-------|------|-------|
| `pipelines` | `008_pipeline.up.sql` | No `cluster_id`, no `namespace`. Globally unique `name`. |
| `pipeline_runs` | `008_pipeline.up.sql` | Has `environment_id`, `cluster_id` (denormalized), `namespace` (denormalized). |
| `pipeline_secrets` | `008_pipeline.up.sql` | Scope: `global` / `environment` / `pipeline`. |
| `environments` | `009_registry_environment.up.sql` | Has `notify_channel_ids`, `variables_json`. Unique `(pipeline_id, name)`. |
| `promotion_history` | `009_registry_environment.up.sql` | Records all promotion events. |

### `pipelines` (relevant columns)

```sql
CREATE TABLE IF NOT EXISTS pipelines (
    id                  BIGSERIAL PRIMARY KEY,
    name                VARCHAR(255)  NOT NULL,
    -- no cluster_id, no namespace
    current_version_id  BIGINT,
    concurrency_policy  VARCHAR(30)   DEFAULT 'cancel_previous',
    max_concurrent_runs INT           DEFAULT 1,
    created_by          BIGINT        NOT NULL,
    ...
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_name ON pipelines (name) WHERE deleted_at IS NULL;
```

### `environments` (relevant columns)

```sql
CREATE TABLE IF NOT EXISTS environments (
    id                   BIGSERIAL PRIMARY KEY,
    name                 VARCHAR(255)  NOT NULL,
    pipeline_id          BIGINT        NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    cluster_id           BIGINT        NOT NULL,    -- target cluster
    namespace            VARCHAR(253)  NOT NULL,    -- target namespace
    order_index          INT           NOT NULL,    -- promotion order
    auto_promote         BOOLEAN       DEFAULT FALSE,
    approval_required    BOOLEAN       DEFAULT FALSE,
    approver_ids         TEXT,                      -- JSON array of user IDs
    smoke_test_step_name VARCHAR(255),
    notify_channel_ids   TEXT,                      -- JSON array of channel IDs
    variables_json       TEXT          DEFAULT '{}',-- env-specific variable overrides
    ...
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_pipeline_env ON environments (pipeline_id, name) WHERE deleted_at IS NULL;
```

### `pipeline_runs` (relevant columns)

```sql
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id                 BIGSERIAL PRIMARY KEY,
    pipeline_id        BIGINT        NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    environment_id     BIGINT        NOT NULL DEFAULT 0,  -- execution target
    snapshot_id        BIGINT        NOT NULL,
    cluster_id         BIGINT        NOT NULL,            -- denormalized from environment
    namespace          VARCHAR(253)  NOT NULL,            -- denormalized from environment
    ...
);
```

---

## 7. Implementation Plan

### Sprint Status

| Sprint | Focus | Status |
|--------|-------|--------|
| Sprint 1 | Data Foundation (Models + Migration) | ✅ Complete |
| Sprint 2 | Core Services | 🔲 Pending |
| Sprint 3 | Execution Engine (highest risk) | 🔲 Pending |
| Sprint 4 | API Handlers + Routes | 🔲 Pending |
| Sprint 5 | Frontend Core | ✅ Complete (ahead of schedule) |
| Sprint 6 | Frontend Polish | ✅ Complete (ahead of schedule) |

### Priority Matrix

#### Layer 0: Data Foundation ✅

| ID | Work Item | Score | Status |
|----|-----------|-------|--------|
| M1 | Pipeline model: remove `ClusterID`, `Namespace` | 4.6 | ✅ |
| M2 | PipelineRun model: add `EnvironmentID` | 4.8 | ✅ |
| M3 | Environment model: unique index + `VariablesJSON` | 4.4 | ✅ |
| M4 | PipelineSecret: `cluster` scope → `environment` scope | 3.5 | ✅ |
| M5 | Database migration SQL (merged into 008/009) | 4.2 | ✅ |

#### Layer 1: Backend Services

| ID | Work Item | Score | Blocked By |
|----|-----------|-------|------------|
| S1 | PipelineService: remove ClusterID from queries | 4.0 | M1 |
| S2 | PipelineScheduler: `TriggerRunInput` refactor (env-based) | 3.6 | M1, M2 |
| S3 | JobBuilder: resolve cluster from env context | 3.6 | M2 |
| S4 | JobWatcher: env-aware cluster lookup | 2.9 | M1, M2 |
| S5 | EnvironmentService: `GetEnvironmentWithCluster`, promotion helpers | 3.8 | M3 |
| S6 | Webhook trigger: default env resolution (lowest OrderIndex) | 2.9 | S2 |

#### Layer 2: API Handlers + Routes

| ID | Work Item | Score | Blocked By |
|----|-----------|-------|------------|
| H1 | PipelineHandler: cluster-free CRUD | 3.5 | S1 |
| H2 | PipelineRunHandler: env-based TriggerRun | 3.7 | S2, S3 |
| H3 | PipelineRunHandler: `PromoteRun` endpoint | 3.6 | S2, S5 |
| H4 | EnvironmentHandler: add `Get` endpoint | 3.1 | S5 |
| H5 | PipelineSecretHandler: 3-level scope hierarchy | 2.3 | M4 |
| R1 | `routes_pipeline.go`: new top-level route file | 3.8 | H1–H5 |
| R2 | `PipelineAccessRequired` middleware | 3.8 | — |
| R3 | Delete `routes_cluster_pipeline.go` | 2.9 | R1 |

#### Layer 3: Frontend

| ID | Work Item | Score | Blocked By |
|----|-----------|-------|------------|
| F1 | `pipelineService.ts`: remove clusterId from all API paths | 4.0 | R1 | ✅ |
| F2 | `PipelineList.tsx`: remove cluster context, add env summary | 3.0 | F1, H1 | ✅ |
| F3 | `PipelineRunDetail.tsx`: env info + Promote button | 3.3 | F1, H2, H3 | ✅ (partial — Promote pending H3) |
| F4 | `PipelineEnvironments.tsx`: cluster selector per env | 2.8 | F1, H4 | ✅ |
| F5 | Routes & sidebar: move Pipeline to top-level CI/CD | 3.1 | F1 | ✅ |
| F6 | `environmentService.ts`: update API paths | 3.5 | R1 | ✅ |
| F7 | `PipelineEditor.tsx`: remove namespace field | 2.9 | F1 | ✅ |

### Critical Path

```
M1 → M2 → S2 → H2 → R1 → F1 → F3
```

**Sprint 3 (S2 Scheduler)** is the bottleneck — most complex single item (~200 LOC, 10+ ClusterID references, concurrency logic). Plan extra review before proceeding to Sprint 4.

### Sprint Detail

#### Sprint 2 — Core Services

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 2.1 | S1 | PipelineService: remove ClusterID queries | ~80 |
| 2.2 | S5 | EnvironmentService: enrichment + promotion helpers | ~120 |
| 2.3 | R2 | `PipelineAccessRequired` middleware | ~40 |

**Gate**: `go build ./...` passes. Service unit tests pass.

#### Sprint 3 — Execution Engine ⚠️ Highest Risk

| Order | ID | Work Item | Est. LOC | Risk Notes |
|-------|----|-----------|----------|------------|
| 3.1 | S2 | PipelineScheduler: `TriggerRunInput` refactor | ~200 | 10+ ClusterID refs, concurrency logic |
| 3.2 | S3 | JobBuilder: cluster from env context | ~60 | K8s client resolution path |
| 3.3 | S4 | JobWatcher: env-aware cluster lookup | ~80 | 6+ ClusterID refs, polling logic |
| 3.4 | S6 | Webhook trigger: default env resolution | ~40 | Must handle "no env" gracefully |

**Gate**: Can trigger a run via scheduler with env-derived cluster. Watcher picks up job status.

#### Sprint 4 — API Layer

| Order | ID | Work Item | Est. LOC |
|-------|----|-----------|----------|
| 4.1 | H1 | PipelineHandler: cluster-free CRUD | ~150 |
| 4.2 | H2 | PipelineRunHandler: env-based trigger | ~180 |
| 4.3 | H3 | PipelineRunHandler: `PromoteRun` | ~80 |
| 4.4 | H4 | EnvironmentHandler: add Get | ~60 |
| 4.5 | H5 | PipelineSecretHandler: 3-level scope | ~60 |
| 4.6 | R1 | `routes_pipeline.go`: new top-level routes | ~90 |
| 4.7 | R3 | Delete `routes_cluster_pipeline.go` | ~5 |

**Gate**: Manual API test — create pipeline → add env → trigger run → promote.

#### Sprint 5 — Frontend Core ✅

| Order | ID | Work Item | Est. LOC | Status |
|-------|----|-----------|----------|--------|
| 5.1 | F1 | `pipelineService.ts`: remove clusterId | ~80 | ✅ |
| 5.2 | F6 | `environmentService.ts`: update paths | ~30 | ✅ |
| 5.3 | F5 | Routes & sidebar: top-level CI/CD | ~40 | ✅ |
| 5.4 | F2 | `PipelineList.tsx`: env summary column | ~100 | ✅ |

**Gate**: Navigate to `/pipelines`, see list with environment badges. ✅

#### Sprint 6 — Frontend Polish ✅

| Order | ID | Work Item | Est. LOC | Status |
|-------|----|-----------|----------|--------|
| 6.1 | F3 | `PipelineRunDetail.tsx`: env info + Promote button | ~120 | ✅ (Promote button pending H3) |
| 6.2 | F4 | `PipelineEnvironments.tsx`: cluster selector | ~80 | ✅ |
| 6.3 | F7 | `PipelineEditor.tsx`: remove namespace field | ~20 | ✅ |

**Gate**: End-to-end walkthrough in browser. ✅ (pending backend Sprint 2-4)

---

## 8. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scheduler refactor breaks concurrency logic | Medium | High | Write concurrency unit tests BEFORE refactoring. Keep cluster-level concurrency counting, just source ClusterID from env. |
| JobWatcher loses track of jobs after env change | Low | High | PipelineRun.ClusterID is denormalized — watcher still reads it directly. |
| Frontend `useClusterId()` hook deeply embedded | Medium | Medium | Audit all `useClusterId` / `useParams` references in pipeline pages before Sprint 5. |
| Webhook breaks for existing integrations | Medium | High | Keep old webhook URL as deprecated alias for one release. Log warnings. |

---

## 9. Open Questions

1. **Pipeline RBAC** — Per-pipeline roles (owner/contributor/viewer)?
   - **Decision**: Defer. Start with "any authenticated user can access any pipeline."

2. **Concurrency across environments** — Does `max_concurrent_runs=1` apply globally or per-environment?
   - **Decision**: Per-pipeline globally. One active run across all environments to prevent conflicting deployments.

3. **Webhook routing** — Should webhooks move from `/clusters/:clusterID/webhooks/...` to `/pipelines/:pipelineID/webhooks/...`?
   - **Decision**: Yes (planned in Sprint 3 / S6). Default environment = lowest `order_index`.

---

## Appendix A: API Response Shapes

### PipelineInfo

```json
{
  "id": 1,
  "name": "frontend-app",
  "description": "Build and deploy frontend application",
  "current_version_id": 5,
  "concurrency_policy": "cancel_previous",
  "max_concurrent_runs": 1,
  "environments": [
    {
      "id": 10,
      "name": "dev",
      "cluster_id": 1,
      "cluster_name": "projectA-cluster-dev",
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
      "cluster_name": "projectA-cluster-stg",
      "cluster_status": "healthy",
      "namespace": "frontend-staging",
      "order_index": 2,
      "auto_promote": false,
      "approval_required": true,
      "last_run_status": "pending_approval"
    },
    {
      "id": 12,
      "name": "prod",
      "cluster_id": 3,
      "cluster_name": "projectA-cluster-pro",
      "cluster_status": "healthy",
      "namespace": "frontend-prod",
      "order_index": 3,
      "auto_promote": false,
      "approval_required": true,
      "last_run_status": "success"
    }
  ],
  "created_at": "2026-04-15T10:00:00Z",
  "updated_at": "2026-04-15T12:00:00Z"
}
```

### TriggerRunRequest

```json
{
  "version_id": null,
  "variables": {
    "IMAGE_TAG": "v1.2.3"
  }
}
```

### PipelineRunInfo

```json
{
  "id": 99,
  "pipeline_id": 1,
  "environment_id": 10,
  "environment_name": "dev",
  "cluster_id": 1,
  "cluster_name": "projectA-cluster-dev",
  "namespace": "frontend-dev",
  "snapshot_id": 5,
  "status": "success",
  "trigger_type": "webhook",
  "triggered_by_user": 42,
  "queued_at": "2026-04-15T10:00:00Z",
  "started_at": "2026-04-15T10:00:05Z",
  "finished_at": "2026-04-15T10:04:30Z"
}
```

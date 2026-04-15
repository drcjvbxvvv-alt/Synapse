-- 008: Pipeline CI/CD core tables + extend existing tables

-- ---------------------------------------------------------------------------
-- pipelines — Pipeline 定義
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipelines (
    id                  BIGSERIAL PRIMARY KEY,
    name                VARCHAR(255)  NOT NULL,
    description         TEXT,
    current_version_id  BIGINT,
    concurrency_group   VARCHAR(255),
    concurrency_policy  VARCHAR(30)   DEFAULT 'cancel_previous',
    max_concurrent_runs INT           DEFAULT 1,
    notify_on_success   JSONB,
    notify_on_failure   JSONB,
    notify_on_scan      JSONB,
    created_by          BIGINT        NOT NULL,
    created_at          TIMESTAMPTZ   DEFAULT now(),
    updated_at          TIMESTAMPTZ   DEFAULT now(),
    deleted_at          TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_name
    ON pipelines (name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pipelines_deleted_at   ON pipelines (deleted_at);

-- ---------------------------------------------------------------------------
-- pipeline_versions — 不可變版本快照
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipeline_versions (
    id            BIGSERIAL PRIMARY KEY,
    pipeline_id   BIGINT       NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    version       INT          NOT NULL,
    steps_json    TEXT         NOT NULL,
    triggers_json TEXT,
    env_json      TEXT,
    runtime_json  TEXT,
    workspace_json TEXT,
    hash_sha256   VARCHAR(64)  NOT NULL,
    created_by    BIGINT       NOT NULL,
    created_at    TIMESTAMPTZ  DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_version
    ON pipeline_versions (pipeline_id, version);
CREATE INDEX IF NOT EXISTS idx_pipeline_versions_hash ON pipeline_versions (hash_sha256);

-- FK: pipelines.current_version_id → pipeline_versions.id
ALTER TABLE pipelines
    ADD CONSTRAINT fk_pipelines_current_version
    FOREIGN KEY (current_version_id) REFERENCES pipeline_versions(id);

-- ---------------------------------------------------------------------------
-- pipeline_runs — 一次具體執行記錄
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id                 BIGSERIAL PRIMARY KEY,
    pipeline_id        BIGINT       NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    environment_id     BIGINT       NOT NULL DEFAULT 0,
    snapshot_id        BIGINT       NOT NULL REFERENCES pipeline_versions(id),
    cluster_id         BIGINT       NOT NULL,
    namespace          VARCHAR(253) NOT NULL,
    status             VARCHAR(20)  NOT NULL DEFAULT 'queued',
    trigger_type       VARCHAR(20)  NOT NULL,
    trigger_payload    TEXT,
    triggered_by_user  BIGINT       NOT NULL,
    concurrency_group  VARCHAR(255),
    rerun_from_id      BIGINT,
    error              TEXT,
    queued_at          TIMESTAMPTZ  DEFAULT now(),
    started_at         TIMESTAMPTZ,
    finished_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ  DEFAULT now(),
    updated_at         TIMESTAMPTZ  DEFAULT now(),
    deleted_at         TIMESTAMPTZ,
    bound_node_name    VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_pipeline_id      ON pipeline_runs (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_env              ON pipeline_runs (environment_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_snapshot_id       ON pipeline_runs (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_cluster_id        ON pipeline_runs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_status            ON pipeline_runs (status);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_concurrency_group ON pipeline_runs (concurrency_group);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_deleted_at        ON pipeline_runs (deleted_at);

-- ---------------------------------------------------------------------------
-- step_runs — 每個 Step 的執行記錄（對應一個 K8s Job）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS step_runs (
    id              BIGSERIAL PRIMARY KEY,
    pipeline_run_id BIGINT       NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    step_name       VARCHAR(255) NOT NULL,
    step_type       VARCHAR(50)  NOT NULL,
    step_index      INT          NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    image           VARCHAR(512),
    command         TEXT,
    config_json     TEXT,
    job_name        VARCHAR(255),
    job_namespace   VARCHAR(253),
    exit_code       INT,
    error           TEXT,
    retry_count     INT          DEFAULT 0,
    max_retries     INT          DEFAULT 0,
    depends_on      JSONB,
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  DEFAULT now(),
    updated_at      TIMESTAMPTZ  DEFAULT now(),
    scan_result_id  BIGINT,
    rollout_status  VARCHAR(30),
    rollout_weight  INT
);

CREATE INDEX IF NOT EXISTS idx_step_runs_pipeline_run_id ON step_runs (pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_step_runs_status          ON step_runs (status);

-- ---------------------------------------------------------------------------
-- pipeline_secrets — CI/CD 專用密鑰（AES-256-GCM 加密）
-- scope: global / environment / pipeline
-- scope_ref: NULL(global) / environment_id / pipeline_id
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipeline_secrets (
    id          BIGSERIAL PRIMARY KEY,
    scope       VARCHAR(20)  NOT NULL,
    scope_ref   BIGINT,
    name        VARCHAR(100) NOT NULL,
    value_enc   TEXT         NOT NULL,
    description VARCHAR(255),
    created_by  BIGINT       NOT NULL,
    created_at  TIMESTAMPTZ  DEFAULT now(),
    updated_at  TIMESTAMPTZ  DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_scope_name
    ON pipeline_secrets (scope, scope_ref, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pipeline_secrets_deleted_at ON pipeline_secrets (deleted_at);

-- ---------------------------------------------------------------------------
-- pipeline_artifacts — Step 產出物記錄
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipeline_artifacts (
    id              BIGSERIAL PRIMARY KEY,
    pipeline_run_id BIGINT       NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    step_run_id     BIGINT       NOT NULL REFERENCES step_runs(id) ON DELETE CASCADE,
    kind            VARCHAR(50),
    name            VARCHAR(255),
    reference       TEXT,
    size_bytes      BIGINT,
    metadata_json   TEXT,
    created_at      TIMESTAMPTZ  DEFAULT now(),
    expires_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pipeline_artifacts_run  ON pipeline_artifacts (pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_artifacts_kind ON pipeline_artifacts (kind);

-- ---------------------------------------------------------------------------
-- pipeline_logs — Step 執行日誌（分塊儲存）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipeline_logs (
    id              BIGSERIAL PRIMARY KEY,
    pipeline_run_id BIGINT       NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    step_run_id     BIGINT       NOT NULL REFERENCES step_runs(id) ON DELETE CASCADE,
    chunk_seq       INT          NOT NULL,
    content         TEXT,
    stored_at       TIMESTAMPTZ  DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_step_chunk ON pipeline_logs (step_run_id, chunk_seq);
CREATE INDEX IF NOT EXISTS idx_pipeline_logs_run ON pipeline_logs (pipeline_run_id);

-- ---------------------------------------------------------------------------
-- Extend existing tables for Pipeline integration
-- ---------------------------------------------------------------------------

-- image_scan_results: add pipeline source tracking
ALTER TABLE image_scan_results
    ADD COLUMN IF NOT EXISTS scan_source      VARCHAR(20) DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS pipeline_run_id  BIGINT,
    ADD COLUMN IF NOT EXISTS step_run_id      BIGINT;

CREATE INDEX IF NOT EXISTS idx_scan_results_pipeline_run
    ON image_scan_results (pipeline_run_id) WHERE pipeline_run_id IS NOT NULL;

-- approval_requests: add pipeline deployment gate fields
ALTER TABLE approval_requests
    ADD COLUMN IF NOT EXISTS pipeline_run_id   BIGINT,
    ADD COLUMN IF NOT EXISTS from_environment  VARCHAR(100),
    ADD COLUMN IF NOT EXISTS to_environment    VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_approval_requests_pipeline_run
    ON approval_requests (pipeline_run_id) WHERE pipeline_run_id IS NOT NULL;

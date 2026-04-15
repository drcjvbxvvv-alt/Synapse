-- 011 down: 還原 Environment 表與 pipeline_runs.environment_id 欄位

-- ---------------------------------------------------------------------------
-- environments — 環境管理（§13）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS environments (
    id                   BIGSERIAL PRIMARY KEY,
    name                 VARCHAR(255)  NOT NULL,
    pipeline_id          BIGINT        NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    cluster_id           BIGINT        NOT NULL,
    namespace            VARCHAR(253)  NOT NULL,
    order_index          INT           NOT NULL,
    auto_promote         BOOLEAN       DEFAULT FALSE,
    approval_required    BOOLEAN       DEFAULT FALSE,
    approver_ids         TEXT,
    smoke_test_step_name VARCHAR(255),
    notify_channel_ids   TEXT,
    variables_json       TEXT          DEFAULT '{}',
    created_at           TIMESTAMPTZ   DEFAULT now(),
    updated_at           TIMESTAMPTZ   DEFAULT now(),
    deleted_at           TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_pipeline_env
    ON environments (pipeline_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_env_order      ON environments (pipeline_id, order_index);
CREATE INDEX IF NOT EXISTS idx_env_cluster    ON environments (cluster_id);
CREATE INDEX IF NOT EXISTS idx_env_deleted_at ON environments (deleted_at);

-- ---------------------------------------------------------------------------
-- promotion_history — 環境晉升歷史（§13）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS promotion_history (
    id               BIGSERIAL PRIMARY KEY,
    pipeline_id      BIGINT        NOT NULL,
    pipeline_run_id  BIGINT        NOT NULL,
    from_environment VARCHAR(255)  NOT NULL,
    to_environment   VARCHAR(255)  NOT NULL,
    status           VARCHAR(30)   NOT NULL,
    promoted_by      BIGINT,
    approval_id      BIGINT,
    reason           TEXT,
    created_at       TIMESTAMPTZ   DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_promo_pipeline ON promotion_history (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_promo_run      ON promotion_history (pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_promo_approval ON promotion_history (approval_id);

-- 還原 pipeline_runs.environment_id 欄位
ALTER TABLE pipeline_runs ADD COLUMN IF NOT EXISTS environment_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_env ON pipeline_runs (environment_id);

-- 009: Registry credentials + Environment management tables

-- ---------------------------------------------------------------------------
-- registries — 映像 Registry 連線設定（§11）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS registries (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(255)  NOT NULL,
    type            VARCHAR(50)   NOT NULL,          -- harbor / dockerhub / acr / ecr / gcr
    url             VARCHAR(512)  NOT NULL,
    username        VARCHAR(255),
    password_enc    TEXT,                             -- AES-256-GCM
    insecure_tls    BOOLEAN       DEFAULT FALSE,
    ca_bundle_enc   TEXT,                             -- AES-256-GCM（自簽 CA）
    default_project VARCHAR(255),
    enabled         BOOLEAN       DEFAULT TRUE,
    created_by      BIGINT        NOT NULL,
    created_at      TIMESTAMPTZ   DEFAULT now(),
    updated_at      TIMESTAMPTZ   DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_registries_name
    ON registries (name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_registries_type       ON registries (type);
CREATE INDEX IF NOT EXISTS idx_registries_deleted_at ON registries (deleted_at);

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
    approver_ids         TEXT,                        -- JSON array of user IDs
    smoke_test_step_name VARCHAR(255),
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
    status           VARCHAR(30)   NOT NULL,          -- pending / approved / rejected / auto_promoted
    promoted_by      BIGINT,
    approval_id      BIGINT,
    reason           TEXT,
    created_at       TIMESTAMPTZ   DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_promo_pipeline ON promotion_history (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_promo_run      ON promotion_history (pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_promo_approval ON promotion_history (approval_id);

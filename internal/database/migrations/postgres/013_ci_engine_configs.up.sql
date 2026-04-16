-- 013_ci_engine_configs.up.sql
-- M18a: External CI engine connection profiles
--
-- Each row stores credentials and metadata for one external CI engine instance
-- (GitLab CI, Jenkins, Tekton, Argo Workflows, GitHub Actions).
-- The built-in "native" engine does NOT require a row in this table.
--
-- Sensitive columns (token, password, webhook_secret, ca_bundle) are stored
-- encrypted via AES-256-GCM by the GORM model hooks — the DB sees cipher text.

CREATE TABLE IF NOT EXISTS ci_engine_configs (
    id                  BIGSERIAL     PRIMARY KEY,
    name                VARCHAR(100)  NOT NULL,
    engine_type         VARCHAR(20)   NOT NULL,          -- native/gitlab/jenkins/tekton/argo/github
    enabled             BOOLEAN       NOT NULL DEFAULT TRUE,

    -- Connection
    endpoint            VARCHAR(500),

    -- Authentication
    auth_type           VARCHAR(20),                     -- token/basic/kubeconfig/service_acct
    username            VARCHAR(100),
    token               TEXT,                            -- encrypted PAT / API token
    password            TEXT,                            -- encrypted basic-auth password
    webhook_secret      TEXT,                            -- encrypted HMAC shared secret

    -- Cluster reference (Tekton / Argo only; NULL for off-cluster engines)
    cluster_id          BIGINT,

    -- Engine-specific settings (JSON blob)
    extra_json          TEXT,

    -- TLS
    insecure_skip_verify BOOLEAN      NOT NULL DEFAULT FALSE,
    ca_bundle           TEXT,                            -- encrypted PEM CA bundle

    -- Health tracking (updated by probe worker)
    last_checked_at     TIMESTAMPTZ,
    last_healthy        BOOLEAN       NOT NULL DEFAULT FALSE,
    last_version        VARCHAR(50),
    last_error          TEXT,

    -- Ownership
    created_by          BIGINT        NOT NULL,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

-- Unique engine name (soft-delete safe)
CREATE UNIQUE INDEX IF NOT EXISTS idx_ci_engine_name
    ON ci_engine_configs (name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ci_engine_configs_engine_type ON ci_engine_configs (engine_type);
CREATE INDEX IF NOT EXISTS idx_ci_engine_configs_cluster_id  ON ci_engine_configs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_ci_engine_configs_deleted_at  ON ci_engine_configs (deleted_at);

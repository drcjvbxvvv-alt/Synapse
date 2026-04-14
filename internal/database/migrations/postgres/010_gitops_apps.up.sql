-- 010: GitOps Applications table (§12)

CREATE TABLE IF NOT EXISTS gitops_apps (
    id               BIGSERIAL PRIMARY KEY,
    name             VARCHAR(255)  NOT NULL,
    source           VARCHAR(20)   NOT NULL DEFAULT 'native',  -- native / argocd
    git_provider_id  BIGINT,
    repo_url         VARCHAR(512),
    branch           VARCHAR(255),
    path             VARCHAR(512),
    render_type      VARCHAR(50)   NOT NULL DEFAULT 'raw',     -- raw / kustomize / helm
    helm_values      TEXT,
    cluster_id       BIGINT        NOT NULL,
    namespace        VARCHAR(253)  NOT NULL,
    sync_policy      VARCHAR(50)   NOT NULL DEFAULT 'manual',  -- auto / manual
    sync_interval    INT           DEFAULT 300,
    last_synced_at   TIMESTAMPTZ,
    last_diff_at     TIMESTAMPTZ,
    last_diff_result TEXT,
    status           VARCHAR(50)   NOT NULL DEFAULT 'unknown',
    status_message   TEXT,
    created_by       BIGINT        NOT NULL,
    created_at       TIMESTAMPTZ   DEFAULT now(),
    updated_at       TIMESTAMPTZ   DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_gitops_name_cluster
    ON gitops_apps (name, cluster_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_gitops_source     ON gitops_apps (source);
CREATE INDEX IF NOT EXISTS idx_gitops_status     ON gitops_apps (status);
CREATE INDEX IF NOT EXISTS idx_gitops_cluster    ON gitops_apps (cluster_id);
CREATE INDEX IF NOT EXISTS idx_gitops_deleted_at ON gitops_apps (deleted_at);

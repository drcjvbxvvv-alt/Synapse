CREATE TABLE IF NOT EXISTS namespace_budgets (
    id                 bigserial        PRIMARY KEY,
    cluster_id         bigint           NOT NULL,
    namespace          varchar(128)     NOT NULL,
    cpu_cores_limit    double precision NOT NULL DEFAULT 0,
    memory_gib_limit   double precision NOT NULL DEFAULT 0,
    monthly_cost_limit double precision NOT NULL DEFAULT 0,
    alert_threshold    double precision NOT NULL DEFAULT 0.8,
    enabled            boolean          NOT NULL DEFAULT true,
    created_at         timestamptz(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         timestamptz(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_budget_cluster_ns ON namespace_budgets (cluster_id, namespace);

CREATE TABLE IF NOT EXISTS namespace_budgets (
    id                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    cluster_id         BIGINT UNSIGNED NOT NULL,
    namespace          VARCHAR(128)    NOT NULL,
    cpu_cores_limit    DOUBLE          NOT NULL DEFAULT 0,
    memory_gib_limit   DOUBLE          NOT NULL DEFAULT 0,
    monthly_cost_limit DOUBLE          NOT NULL DEFAULT 0,
    alert_threshold    DOUBLE          NOT NULL DEFAULT 0.8,
    enabled            BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE INDEX idx_budget_cluster_ns (cluster_id, namespace)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

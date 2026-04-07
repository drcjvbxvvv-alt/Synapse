-- =============================================================================
-- Synapse — SQLite 初始化 SQL
-- 適用：SQLite 3.x（開發 / 單節點部署）
-- 用法：sqlite3 ./data/synapse.db < init_sqlite.sql
--
-- 正式環境建議改用 MySQL（init_mysql.sql）。
-- 若使用 SQLCipher 加密 build，請改用：
--   sqlcipher ./data/synapse.db
--   sqlite> PRAGMA key='<your-passphrase>';
--   sqlite> .read deploy/sql/init_sqlite.sql
-- =============================================================================

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = OFF;

-- -----------------------------------------------------------------------------
-- 1. users
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "users" (
  "id"            integer        NOT NULL PRIMARY KEY AUTOINCREMENT,
  "username"      varchar(50)    NOT NULL UNIQUE,
  "password_hash" varchar(255)   NOT NULL DEFAULT '',
  "salt"          varchar(32)    NOT NULL DEFAULT '',
  "email"         varchar(100)   NOT NULL DEFAULT '',
  "display_name"  varchar(100)   NOT NULL DEFAULT '',
  "phone"         varchar(20)    NOT NULL DEFAULT '',
  "auth_type"     varchar(20)    NOT NULL DEFAULT 'local',
  "status"        varchar(20)    NOT NULL DEFAULT 'active',
  "last_login_at" datetime,
  "last_login_ip" varchar(50)    NOT NULL DEFAULT '',
  "created_at"    datetime       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"    datetime       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"    datetime
);
CREATE INDEX IF NOT EXISTS "idx_users_deleted_at" ON "users"("deleted_at");

-- -----------------------------------------------------------------------------
-- 2. clusters
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "clusters" (
  "id"                   integer     NOT NULL PRIMARY KEY AUTOINCREMENT,
  "name"                 varchar(100) NOT NULL UNIQUE,
  "api_server"           varchar(255) NOT NULL,
  "kubeconfig_enc"       text,
  "ca_enc"               text,
  "sa_token_enc"         text,
  "version"              varchar(50)  NOT NULL DEFAULT '',
  "status"               varchar(20)  NOT NULL DEFAULT 'unknown',
  "labels"               text,
  "cert_expire_at"       datetime,
  "last_heartbeat"       datetime,
  "created_by"           integer      NOT NULL DEFAULT 0,
  "monitoring_config"    text,
  "alertmanager_config"  text,
  "created_at"           datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"           datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"           datetime
);
CREATE INDEX IF NOT EXISTS "idx_clusters_deleted_at" ON "clusters"("deleted_at");

-- -----------------------------------------------------------------------------
-- 3. cluster_metrics
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "cluster_metrics" (
  "cluster_id"    integer NOT NULL PRIMARY KEY,
  "node_count"    integer NOT NULL DEFAULT 0,
  "ready_nodes"   integer NOT NULL DEFAULT 0,
  "pod_count"     integer NOT NULL DEFAULT 0,
  "running_pods"  integer NOT NULL DEFAULT 0,
  "cpu_usage"     real    NOT NULL DEFAULT 0,
  "memory_usage"  real    NOT NULL DEFAULT 0,
  "storage_usage" real    NOT NULL DEFAULT 0,
  "updated_at"    datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- -----------------------------------------------------------------------------
-- 4. terminal_sessions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "terminal_sessions" (
  "id"          integer     NOT NULL PRIMARY KEY AUTOINCREMENT,
  "user_id"     integer     NOT NULL,
  "cluster_id"  integer     NOT NULL,
  "target_type" varchar(20) NOT NULL,
  "target_ref"  text,
  "namespace"   varchar(100) NOT NULL DEFAULT '',
  "pod"         varchar(100) NOT NULL DEFAULT '',
  "container"   varchar(100) NOT NULL DEFAULT '',
  "node"        varchar(100) NOT NULL DEFAULT '',
  "start_at"    datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "end_at"      datetime,
  "input_size"  integer     NOT NULL DEFAULT 0,
  "status"      varchar(20) NOT NULL DEFAULT 'active',
  "created_at"  datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"  datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"  datetime
);
CREATE INDEX IF NOT EXISTS "idx_terminal_sessions_deleted_at" ON "terminal_sessions"("deleted_at");

-- -----------------------------------------------------------------------------
-- 5. terminal_commands
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "terminal_commands" (
  "id"         integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "session_id" integer      NOT NULL,
  "timestamp"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "raw_input"  text,
  "parsed_cmd" varchar(1024) NOT NULL DEFAULT '',
  "exit_code"  integer,
  "created_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_terminal_commands_session_id" ON "terminal_commands"("session_id");

-- -----------------------------------------------------------------------------
-- 6. audit_logs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "audit_logs" (
  "id"            integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "user_id"       integer      NOT NULL,
  "action"        varchar(100) NOT NULL,
  "resource_type" varchar(50)  NOT NULL,
  "resource_ref"  text,
  "result"        varchar(20)  NOT NULL,
  "ip"            varchar(45)  NOT NULL DEFAULT '',
  "user_agent"    varchar(500) NOT NULL DEFAULT '',
  "details"       text,
  "created_at"    datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_audit_logs_user_id" ON "audit_logs"("user_id");

-- -----------------------------------------------------------------------------
-- 7. operation_logs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "operation_logs" (
  "id"            integer       NOT NULL PRIMARY KEY AUTOINCREMENT,
  "user_id"       integer,
  "username"      varchar(100)  NOT NULL DEFAULT '',
  "method"        varchar(10)   NOT NULL DEFAULT '',
  "path"          varchar(500)  NOT NULL DEFAULT '',
  "query"         varchar(1000) NOT NULL DEFAULT '',
  "module"        varchar(50)   NOT NULL DEFAULT '',
  "action"        varchar(100)  NOT NULL DEFAULT '',
  "cluster_id"    integer,
  "cluster_name"  varchar(100)  NOT NULL DEFAULT '',
  "namespace"     varchar(100)  NOT NULL DEFAULT '',
  "resource_type" varchar(50)   NOT NULL DEFAULT '',
  "resource_name" varchar(200)  NOT NULL DEFAULT '',
  "request_body"  text,
  "status_code"   integer       NOT NULL DEFAULT 0,
  "success"       integer       NOT NULL DEFAULT 0,
  "error_message" varchar(1000) NOT NULL DEFAULT '',
  "client_ip"     varchar(45)   NOT NULL DEFAULT '',
  "user_agent"    varchar(500)  NOT NULL DEFAULT '',
  "duration"      integer       NOT NULL DEFAULT 0,
  "created_at"    datetime      NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_op_user_time"    ON "operation_logs"("user_id", "created_at");
CREATE INDEX IF NOT EXISTS "idx_op_logs_success" ON "operation_logs"("success");

-- -----------------------------------------------------------------------------
-- 8. system_settings
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "system_settings" (
  "id"         integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "config_key" varchar(100) NOT NULL UNIQUE,
  "value"      text,
  "type"       varchar(50)  NOT NULL DEFAULT '',
  "created_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" datetime
);
CREATE INDEX IF NOT EXISTS "idx_system_settings_deleted_at" ON "system_settings"("deleted_at");

-- -----------------------------------------------------------------------------
-- 9. argocd_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "argocd_configs" (
  "id"                   integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"           integer      NOT NULL UNIQUE,
  "enabled"              integer      NOT NULL DEFAULT 0,
  "server_url"           varchar(255) NOT NULL DEFAULT '',
  "auth_type"            varchar(20)  NOT NULL DEFAULT '',
  "token"                text,
  "username"             varchar(100) NOT NULL DEFAULT '',
  "password"             text,
  "insecure"             integer      NOT NULL DEFAULT 0,
  "git_repo_url"         varchar(500) NOT NULL DEFAULT '',
  "git_branch"           varchar(100) NOT NULL DEFAULT 'main',
  "git_path"             varchar(255) NOT NULL DEFAULT '',
  "git_auth_type"        varchar(20)  NOT NULL DEFAULT '',
  "git_username"         varchar(100) NOT NULL DEFAULT '',
  "git_password"         text,
  "git_ssh_key"          text,
  "argo_cd_cluster_name" varchar(100) NOT NULL DEFAULT '',
  "argo_cd_project"      varchar(100) NOT NULL DEFAULT 'default',
  "connection_status"    varchar(20)  NOT NULL DEFAULT '',
  "last_test_at"         datetime,
  "error_message"        text,
  "created_at"           datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"           datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"           datetime
);
CREATE INDEX IF NOT EXISTS "idx_argocd_configs_deleted_at" ON "argocd_configs"("deleted_at");

-- -----------------------------------------------------------------------------
-- 10. user_groups
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "user_groups" (
  "id"          integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "name"        varchar(50)  NOT NULL UNIQUE,
  "description" varchar(255) NOT NULL DEFAULT '',
  "created_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"  datetime
);
CREATE INDEX IF NOT EXISTS "idx_user_groups_deleted_at" ON "user_groups"("deleted_at");

-- -----------------------------------------------------------------------------
-- 11. user_group_members
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "user_group_members" (
  "user_id"       integer NOT NULL,
  "user_group_id" integer NOT NULL,
  PRIMARY KEY ("user_id", "user_group_id")
);

-- -----------------------------------------------------------------------------
-- 12. cluster_permissions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "cluster_permissions" (
  "id"              integer     NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"      integer     NOT NULL,
  "user_id"         integer,
  "user_group_id"   integer,
  "permission_type" varchar(50) NOT NULL,
  "namespaces"      text,
  "custom_role_ref" varchar(200) NOT NULL DEFAULT '',
  "created_at"      datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"      datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"      datetime
);
CREATE INDEX IF NOT EXISTS "idx_cluster_permissions_cluster_id"    ON "cluster_permissions"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_cluster_permissions_user_id"       ON "cluster_permissions"("user_id");
CREATE INDEX IF NOT EXISTS "idx_cluster_permissions_user_group_id" ON "cluster_permissions"("user_group_id");
CREATE INDEX IF NOT EXISTS "idx_cluster_permissions_deleted_at"    ON "cluster_permissions"("deleted_at");

-- -----------------------------------------------------------------------------
-- 13. ai_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "ai_configs" (
  "id"          integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "provider"    varchar(50)  NOT NULL DEFAULT 'openai',
  "endpoint"    varchar(255) NOT NULL DEFAULT '',
  "api_key"     text,
  "model"       varchar(100) NOT NULL DEFAULT '',
  "api_version" varchar(50)  NOT NULL DEFAULT '',
  "enabled"     integer      NOT NULL DEFAULT 0,
  "created_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"  datetime
);
CREATE INDEX IF NOT EXISTS "idx_ai_configs_deleted_at" ON "ai_configs"("deleted_at");

-- -----------------------------------------------------------------------------
-- 14. helm_repositories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "helm_repositories" (
  "id"         integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "name"       varchar(128) NOT NULL UNIQUE,
  "url"        varchar(512) NOT NULL DEFAULT '',
  "username"   varchar(256) NOT NULL DEFAULT '',
  "password"   varchar(256) NOT NULL DEFAULT '',
  "created_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" datetime
);
CREATE INDEX IF NOT EXISTS "idx_helm_repositories_deleted_at" ON "helm_repositories"("deleted_at");

-- -----------------------------------------------------------------------------
-- 15. event_alert_rules
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "event_alert_rules" (
  "id"           integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"   integer      NOT NULL,
  "name"         varchar(100) NOT NULL,
  "description"  varchar(255) NOT NULL DEFAULT '',
  "namespace"    varchar(100) NOT NULL DEFAULT '',
  "event_reason" varchar(100) NOT NULL DEFAULT '',
  "event_type"   varchar(20)  NOT NULL DEFAULT '',
  "min_count"    integer      NOT NULL DEFAULT 1,
  "notify_type"  varchar(20)  NOT NULL DEFAULT '',
  "notify_url"   varchar(500) NOT NULL DEFAULT '',
  "enabled"      integer      NOT NULL DEFAULT 1,
  "created_at"   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"   datetime
);
CREATE INDEX IF NOT EXISTS "idx_event_alert_rules_cluster_id"  ON "event_alert_rules"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_event_alert_rules_deleted_at"  ON "event_alert_rules"("deleted_at");

-- -----------------------------------------------------------------------------
-- 16. event_alert_histories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "event_alert_histories" (
  "id"            integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "rule_id"       integer      NOT NULL,
  "cluster_id"    integer      NOT NULL,
  "rule_name"     varchar(100) NOT NULL DEFAULT '',
  "namespace"     varchar(100) NOT NULL DEFAULT '',
  "event_reason"  varchar(100) NOT NULL DEFAULT '',
  "event_type"    varchar(20)  NOT NULL DEFAULT '',
  "message"       text,
  "involved_obj"  varchar(200) NOT NULL DEFAULT '',
  "notify_result" varchar(50)  NOT NULL DEFAULT '',
  "triggered_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_event_alert_histories_rule_id"     ON "event_alert_histories"("rule_id");
CREATE INDEX IF NOT EXISTS "idx_event_alert_histories_cluster_id"  ON "event_alert_histories"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_event_alert_histories_triggered_at" ON "event_alert_histories"("triggered_at");

-- -----------------------------------------------------------------------------
-- 17. cost_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "cost_configs" (
  "id"                 integer     NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"         integer     NOT NULL UNIQUE,
  "cpu_price_per_core" real        NOT NULL DEFAULT 0.048,
  "mem_price_per_gi_b" real        NOT NULL DEFAULT 0.006,
  "currency"           varchar(10) NOT NULL DEFAULT 'USD',
  "updated_at"         datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- -----------------------------------------------------------------------------
-- 18. resource_snapshots
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "resource_snapshots" (
  "id"          integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"  integer      NOT NULL,
  "namespace"   varchar(128) NOT NULL,
  "workload"    varchar(256) NOT NULL,
  "date"        datetime     NOT NULL,
  "cpu_request" real         NOT NULL DEFAULT 0,
  "cpu_usage"   real         NOT NULL DEFAULT 0,
  "mem_request" real         NOT NULL DEFAULT 0,
  "mem_usage"   real         NOT NULL DEFAULT 0,
  "pod_count"   integer      NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS "idx_resource_snapshots_cluster_id" ON "resource_snapshots"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_resource_snapshots_date"       ON "resource_snapshots"("date");

-- -----------------------------------------------------------------------------
-- 19. cluster_occupancy_snapshots
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "cluster_occupancy_snapshots" (
  "id"                 integer  NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"         integer  NOT NULL,
  "date"               datetime NOT NULL,
  "allocatable_cpu"    real     NOT NULL DEFAULT 0,
  "allocatable_memory" real     NOT NULL DEFAULT 0,
  "requested_cpu"      real     NOT NULL DEFAULT 0,
  "requested_memory"   real     NOT NULL DEFAULT 0,
  "node_count"         integer  NOT NULL DEFAULT 0,
  "pod_count"          integer  NOT NULL DEFAULT 0,
  UNIQUE ("cluster_id", "date")
);

-- -----------------------------------------------------------------------------
-- 20. cloud_billing_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "cloud_billing_configs" (
  "id"                       integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"               integer      NOT NULL UNIQUE,
  "provider"                 varchar(20)  NOT NULL DEFAULT 'disabled',
  "aws_access_key_id"        varchar(128) NOT NULL DEFAULT '',
  "aws_secret_access_key"    varchar(256) NOT NULL DEFAULT '',
  "aws_region"               varchar(32)  NOT NULL DEFAULT 'us-east-1',
  "aws_linked_account_id"    varchar(20)  NOT NULL DEFAULT '',
  "gcp_project_id"           varchar(128) NOT NULL DEFAULT '',
  "gcp_billing_account_id"   varchar(64)  NOT NULL DEFAULT '',
  "gcp_service_account_json" text,
  "last_synced_at"           datetime,
  "last_error"               varchar(512) NOT NULL DEFAULT '',
  "created_at"               datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"               datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- -----------------------------------------------------------------------------
-- 21. cloud_billing_records
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "cloud_billing_records" (
  "id"         integer     NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id" integer     NOT NULL,
  "month"      varchar(7)  NOT NULL,
  "provider"   varchar(20) NOT NULL DEFAULT '',
  "service"    varchar(256) NOT NULL DEFAULT '',
  "amount"     real        NOT NULL DEFAULT 0,
  "currency"   varchar(10) NOT NULL DEFAULT 'USD',
  "created_at" datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_cloud_billing_records_cluster_id" ON "cloud_billing_records"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_cloud_billing_records_month"      ON "cloud_billing_records"("month");

-- -----------------------------------------------------------------------------
-- 22. image_scan_results
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "image_scan_results" (
  "id"             integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"     integer      NOT NULL,
  "namespace"      varchar(100) NOT NULL DEFAULT '',
  "pod_name"       varchar(255) NOT NULL DEFAULT '',
  "container_name" varchar(255) NOT NULL DEFAULT '',
  "image"          varchar(512) NOT NULL,
  "status"         varchar(20)  NOT NULL DEFAULT 'pending',
  "critical"       integer      NOT NULL DEFAULT 0,
  "high"           integer      NOT NULL DEFAULT 0,
  "medium"         integer      NOT NULL DEFAULT 0,
  "low"            integer      NOT NULL DEFAULT 0,
  "unknown"        integer      NOT NULL DEFAULT 0,
  "result_json"    text,
  "error"          varchar(512) NOT NULL DEFAULT '',
  "scanned_at"     datetime,
  "created_at"     datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"     datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_image_scan_results_cluster_id" ON "image_scan_results"("cluster_id");

-- -----------------------------------------------------------------------------
-- 23. bench_results
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "bench_results" (
  "id"          integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"  integer      NOT NULL,
  "status"      varchar(20)  NOT NULL DEFAULT 'pending',
  "pass"        integer      NOT NULL DEFAULT 0,
  "fail"        integer      NOT NULL DEFAULT 0,
  "warn"        integer      NOT NULL DEFAULT 0,
  "info"        integer      NOT NULL DEFAULT 0,
  "score"       real         NOT NULL DEFAULT 0,
  "result_json" text,
  "error"       varchar(512) NOT NULL DEFAULT '',
  "job_name"    varchar(255) NOT NULL DEFAULT '',
  "run_at"      datetime,
  "created_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"  datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_bench_results_cluster_id" ON "bench_results"("cluster_id");

-- -----------------------------------------------------------------------------
-- 24. sync_policies
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "sync_policies" (
  "id"                integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "name"              varchar(128) NOT NULL,
  "description"       varchar(512) NOT NULL DEFAULT '',
  "source_cluster_id" integer      NOT NULL,
  "source_namespace"  varchar(128) NOT NULL,
  "resource_type"     varchar(32)  NOT NULL,
  "resource_names"    text,
  "target_clusters"   text,
  "conflict_policy"   varchar(16)  NOT NULL DEFAULT 'skip',
  "schedule"          varchar(64)  NOT NULL DEFAULT '',
  "enabled"           integer      NOT NULL DEFAULT 1,
  "last_sync_at"      datetime,
  "last_sync_status"  varchar(16)  NOT NULL DEFAULT '',
  "created_at"        datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"        datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_sync_policies_source_cluster_id" ON "sync_policies"("source_cluster_id");

-- -----------------------------------------------------------------------------
-- 25. sync_histories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "sync_histories" (
  "id"           integer     NOT NULL PRIMARY KEY AUTOINCREMENT,
  "policy_id"    integer     NOT NULL,
  "triggered_by" varchar(64) NOT NULL DEFAULT '',
  "status"       varchar(16) NOT NULL DEFAULT '',
  "message"      text,
  "details"      text,
  "started_at"   datetime    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "finished_at"  datetime
);
CREATE INDEX IF NOT EXISTS "idx_sync_histories_policy_id" ON "sync_histories"("policy_id");

-- -----------------------------------------------------------------------------
-- 26. config_versions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "config_versions" (
  "id"            integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"    integer      NOT NULL,
  "resource_type" varchar(20)  NOT NULL,
  "namespace"     varchar(255) NOT NULL,
  "name"          varchar(255) NOT NULL,
  "version"       integer      NOT NULL,
  "content_json"  text,
  "changed_by"    varchar(100) NOT NULL DEFAULT '',
  "changed_at"    datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_config_ver" ON "config_versions"("cluster_id", "resource_type", "namespace", "name");

-- -----------------------------------------------------------------------------
-- 27. log_source_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "log_source_configs" (
  "id"         integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id" integer      NOT NULL,
  "type"       varchar(20)  NOT NULL DEFAULT '',
  "name"       varchar(100) NOT NULL DEFAULT '',
  "url"        varchar(255) NOT NULL DEFAULT '',
  "username"   varchar(100) NOT NULL DEFAULT '',
  "password"   varchar(255) NOT NULL DEFAULT '',
  "api_key"    varchar(255) NOT NULL DEFAULT '',
  "enabled"    integer      NOT NULL DEFAULT 1,
  "created_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at" datetime
);
CREATE INDEX IF NOT EXISTS "idx_log_source_configs_cluster_id" ON "log_source_configs"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_log_source_configs_deleted_at" ON "log_source_configs"("deleted_at");

-- -----------------------------------------------------------------------------
-- 28. namespace_protections
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "namespace_protections" (
  "id"               integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"       integer      NOT NULL,
  "namespace"        varchar(253) NOT NULL,
  "require_approval" integer      NOT NULL DEFAULT 0,
  "description"      text,
  "created_at"       datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"       datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"       datetime,
  UNIQUE ("cluster_id", "namespace")
);
CREATE INDEX IF NOT EXISTS "idx_namespace_protections_deleted_at" ON "namespace_protections"("deleted_at");

-- -----------------------------------------------------------------------------
-- 29. approval_requests
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "approval_requests" (
  "id"             integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"     integer      NOT NULL,
  "cluster_name"   text,
  "namespace"      varchar(253) NOT NULL,
  "resource_kind"  text         NOT NULL,
  "resource_name"  text         NOT NULL,
  "action"         text         NOT NULL,
  "requester_id"   integer      NOT NULL,
  "requester_name" text,
  "approver_id"    integer,
  "approver_name"  text,
  "status"         text         NOT NULL DEFAULT 'pending',
  "payload"        text,
  "reason"         text,
  "expires_at"     datetime     NOT NULL,
  "approved_at"    datetime,
  "created_at"     datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"     datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"     datetime
);
CREATE INDEX IF NOT EXISTS "idx_approval_requests_cluster_id" ON "approval_requests"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_approval_requests_namespace"  ON "approval_requests"("namespace");
CREATE INDEX IF NOT EXISTS "idx_approval_requests_deleted_at" ON "approval_requests"("deleted_at");

-- -----------------------------------------------------------------------------
-- 30. image_indices
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "image_indices" (
  "id"             integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"     integer      NOT NULL,
  "cluster_name"   text,
  "namespace"      varchar(253) NOT NULL,
  "workload_kind"  varchar(64)  NOT NULL,
  "workload_name"  varchar(253) NOT NULL,
  "container_name" varchar(253) NOT NULL,
  "image"          varchar(512) NOT NULL,
  "image_name"     varchar(512) NOT NULL,
  "image_tag"      text,
  "last_sync_at"   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "created_at"     datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"     datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"     datetime
);
CREATE INDEX IF NOT EXISTS "idx_image_indices_cluster_id" ON "image_indices"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_image_indices_namespace"  ON "image_indices"("namespace");
CREATE INDEX IF NOT EXISTS "idx_image_indices_deleted_at" ON "image_indices"("deleted_at");

-- -----------------------------------------------------------------------------
-- 31. port_forward_sessions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "port_forward_sessions" (
  "id"           integer  NOT NULL PRIMARY KEY AUTOINCREMENT,
  "cluster_id"   integer  NOT NULL,
  "cluster_name" text,
  "namespace"    text     NOT NULL,
  "pod_name"     text     NOT NULL,
  "pod_port"     integer  NOT NULL,
  "local_port"   integer  NOT NULL,
  "user_id"      integer  NOT NULL,
  "username"     text,
  "status"       text     NOT NULL DEFAULT 'active',
  "stopped_at"   datetime,
  "created_at"   datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"   datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"   datetime
);
CREATE INDEX IF NOT EXISTS "idx_port_forward_sessions_cluster_id" ON "port_forward_sessions"("cluster_id");
CREATE INDEX IF NOT EXISTS "idx_port_forward_sessions_deleted_at" ON "port_forward_sessions"("deleted_at");

-- -----------------------------------------------------------------------------
-- 32. siem_webhook_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "siem_webhook_configs" (
  "id"            integer  NOT NULL PRIMARY KEY AUTOINCREMENT,
  "enabled"       integer  NOT NULL DEFAULT 0,
  "webhook_url"   text     NOT NULL,
  "secret_header" text,
  "secret_value"  text,
  "batch_size"    integer  NOT NULL DEFAULT 100,
  "created_at"    datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"    datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"    datetime
);
CREATE INDEX IF NOT EXISTS "idx_siem_webhook_configs_deleted_at" ON "siem_webhook_configs"("deleted_at");

-- -----------------------------------------------------------------------------
-- 33. api_tokens
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "api_tokens" (
  "id"           integer      NOT NULL PRIMARY KEY AUTOINCREMENT,
  "user_id"      integer      NOT NULL,
  "name"         varchar(100) NOT NULL,
  "token_hash"   varchar(64)  NOT NULL UNIQUE,
  "scopes"       varchar(200) NOT NULL DEFAULT '',
  "expires_at"   datetime,
  "last_used_at" datetime,
  "created_at"   datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS "idx_api_tokens_user_id" ON "api_tokens"("user_id");

-- -----------------------------------------------------------------------------
-- 34. notify_channels
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "notify_channels" (
  "id"               integer       NOT NULL PRIMARY KEY AUTOINCREMENT,
  "name"             varchar(100)  NOT NULL UNIQUE,
  "type"             varchar(20)   NOT NULL,
  "webhook_url"      varchar(1000) NOT NULL,
  "telegram_chat_id" varchar(200)  NOT NULL DEFAULT '',
  "description"      varchar(255)  NOT NULL DEFAULT '',
  "enabled"          integer       NOT NULL DEFAULT 1,
  "created_at"       datetime      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at"       datetime      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "deleted_at"       datetime
);
CREATE INDEX IF NOT EXISTS "idx_notify_channels_deleted_at" ON "notify_channels"("deleted_at");

PRAGMA foreign_keys = ON;

-- =============================================================================
-- 初始資料（Seed Data）
-- =============================================================================

-- 預設管理員（admin / Synapse@2026）
-- 密碼為 bcrypt(cost=10) of "Synapse@2026synapse_salt"
-- ⚠️  首次登入後請立即修改密碼
INSERT OR IGNORE INTO "users"
  (username, password_hash, salt, email, display_name, auth_type, status, created_at, updated_at)
VALUES (
  'admin',
  '$2a$10$4./Tk094GlfLRsNU4nhYQ.FrxyxW/91G2ajMAY/oD0GgGkxEqKSG.',
  'synapse_salt',
  'admin@synapse.io',
  '管理員',
  'local',
  'active',
  datetime('now'),
  datetime('now')
);

-- 預設 LDAP 設定
INSERT OR IGNORE INTO "system_settings" (config_key, value, type, created_at, updated_at)
VALUES (
  'ldap_config',
  '{"enabled":false,"server":"","port":389,"use_tls":false,"skip_tls_verify":false,"bind_dn":"","bind_password":"","base_dn":"","user_filter":"(uid=%s)","username_attr":"uid","email_attr":"mail","display_name_attr":"cn","group_filter":"(memberUid=%s)","group_attr":"cn"}',
  'ldap',
  datetime('now'),
  datetime('now')
);

-- 預設安全設定
INSERT OR IGNORE INTO "system_settings" (config_key, value, type, created_at, updated_at)
VALUES (
  'security_config',
  '{"session_ttl_minutes":480,"login_fail_lock_threshold":5,"lock_duration_minutes":30,"password_min_length":8}',
  'security',
  datetime('now'),
  datetime('now')
);

-- 預設 Grafana 設定（空，需在 UI 中填入）
INSERT OR IGNORE INTO "system_settings" (config_key, value, type, created_at, updated_at)
VALUES (
  'grafana_config',
  '{"url":"","api_key":""}',
  'grafana',
  datetime('now'),
  datetime('now')
);

-- 預設使用者組
INSERT OR IGNORE INTO "user_groups" (name, description, created_at, updated_at)
VALUES
  ('運維組', '運維團隊成員，擁有運維權限', datetime('now'), datetime('now')),
  ('開發組', '開發團隊成員，擁有開發權限', datetime('now'), datetime('now')),
  ('只讀組', '只讀權限使用者組',           datetime('now'), datetime('now'));

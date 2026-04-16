-- Migration 001: Consolidated baseline schema.
-- All tables from migrations 001–013 are merged here.
-- Drop and recreate the DB to apply this fresh baseline.
-- Development only — no backward-compatible ALTER TABLE guards needed.


-- ── users ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
  id            bigserial       PRIMARY KEY,
  username      varchar(50)     NOT NULL,
  password_hash varchar(255)    NOT NULL DEFAULT '',
  salt          varchar(32)     NOT NULL DEFAULT '',
  email         varchar(100)    NOT NULL DEFAULT '',
  display_name  varchar(100)    NOT NULL DEFAULT '',
  phone         varchar(20)     NOT NULL DEFAULT '',
  auth_type     varchar(20)     NOT NULL DEFAULT 'local',
  status        varchar(20)     NOT NULL DEFAULT 'active',
  system_role   varchar(32)     NOT NULL DEFAULT 'user',
  last_login_at timestamptz(3)  NULL,
  last_login_ip varchar(50)     NOT NULL DEFAULT '',
  created_at    timestamptz(3)  NOT NULL,
  updated_at    timestamptz(3)  NOT NULL,
  deleted_at    timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username     ON users (username);
CREATE INDEX        IF NOT EXISTS idx_users_system_role  ON users (system_role);
CREATE INDEX        IF NOT EXISTS idx_users_deleted_at   ON users (deleted_at);

-- ── user_groups ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_groups (
  id          bigserial       PRIMARY KEY,
  name        varchar(50)     NOT NULL,
  description varchar(255)    NOT NULL DEFAULT '',
  created_at  timestamptz(3)  NOT NULL,
  updated_at  timestamptz(3)  NOT NULL,
  deleted_at  timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_groups_name       ON user_groups (name);
CREATE INDEX        IF NOT EXISTS idx_user_groups_deleted_at ON user_groups (deleted_at);

-- ── user_group_members (many2many) ───────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_group_members (
  user_id       bigint NOT NULL,
  user_group_id bigint NOT NULL,
  PRIMARY KEY (user_id, user_group_id)
);

-- ── clusters ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS clusters (
  id                   bigserial       PRIMARY KEY,
  name                 varchar(100)    NOT NULL,
  api_server           varchar(255)    NOT NULL,
  kubeconfig_enc       text,
  ca_enc               text,
  sa_token_enc         text,
  version              varchar(50)     NOT NULL DEFAULT '',
  status               varchar(20)     NOT NULL DEFAULT 'unknown',
  labels               jsonb           DEFAULT NULL,
  cert_expire_at       timestamptz(3)  NULL,
  last_heartbeat       timestamptz(3)  NULL,
  created_by           bigint          NOT NULL DEFAULT 0,
  monitoring_config    jsonb           DEFAULT NULL,
  alert_manager_config jsonb           DEFAULT NULL,
  created_at           timestamptz(3)  NOT NULL,
  updated_at           timestamptz(3)  NOT NULL,
  deleted_at           timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_name       ON clusters (name);
CREATE INDEX        IF NOT EXISTS idx_clusters_deleted_at ON clusters (deleted_at);

-- ── cluster_metrics ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS cluster_metrics (
  cluster_id    bigint           NOT NULL,
  node_count    bigint           NOT NULL DEFAULT 0,
  ready_nodes   bigint           NOT NULL DEFAULT 0,
  pod_count     bigint           NOT NULL DEFAULT 0,
  running_pods  bigint           NOT NULL DEFAULT 0,
  cpu_usage     double precision NOT NULL DEFAULT 0,
  memory_usage  double precision NOT NULL DEFAULT 0,
  storage_usage double precision NOT NULL DEFAULT 0,
  updated_at    timestamptz(3)   NOT NULL,
  PRIMARY KEY (cluster_id)
);

-- ── cluster_permissions ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS cluster_permissions (
  id              bigserial       PRIMARY KEY,
  cluster_id      bigint          NOT NULL,
  user_id         bigint          NULL,
  user_group_id   bigint          NULL,
  permission_type varchar(50)     NOT NULL,
  namespaces      text,
  custom_role_ref varchar(200)    NOT NULL DEFAULT '',
  feature_policy  text            NULL,      -- from 007
  created_at      timestamptz(3)  NOT NULL,
  updated_at      timestamptz(3)  NOT NULL,
  deleted_at      timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_cluster_id    ON cluster_permissions (cluster_id);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_user_id       ON cluster_permissions (user_id);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_user_group_id ON cluster_permissions (user_group_id);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_deleted_at    ON cluster_permissions (deleted_at);

-- ── terminal_sessions ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS terminal_sessions (
  id          bigserial       PRIMARY KEY,
  user_id     bigint          NOT NULL,
  cluster_id  bigint          NOT NULL,
  target_type varchar(20)     NOT NULL,
  target_ref  jsonb           DEFAULT NULL,
  namespace   varchar(100)    NOT NULL DEFAULT '',
  pod         varchar(100)    NOT NULL DEFAULT '',
  container   varchar(100)    NOT NULL DEFAULT '',
  node        varchar(100)    NOT NULL DEFAULT '',
  start_at    timestamptz(3)  NOT NULL,
  end_at      timestamptz(3)  NULL,
  input_size  bigint          NOT NULL DEFAULT 0,
  status      varchar(20)     NOT NULL DEFAULT 'active',
  created_at  timestamptz(3)  NOT NULL,
  updated_at  timestamptz(3)  NOT NULL,
  deleted_at  timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_deleted_at ON terminal_sessions (deleted_at);

-- ── terminal_commands ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS terminal_commands (
  id         bigserial       PRIMARY KEY,
  session_id bigint          NOT NULL,
  timestamp  timestamptz(3)  NOT NULL,
  raw_input  text,
  parsed_cmd varchar(1024)   NOT NULL DEFAULT '',
  exit_code  bigint          NULL,
  created_at timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_terminal_commands_session_id ON terminal_commands (session_id);

-- ── audit_logs ───────────────────────────────────────────────────────────────
-- resource_ref is TEXT (not jsonb) — code inserts plain string identifiers.
CREATE TABLE IF NOT EXISTS audit_logs (
  id            bigserial       PRIMARY KEY,
  user_id       bigint          NOT NULL,
  action        varchar(100)    NOT NULL,
  resource_type varchar(50)     NOT NULL,
  resource_ref  text            DEFAULT NULL,
  result        varchar(20)     NOT NULL,
  ip            varchar(45)     NOT NULL DEFAULT '',
  user_agent    varchar(500)    NOT NULL DEFAULT '',
  details       text,
  prev_hash     varchar(64)     NOT NULL DEFAULT '',
  hash          varchar(64)     NOT NULL DEFAULT '',
  created_at    timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_hash    ON audit_logs (hash);

-- ── operation_logs ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS operation_logs (
  id            bigserial       PRIMARY KEY,
  user_id       bigint          NULL,
  username      varchar(100)    NOT NULL DEFAULT '',
  method        varchar(10)     NOT NULL DEFAULT '',
  path          varchar(500)    NOT NULL DEFAULT '',
  query         varchar(1000)   NOT NULL DEFAULT '',
  module        varchar(50)     NOT NULL DEFAULT '',
  action        varchar(100)    NOT NULL DEFAULT '',
  cluster_id    bigint          NULL,
  cluster_name  varchar(100)    NOT NULL DEFAULT '',
  namespace     varchar(100)    NOT NULL DEFAULT '',
  resource_type varchar(50)     NOT NULL DEFAULT '',
  resource_name varchar(200)    NOT NULL DEFAULT '',
  request_body  text,
  status_code   bigint          NOT NULL DEFAULT 0,
  success       boolean         NOT NULL DEFAULT false,
  error_message varchar(1000)   NOT NULL DEFAULT '',
  client_ip     varchar(45)     NOT NULL DEFAULT '',
  user_agent    varchar(500)    NOT NULL DEFAULT '',
  duration      bigint          NOT NULL DEFAULT 0,
  created_at    timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_op_user_time              ON operation_logs (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_operation_logs_username   ON operation_logs (username);
CREATE INDEX IF NOT EXISTS idx_operation_logs_method     ON operation_logs (method);
CREATE INDEX IF NOT EXISTS idx_operation_logs_module     ON operation_logs (module);
CREATE INDEX IF NOT EXISTS idx_operation_logs_action     ON operation_logs (action);
CREATE INDEX IF NOT EXISTS idx_operation_logs_cluster_id ON operation_logs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_success    ON operation_logs (success);

-- ── system_settings ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS system_settings (
  id         bigserial       PRIMARY KEY,
  config_key varchar(100)    NOT NULL,
  value      text,
  type       varchar(50)     NOT NULL DEFAULT '',
  created_at timestamptz(3)  NOT NULL,
  updated_at timestamptz(3)  NOT NULL,
  deleted_at timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_system_settings_config_key ON system_settings (config_key);
CREATE INDEX        IF NOT EXISTS idx_system_settings_deleted_at ON system_settings (deleted_at);

-- ── argocd_configs ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS argocd_configs (
  id                   bigserial       PRIMARY KEY,
  cluster_id           bigint          NOT NULL,
  enabled              boolean         NOT NULL DEFAULT false,
  server_url           varchar(255)    NOT NULL DEFAULT '',
  auth_type            varchar(20)     NOT NULL DEFAULT '',
  token                text,
  username             varchar(100)    NOT NULL DEFAULT '',
  password             text,
  insecure             boolean         NOT NULL DEFAULT false,
  git_repo_url         varchar(500)    NOT NULL DEFAULT '',
  git_branch           varchar(100)    NOT NULL DEFAULT 'main',
  git_path             varchar(255)    NOT NULL DEFAULT '',
  git_auth_type        varchar(20)     NOT NULL DEFAULT '',
  git_username         varchar(100)    NOT NULL DEFAULT '',
  git_password         text,
  git_ssh_key          text,
  argo_cd_cluster_name varchar(100)    NOT NULL DEFAULT '',
  argo_cd_project      varchar(100)    NOT NULL DEFAULT 'default',
  connection_status    varchar(20)     NOT NULL DEFAULT '',
  last_test_at         timestamptz(3)  NULL,
  error_message        text,
  created_at           timestamptz(3)  NOT NULL,
  updated_at           timestamptz(3)  NOT NULL,
  deleted_at           timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_argocd_configs_cluster_id ON argocd_configs (cluster_id);
CREATE INDEX        IF NOT EXISTS idx_argocd_configs_deleted_at ON argocd_configs (deleted_at);

-- ── ai_configs ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ai_configs (
  id          bigserial       PRIMARY KEY,
  provider    varchar(50)     NOT NULL DEFAULT 'openai',
  endpoint    varchar(255)    NOT NULL DEFAULT '',
  api_key     text,
  model       varchar(100)    NOT NULL DEFAULT '',
  api_version varchar(50)     NOT NULL DEFAULT '',
  enabled     boolean         NOT NULL DEFAULT false,
  created_at  timestamptz(3)  NOT NULL,
  updated_at  timestamptz(3)  NOT NULL,
  deleted_at  timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_configs_deleted_at ON ai_configs (deleted_at);

-- ── helm_repositories ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS helm_repositories (
  id         bigserial       PRIMARY KEY,
  created_at timestamptz(3)  NOT NULL,
  updated_at timestamptz(3)  NOT NULL,
  deleted_at timestamptz(3)  NULL,
  name       varchar(128)    NOT NULL DEFAULT '',
  url        varchar(512)    NOT NULL DEFAULT '',
  username   varchar(256)    NOT NULL DEFAULT '',
  password   varchar(256)    NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_helm_repositories_name       ON helm_repositories (name);
CREATE INDEX        IF NOT EXISTS idx_helm_repositories_deleted_at ON helm_repositories (deleted_at);

-- ── event_alert_rules ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS event_alert_rules (
  id           bigserial       PRIMARY KEY,
  cluster_id   bigint          NOT NULL,
  name         varchar(100)    NOT NULL,
  description  varchar(255)    NOT NULL DEFAULT '',
  namespace    varchar(100)    NOT NULL DEFAULT '',
  event_reason varchar(100)    NOT NULL DEFAULT '',
  event_type   varchar(20)     NOT NULL DEFAULT '',
  min_count    bigint          NOT NULL DEFAULT 1,
  notify_type  varchar(20)     NOT NULL DEFAULT '',
  notify_url   varchar(500)    NOT NULL DEFAULT '',
  enabled      boolean         NOT NULL DEFAULT true,
  created_at   timestamptz(3)  NOT NULL,
  updated_at   timestamptz(3)  NOT NULL,
  deleted_at   timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_event_alert_rules_cluster_id ON event_alert_rules (cluster_id);
CREATE INDEX IF NOT EXISTS idx_event_alert_rules_deleted_at ON event_alert_rules (deleted_at);

-- ── event_alert_histories ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS event_alert_histories (
  id            bigserial       PRIMARY KEY,
  rule_id       bigint          NOT NULL,
  cluster_id    bigint          NOT NULL,
  rule_name     varchar(100)    NOT NULL DEFAULT '',
  namespace     varchar(100)    NOT NULL DEFAULT '',
  event_reason  varchar(100)    NOT NULL DEFAULT '',
  event_type    varchar(20)     NOT NULL DEFAULT '',
  message       text,
  involved_obj  varchar(200)    NOT NULL DEFAULT '',
  notify_result varchar(50)     NOT NULL DEFAULT '',
  is_read       boolean         NOT NULL DEFAULT false,
  triggered_at  timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_rule_id      ON event_alert_histories (rule_id);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_cluster_id   ON event_alert_histories (cluster_id);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_is_read      ON event_alert_histories (is_read);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_triggered_at ON event_alert_histories (triggered_at);

-- ── cost_configs ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS cost_configs (
  id                 bigserial        PRIMARY KEY,
  cluster_id         bigint           NOT NULL,
  cpu_price_per_core double precision NOT NULL DEFAULT 0.048,
  mem_price_per_gi_b double precision NOT NULL DEFAULT 0.006,
  currency           varchar(10)      NOT NULL DEFAULT 'USD',
  updated_at         timestamptz(3)   NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cost_configs_cluster_id ON cost_configs (cluster_id);

-- ── resource_snapshots ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS resource_snapshots (
  id          bigserial        PRIMARY KEY,
  cluster_id  bigint           NOT NULL,
  namespace   varchar(128)     NOT NULL,
  workload    varchar(256)     NOT NULL,
  date        timestamptz(3)   NOT NULL,
  cpu_request double precision NOT NULL DEFAULT 0,
  cpu_usage   double precision NOT NULL DEFAULT 0,
  mem_request double precision NOT NULL DEFAULT 0,
  mem_usage   double precision NOT NULL DEFAULT 0,
  pod_count   bigint           NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_resource_snapshots_cluster_id ON resource_snapshots (cluster_id);
CREATE INDEX IF NOT EXISTS idx_resource_snapshots_date       ON resource_snapshots (date);

-- ── cluster_occupancy_snapshots ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS cluster_occupancy_snapshots (
  id                 bigserial        PRIMARY KEY,
  cluster_id         bigint           NOT NULL,
  date               timestamptz(3)   NOT NULL,
  allocatable_cpu    double precision NOT NULL DEFAULT 0,
  allocatable_memory double precision NOT NULL DEFAULT 0,
  requested_cpu      double precision NOT NULL DEFAULT 0,
  requested_memory   double precision NOT NULL DEFAULT 0,
  node_count         bigint           NOT NULL DEFAULT 0,
  pod_count          bigint           NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cluster_date ON cluster_occupancy_snapshots (cluster_id, date);

-- ── cloud_billing_configs ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS cloud_billing_configs (
  id                     bigserial       PRIMARY KEY,
  cluster_id             bigint          NOT NULL,
  provider               varchar(20)     NOT NULL DEFAULT 'disabled',
  aws_access_key_id      varchar(128)    NOT NULL DEFAULT '',
  aws_secret_access_key  varchar(256)    NOT NULL DEFAULT '',
  aws_region             varchar(32)     NOT NULL DEFAULT 'us-east-1',
  aws_linked_account_id  varchar(20)     NOT NULL DEFAULT '',
  gcp_project_id         varchar(128)    NOT NULL DEFAULT '',
  gcp_billing_account_id varchar(64)     NOT NULL DEFAULT '',
  gcp_service_account_json text,
  last_synced_at         timestamptz(3)  NULL,
  last_error             varchar(512)    NOT NULL DEFAULT '',
  created_at             timestamptz(3)  NOT NULL,
  updated_at             timestamptz(3)  NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cloud_billing_configs_cluster_id ON cloud_billing_configs (cluster_id);

-- ── cloud_billing_records ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS cloud_billing_records (
  id         bigserial        PRIMARY KEY,
  cluster_id bigint           NOT NULL,
  month      varchar(7)       NOT NULL,
  provider   varchar(20)      NOT NULL DEFAULT '',
  service    varchar(256)     NOT NULL DEFAULT '',
  amount     double precision NOT NULL DEFAULT 0,
  currency   varchar(10)      NOT NULL DEFAULT 'USD',
  created_at timestamptz(3)   NOT NULL,
  updated_at timestamptz(3)   NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cloud_billing_records_cluster_id ON cloud_billing_records (cluster_id);
CREATE INDEX IF NOT EXISTS idx_cloud_billing_records_month      ON cloud_billing_records (month);

-- ── image_scan_results ───────────────────────────────────────────────────────
-- Includes pipeline source tracking columns from migration 008.
CREATE TABLE IF NOT EXISTS image_scan_results (
  id             bigserial       PRIMARY KEY,
  cluster_id     bigint          NOT NULL,
  namespace      varchar(100)    NOT NULL DEFAULT '',
  pod_name       varchar(255)    NOT NULL DEFAULT '',
  container_name varchar(255)    NOT NULL DEFAULT '',
  image          varchar(512)    NOT NULL,
  status         varchar(20)     NOT NULL DEFAULT 'pending',
  critical       bigint          NOT NULL DEFAULT 0,
  high           bigint          NOT NULL DEFAULT 0,
  medium         bigint          NOT NULL DEFAULT 0,
  low            bigint          NOT NULL DEFAULT 0,
  unknown        bigint          NOT NULL DEFAULT 0,
  result_json    text,
  error          varchar(512)    NOT NULL DEFAULT '',
  scanned_at     timestamptz(3)  NULL,
  scan_source    varchar(20)     DEFAULT 'manual',
  pipeline_run_id bigint,
  step_run_id    bigint,
  created_at     timestamptz(3)  NOT NULL,
  updated_at     timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_image_scan_results_cluster_id   ON image_scan_results (cluster_id);
CREATE INDEX IF NOT EXISTS idx_scan_results_pipeline_run
    ON image_scan_results (pipeline_run_id) WHERE pipeline_run_id IS NOT NULL;

-- ── bench_results ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS bench_results (
  id          bigserial        PRIMARY KEY,
  cluster_id  bigint           NOT NULL,
  status      varchar(20)      NOT NULL DEFAULT 'pending',
  pass        bigint           NOT NULL DEFAULT 0,
  fail        bigint           NOT NULL DEFAULT 0,
  warn        bigint           NOT NULL DEFAULT 0,
  info        bigint           NOT NULL DEFAULT 0,
  score       double precision NOT NULL DEFAULT 0,
  result_json text,
  error       varchar(512)     NOT NULL DEFAULT '',
  job_name    varchar(255)     NOT NULL DEFAULT '',
  run_at      timestamptz(3)   NULL,
  created_at  timestamptz(3)   NOT NULL,
  updated_at  timestamptz(3)   NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bench_results_cluster_id ON bench_results (cluster_id);

-- ── sync_policies ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS sync_policies (
  id                bigserial       PRIMARY KEY,
  name              varchar(128)    NOT NULL,
  description       varchar(512)    NOT NULL DEFAULT '',
  source_cluster_id bigint          NOT NULL,
  source_namespace  varchar(128)    NOT NULL,
  resource_type     varchar(32)     NOT NULL,
  resource_names    text,
  target_clusters   text,
  conflict_policy   varchar(16)     NOT NULL DEFAULT 'skip',
  schedule          varchar(64)     NOT NULL DEFAULT '',
  enabled           boolean         NOT NULL DEFAULT true,
  last_sync_at      timestamptz(3)  NULL,
  last_sync_status  varchar(16)     NOT NULL DEFAULT '',
  created_at        timestamptz(3)  NOT NULL,
  updated_at        timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sync_policies_source_cluster_id ON sync_policies (source_cluster_id);

-- ── sync_histories ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS sync_histories (
  id           bigserial       PRIMARY KEY,
  policy_id    bigint          NOT NULL,
  triggered_by varchar(64)     NOT NULL DEFAULT '',
  status       varchar(16)     NOT NULL DEFAULT '',
  message      text,
  details      text,
  started_at   timestamptz(3)  NOT NULL,
  finished_at  timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_sync_histories_policy_id ON sync_histories (policy_id);

-- ── config_versions ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS config_versions (
  id            bigserial       PRIMARY KEY,
  cluster_id    bigint          NOT NULL,
  resource_type varchar(20)     NOT NULL,
  namespace     varchar(255)    NOT NULL,
  name          varchar(255)    NOT NULL,
  version       bigint          NOT NULL,
  content_json  text,
  changed_by    varchar(100)    NOT NULL DEFAULT '',
  changed_at    timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_config_ver ON config_versions (cluster_id, resource_type, namespace, name);

-- ── namespace_protections ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS namespace_protections (
  id               bigserial       PRIMARY KEY,
  created_at       timestamptz(3)  NOT NULL,
  updated_at       timestamptz(3)  NOT NULL,
  deleted_at       timestamptz(3)  NULL,
  cluster_id       bigint          NOT NULL,
  namespace        varchar(253)    NOT NULL,
  require_approval boolean         NOT NULL DEFAULT false,
  description      text
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ns_protection                  ON namespace_protections (cluster_id, namespace);
CREATE INDEX        IF NOT EXISTS idx_namespace_protections_deleted_at ON namespace_protections (deleted_at);

-- ── approval_requests ────────────────────────────────────────────────────────
-- Includes pipeline deployment gate columns from migration 008.
CREATE TABLE IF NOT EXISTS approval_requests (
  id               bigserial       PRIMARY KEY,
  created_at       timestamptz(3)  NOT NULL,
  updated_at       timestamptz(3)  NOT NULL,
  deleted_at       timestamptz(3)  NULL,
  cluster_id       bigint          NOT NULL,
  cluster_name     varchar(255)    NOT NULL DEFAULT '',
  namespace        varchar(253)    NOT NULL,
  resource_kind    varchar(255)    NOT NULL,
  resource_name    varchar(255)    NOT NULL,
  action           varchar(255)    NOT NULL,
  requester_id     bigint          NOT NULL,
  requester_name   varchar(255)    NOT NULL DEFAULT '',
  approver_id      bigint          NULL,
  approver_name    varchar(255)    NOT NULL DEFAULT '',
  status           varchar(255)    NOT NULL DEFAULT 'pending',
  payload          text,
  reason           text,
  expires_at       timestamptz(3)  NOT NULL,
  approved_at      timestamptz(3)  NULL,
  pipeline_run_id  bigint,
  from_environment varchar(100),
  to_environment   varchar(100)
);
CREATE INDEX IF NOT EXISTS idx_approval_requests_cluster_id  ON approval_requests (cluster_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_namespace   ON approval_requests (namespace);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status      ON approval_requests (status);
CREATE INDEX IF NOT EXISTS idx_approval_requests_deleted_at  ON approval_requests (deleted_at);
CREATE INDEX IF NOT EXISTS idx_approval_requests_pipeline_run
    ON approval_requests (pipeline_run_id) WHERE pipeline_run_id IS NOT NULL;

-- ── port_forward_sessions ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS port_forward_sessions (
  id           bigserial       PRIMARY KEY,
  created_at   timestamptz(3)  NOT NULL,
  updated_at   timestamptz(3)  NOT NULL,
  deleted_at   timestamptz(3)  NULL,
  cluster_id   bigint          NOT NULL,
  cluster_name varchar(255)    NOT NULL DEFAULT '',
  namespace    varchar(255)    NOT NULL,
  pod_name     varchar(255)    NOT NULL,
  pod_port     bigint          NOT NULL,
  local_port   bigint          NOT NULL,
  user_id      bigint          NOT NULL,
  username     varchar(255)    NOT NULL DEFAULT '',
  status       varchar(255)    NOT NULL DEFAULT 'active',
  stopped_at   timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_port_forward_sessions_cluster_id ON port_forward_sessions (cluster_id);
CREATE INDEX IF NOT EXISTS idx_port_forward_sessions_deleted_at ON port_forward_sessions (deleted_at);

-- ── siem_webhook_configs ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS siem_webhook_configs (
  id            bigserial       PRIMARY KEY,
  created_at    timestamptz(3)  NOT NULL,
  updated_at    timestamptz(3)  NOT NULL,
  deleted_at    timestamptz(3)  NULL,
  enabled       boolean         NOT NULL DEFAULT false,
  webhook_url   varchar(255)    NOT NULL,
  secret_header varchar(255)    NOT NULL DEFAULT '',
  secret_value  varchar(255)    NOT NULL DEFAULT '',
  batch_size    bigint          NOT NULL DEFAULT 100
);
CREATE INDEX IF NOT EXISTS idx_siem_webhook_configs_deleted_at ON siem_webhook_configs (deleted_at);

-- ── api_tokens ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS api_tokens (
  id           bigserial       PRIMARY KEY,
  user_id      bigint          NOT NULL,
  name         varchar(100)    NOT NULL,
  token_hash   varchar(64)     NOT NULL,
  scopes       varchar(200)    NOT NULL DEFAULT '',
  expires_at   timestamptz(3)  NULL,
  last_used_at timestamptz(3)  NULL,
  created_at   timestamptz(3)  NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens (token_hash);
CREATE INDEX        IF NOT EXISTS idx_api_tokens_user_id    ON api_tokens (user_id);

-- ── notify_channels ──────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS notify_channels (
  id               bigserial       PRIMARY KEY,
  name             varchar(100)    NOT NULL,
  type             varchar(20)     NOT NULL,
  webhook_url      varchar(1000)   NOT NULL,
  telegram_chat_id varchar(200)    NOT NULL DEFAULT '',
  description      varchar(255)    NOT NULL DEFAULT '',
  enabled          boolean         NOT NULL DEFAULT true,
  created_at       timestamptz(3)  NOT NULL,
  updated_at       timestamptz(3)  NOT NULL,
  deleted_at       timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notify_channels_name       ON notify_channels (name);
CREATE INDEX        IF NOT EXISTS idx_notify_channels_deleted_at ON notify_channels (deleted_at);

-- ── log_source_configs ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS log_source_configs (
  id         bigserial       PRIMARY KEY,
  cluster_id bigint          NOT NULL,
  type       varchar(20)     NOT NULL DEFAULT '',
  name       varchar(100)    NOT NULL DEFAULT '',
  url        varchar(255)    NOT NULL DEFAULT '',
  username   varchar(100)    NOT NULL DEFAULT '',
  password   varchar(255)    NOT NULL DEFAULT '',
  api_key    varchar(255)    NOT NULL DEFAULT '',
  enabled    boolean         NOT NULL DEFAULT true,
  created_at timestamptz(3)  NOT NULL,
  updated_at timestamptz(3)  NOT NULL,
  deleted_at timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_log_source_configs_cluster_id ON log_source_configs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_log_source_configs_deleted_at ON log_source_configs (deleted_at);

-- ── image_indices ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS image_indices (
  id             bigserial       PRIMARY KEY,
  created_at     timestamptz(3)  NOT NULL,
  updated_at     timestamptz(3)  NOT NULL,
  deleted_at     timestamptz(3)  NULL,
  cluster_id     bigint          NOT NULL,
  cluster_name   varchar(255)    NOT NULL DEFAULT '',
  namespace      varchar(253)    NOT NULL,
  workload_kind  varchar(64)     NOT NULL,
  workload_name  varchar(253)    NOT NULL,
  container_name varchar(253)    NOT NULL,
  image          varchar(512)    NOT NULL,
  image_name     varchar(512)    NOT NULL,
  image_tag      varchar(255)    NOT NULL DEFAULT '',
  last_sync_at   timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_image_indices_cluster_id  ON image_indices (cluster_id);
CREATE INDEX IF NOT EXISTS idx_image_indices_namespace   ON image_indices (namespace);
CREATE INDEX IF NOT EXISTS idx_image_indices_image       ON image_indices (image);
CREATE INDEX IF NOT EXISTS idx_image_indices_image_name  ON image_indices (image_name);
CREATE INDEX IF NOT EXISTS idx_image_indices_deleted_at  ON image_indices (deleted_at);

-- ── token_blacklists ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS token_blacklists (
  id         bigserial       PRIMARY KEY,
  jti        varchar(64)     NOT NULL,
  user_id    bigint          NOT NULL DEFAULT 0,
  reason     varchar(64)     NOT NULL DEFAULT '',
  expires_at timestamptz(3)  NOT NULL,
  created_at timestamptz(3)  NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_token_blacklists_jti        ON token_blacklists (jti);
CREATE INDEX        IF NOT EXISTS idx_token_blacklists_user_id    ON token_blacklists (user_id);
CREATE INDEX        IF NOT EXISTS idx_token_blacklists_expires_at ON token_blacklists (expires_at);

-- ── feature_flags ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS feature_flags (
  key         varchar(100)    NOT NULL,
  enabled     boolean         NOT NULL DEFAULT false,
  description varchar(500)    NOT NULL DEFAULT '',
  updated_by  varchar(100)    NOT NULL DEFAULT '',
  created_at  timestamptz(3)  NOT NULL,
  updated_at  timestamptz(3)  NOT NULL,
  PRIMARY KEY (key)
);

INSERT INTO feature_flags (key, enabled, description, created_at, updated_at) VALUES
  ('use_repo_layer',         false, 'P0-4: Route DB access through Repository layer instead of raw *gorm.DB in services', NOW(), NOW()),
  ('use_split_router',       false, 'P1-2: Use the split router module structure',                                         NOW(), NOW()),
  ('enable_otel_tracing',    false, 'P1-10: Enable OpenTelemetry distributed tracing',                                     NOW(), NOW()),
  ('use_redis_ratelimit',    false, 'P1-8: Use Redis-backed rate limiter for multi-pod deployments',                       NOW(), NOW()),
  ('use_zustand_store',      true,  'P2-5: Zustand frontend global state (session/cluster/UI)',                            NOW(), NOW()),
  ('enable_audit_hashchain', false, 'P2-2: Enable SHA-256 audit log hash-chain verification',                             NOW(), NOW())
ON CONFLICT DO NOTHING;

-- ── namespace_budgets ────────────────────────────────────────────────────────
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

-- ── slos ─────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS slos (
  id                 bigserial        PRIMARY KEY,
  cluster_id         bigint           NOT NULL,
  name               varchar(255)     NOT NULL,
  description        varchar(1024)    NOT NULL DEFAULT '',
  namespace          varchar(128)     NOT NULL DEFAULT '',
  sli_type           varchar(32)      NOT NULL,
  prom_query         text             NOT NULL,
  total_query        text,
  target             double precision NOT NULL,
  "window"           varchar(16)      NOT NULL,
  burn_rate_warning  double precision NOT NULL DEFAULT 2,
  burn_rate_critical double precision NOT NULL DEFAULT 10,
  enabled            boolean          NOT NULL DEFAULT true,
  created_at         timestamptz(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         timestamptz(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at         timestamptz(3)
);
CREATE INDEX IF NOT EXISTS idx_slos_cluster_id ON slos (cluster_id);
CREATE INDEX IF NOT EXISTS idx_slos_namespace  ON slos (namespace);
CREATE INDEX IF NOT EXISTS idx_slos_deleted_at ON slos (deleted_at);

-- ── registries ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS registries (
  id              bigserial     PRIMARY KEY,
  name            varchar(255)  NOT NULL,
  type            varchar(50)   NOT NULL,
  url             varchar(512)  NOT NULL,
  username        varchar(255),
  password_enc    text,
  insecure_tls    boolean       DEFAULT false,
  ca_bundle_enc   text,
  default_project varchar(255),
  enabled         boolean       DEFAULT true,
  created_by      bigint        NOT NULL,
  created_at      timestamptz   DEFAULT now(),
  updated_at      timestamptz   DEFAULT now(),
  deleted_at      timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_registries_name       ON registries (name) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_registries_type       ON registries (type);
CREATE INDEX        IF NOT EXISTS idx_registries_deleted_at ON registries (deleted_at);

-- ── git_providers ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS git_providers (
  id                 bigserial     PRIMARY KEY,
  name               varchar(255)  NOT NULL,
  type               varchar(50)   NOT NULL,
  base_url           varchar(512)  NOT NULL,
  access_token_enc   text,
  webhook_secret_enc text,
  webhook_token      varchar(64)   NOT NULL,
  enabled            boolean       NOT NULL DEFAULT true,
  created_by         bigint        NOT NULL,
  created_at         timestamptz   NOT NULL DEFAULT NOW(),
  updated_at         timestamptz   NOT NULL DEFAULT NOW(),
  deleted_at         timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_git_providers_name          ON git_providers (name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_git_providers_webhook_token ON git_providers (webhook_token);
CREATE INDEX        IF NOT EXISTS idx_git_providers_deleted_at    ON git_providers (deleted_at);

-- ── projects ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS projects (
  id              bigserial     PRIMARY KEY,
  git_provider_id bigint        NOT NULL REFERENCES git_providers(id) ON DELETE CASCADE,
  name            varchar(255)  NOT NULL,
  repo_url        varchar(512)  NOT NULL,
  default_branch  varchar(255)  NOT NULL DEFAULT 'main',
  description     text,
  created_by      bigint        NOT NULL,
  created_at      timestamptz   NOT NULL DEFAULT NOW(),
  updated_at      timestamptz   NOT NULL DEFAULT NOW(),
  deleted_at      timestamptz,
  UNIQUE (repo_url)
);
CREATE INDEX IF NOT EXISTS idx_projects_git_provider_id ON projects (git_provider_id);
CREATE INDEX IF NOT EXISTS idx_projects_deleted_at       ON projects (deleted_at);

-- ── ci_engine_configs ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS ci_engine_configs (
  id                   bigserial     PRIMARY KEY,
  name                 varchar(100)  NOT NULL,
  engine_type          varchar(20)   NOT NULL,
  enabled              boolean       NOT NULL DEFAULT true,
  endpoint             varchar(500),
  auth_type            varchar(20),
  username             varchar(100),
  token                text,
  password             text,
  webhook_secret       text,
  cluster_id           bigint,
  extra_json           text,
  insecure_skip_verify boolean       NOT NULL DEFAULT false,
  ca_bundle            text,
  last_checked_at      timestamptz,
  last_healthy         boolean       NOT NULL DEFAULT false,
  last_version         varchar(50),
  last_error           text,
  created_by           bigint        NOT NULL,
  created_at           timestamptz   NOT NULL DEFAULT NOW(),
  updated_at           timestamptz   NOT NULL DEFAULT NOW(),
  deleted_at           timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ci_engine_name              ON ci_engine_configs (name) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_ci_engine_configs_engine_type ON ci_engine_configs (engine_type);
CREATE INDEX        IF NOT EXISTS idx_ci_engine_configs_cluster_id  ON ci_engine_configs (cluster_id);
CREATE INDEX        IF NOT EXISTS idx_ci_engine_configs_deleted_at  ON ci_engine_configs (deleted_at);

-- ── gitops_apps ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS gitops_apps (
  id               bigserial     PRIMARY KEY,
  name             varchar(255)  NOT NULL,
  source           varchar(20)   NOT NULL DEFAULT 'native',
  git_provider_id  bigint,
  repo_url         varchar(512),
  branch           varchar(255),
  path             varchar(512),
  render_type      varchar(50)   NOT NULL DEFAULT 'raw',
  helm_values      text,
  cluster_id       bigint        NOT NULL,
  namespace        varchar(253)  NOT NULL,
  sync_policy      varchar(50)   NOT NULL DEFAULT 'manual',
  sync_interval    int           DEFAULT 300,
  last_synced_at   timestamptz,
  last_diff_at     timestamptz,
  last_diff_result text,
  status           varchar(50)   NOT NULL DEFAULT 'unknown',
  status_message   text,
  created_by       bigint        NOT NULL,
  created_at       timestamptz   DEFAULT now(),
  updated_at       timestamptz   DEFAULT now(),
  deleted_at       timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gitops_name_cluster ON gitops_apps (name, cluster_id) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_gitops_source       ON gitops_apps (source);
CREATE INDEX        IF NOT EXISTS idx_gitops_status       ON gitops_apps (status);
CREATE INDEX        IF NOT EXISTS idx_gitops_cluster      ON gitops_apps (cluster_id);
CREATE INDEX        IF NOT EXISTS idx_gitops_deleted_at   ON gitops_apps (deleted_at);

-- ── pipelines ────────────────────────────────────────────────────────────────
-- engine_type / engine_config_id / project_id included from 013/012.
-- current_version_id FK is deferred below (circular dep with pipeline_versions).
CREATE TABLE IF NOT EXISTS pipelines (
  id                 bigserial     PRIMARY KEY,
  name               varchar(255)  NOT NULL,
  description        text,
  current_version_id bigint,
  concurrency_group  varchar(255),
  concurrency_policy varchar(30)   DEFAULT 'cancel_previous',
  max_concurrent_runs int          DEFAULT 1,
  notify_on_success  jsonb,
  notify_on_failure  jsonb,
  notify_on_scan     jsonb,
  engine_type        varchar(20)   NOT NULL DEFAULT 'native',
  engine_config_id   bigint        REFERENCES ci_engine_configs(id) ON DELETE SET NULL,
  project_id         bigint        REFERENCES projects(id) ON DELETE SET NULL,
  created_by         bigint        NOT NULL,
  created_at         timestamptz   DEFAULT now(),
  updated_at         timestamptz   DEFAULT now(),
  deleted_at         timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_name      ON pipelines (name) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_pipelines_deleted_at    ON pipelines (deleted_at);
CREATE INDEX        IF NOT EXISTS idx_pipelines_project_id    ON pipelines (project_id);
CREATE INDEX        IF NOT EXISTS idx_pipelines_engine_config ON pipelines (engine_config_id);

-- ── pipeline_versions ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS pipeline_versions (
  id             bigserial     PRIMARY KEY,
  pipeline_id    bigint        NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  version        int           NOT NULL,
  steps_json     text          NOT NULL,
  triggers_json  text,
  env_json       text,
  runtime_json   text,
  workspace_json text,
  hash_sha256    varchar(64)   NOT NULL,
  created_by     bigint        NOT NULL,
  created_at     timestamptz   DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pipeline_version      ON pipeline_versions (pipeline_id, version);
CREATE INDEX        IF NOT EXISTS idx_pipeline_versions_hash ON pipeline_versions (hash_sha256);

-- Resolve circular FK: pipelines.current_version_id → pipeline_versions.id
ALTER TABLE pipelines
    ADD CONSTRAINT fk_pipelines_current_version
    FOREIGN KEY (current_version_id) REFERENCES pipeline_versions(id)
    NOT VALID;

-- ── pipeline_runs ────────────────────────────────────────────────────────────
-- No environment_id (dropped in 011). Includes rerun_from_step from model.
CREATE TABLE IF NOT EXISTS pipeline_runs (
  id                bigserial     PRIMARY KEY,
  pipeline_id       bigint        NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  snapshot_id       bigint        NOT NULL REFERENCES pipeline_versions(id),
  cluster_id        bigint        NOT NULL,
  namespace         varchar(253)  NOT NULL,
  status            varchar(20)   NOT NULL DEFAULT 'queued',
  trigger_type      varchar(20)   NOT NULL,
  trigger_payload   text,
  triggered_by_user bigint        NOT NULL,
  concurrency_group varchar(255),
  rerun_from_id     bigint,
  rerun_from_step   varchar(255),
  error             text,
  queued_at         timestamptz   DEFAULT now(),
  started_at        timestamptz,
  finished_at       timestamptz,
  bound_node_name   varchar(255),
  created_at        timestamptz   DEFAULT now(),
  updated_at        timestamptz   DEFAULT now(),
  deleted_at        timestamptz
);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_pipeline_id       ON pipeline_runs (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_snapshot_id       ON pipeline_runs (snapshot_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_cluster_id        ON pipeline_runs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_status            ON pipeline_runs (status);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_concurrency_group ON pipeline_runs (concurrency_group);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_deleted_at        ON pipeline_runs (deleted_at);

-- ── step_runs ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS step_runs (
  id              bigserial     PRIMARY KEY,
  pipeline_run_id bigint        NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  step_name       varchar(255)  NOT NULL,
  step_type       varchar(50)   NOT NULL,
  step_index      int           NOT NULL,
  status          varchar(20)   NOT NULL DEFAULT 'pending',
  image           varchar(512),
  command         text,
  config_json     text,
  job_name        varchar(255),
  job_namespace   varchar(253),
  exit_code       int,
  error           text,
  retry_count     int           DEFAULT 0,
  max_retries     int           DEFAULT 0,
  depends_on      jsonb,
  started_at      timestamptz,
  finished_at     timestamptz,
  scan_result_id  bigint,
  rollout_status  varchar(30),
  rollout_weight  int,
  approved_by     varchar(255),
  approved_at     timestamptz,
  created_at      timestamptz   DEFAULT now(),
  updated_at      timestamptz   DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_step_runs_pipeline_run_id ON step_runs (pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_step_runs_status          ON step_runs (status);

-- ── pipeline_secrets ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS pipeline_secrets (
  id          bigserial     PRIMARY KEY,
  scope       varchar(20)   NOT NULL,
  scope_ref   bigint,
  name        varchar(100)  NOT NULL,
  value_enc   text          NOT NULL,
  description varchar(255),
  created_by  bigint        NOT NULL,
  created_at  timestamptz   DEFAULT now(),
  updated_at  timestamptz   DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_scope_name             ON pipeline_secrets (scope, scope_ref, name) WHERE deleted_at IS NULL;
CREATE INDEX        IF NOT EXISTS idx_pipeline_secrets_deleted_at ON pipeline_secrets (deleted_at);

-- ── pipeline_artifacts ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS pipeline_artifacts (
  id              bigserial     PRIMARY KEY,
  pipeline_run_id bigint        NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  step_run_id     bigint        NOT NULL REFERENCES step_runs(id) ON DELETE CASCADE,
  kind            varchar(50),
  name            varchar(255),
  reference       text,
  size_bytes      bigint,
  metadata_json   text,
  created_at      timestamptz   DEFAULT now(),
  expires_at      timestamptz
);
CREATE INDEX IF NOT EXISTS idx_pipeline_artifacts_run  ON pipeline_artifacts (pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_artifacts_kind ON pipeline_artifacts (kind);

-- ── pipeline_logs ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS pipeline_logs (
  id              bigserial     PRIMARY KEY,
  pipeline_run_id bigint        NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  step_run_id     bigint        NOT NULL REFERENCES step_runs(id) ON DELETE CASCADE,
  chunk_seq       int           NOT NULL,
  content         text,
  stored_at       timestamptz   DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_step_chunk      ON pipeline_logs (step_run_id, chunk_seq);
CREATE INDEX        IF NOT EXISTS idx_pipeline_logs_run ON pipeline_logs (pipeline_run_id);

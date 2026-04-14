-- Migration 001: Baseline schema snapshot.
-- Every statement uses CREATE TABLE IF NOT EXISTS so this is safe to run on
-- databases already bootstrapped by GORM AutoMigrate (existing rows are kept).


-- -- users -----------------------------------------------------------------
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE INDEX IF NOT EXISTS idx_users_system_role ON users (system_role);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- -- user_groups -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_groups (
  id          bigserial       PRIMARY KEY,
  name        varchar(50)     NOT NULL,
  description varchar(255)    NOT NULL DEFAULT '',
  created_at  timestamptz(3)  NOT NULL,
  updated_at  timestamptz(3)  NOT NULL,
  deleted_at  timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups (name);
CREATE INDEX IF NOT EXISTS idx_user_groups_deleted_at ON user_groups (deleted_at);

-- -- user_group_members (many2many join table) -----------------------------
CREATE TABLE IF NOT EXISTS user_group_members (
  user_id       bigint NOT NULL,
  user_group_id bigint NOT NULL,
  PRIMARY KEY (user_id, user_group_id)
);

-- -- clusters --------------------------------------------------------------
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_clusters_name ON clusters (name);
CREATE INDEX IF NOT EXISTS idx_clusters_deleted_at ON clusters (deleted_at);

-- -- cluster_metrics -------------------------------------------------------
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

-- -- cluster_permissions ---------------------------------------------------
CREATE TABLE IF NOT EXISTS cluster_permissions (
  id              bigserial       PRIMARY KEY,
  cluster_id      bigint          NOT NULL,
  user_id         bigint          NULL,
  user_group_id   bigint          NULL,
  permission_type varchar(50)     NOT NULL,
  namespaces      text,
  custom_role_ref varchar(200)    NOT NULL DEFAULT '',
  created_at      timestamptz(3)  NOT NULL,
  updated_at      timestamptz(3)  NOT NULL,
  deleted_at      timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_cluster_id ON cluster_permissions (cluster_id);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_user_id ON cluster_permissions (user_id);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_user_group_id ON cluster_permissions (user_group_id);
CREATE INDEX IF NOT EXISTS idx_cluster_permissions_deleted_at ON cluster_permissions (deleted_at);

-- -- terminal_sessions -----------------------------------------------------
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

-- -- terminal_commands -----------------------------------------------------
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

-- -- audit_logs ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
  id            bigserial       PRIMARY KEY,
  user_id       bigint          NOT NULL,
  action        varchar(100)    NOT NULL,
  resource_type varchar(50)     NOT NULL,
  resource_ref  jsonb           DEFAULT NULL,
  result        varchar(20)     NOT NULL,
  ip            varchar(45)     NOT NULL DEFAULT '',
  user_agent    varchar(500)    NOT NULL DEFAULT '',
  details       text,
  prev_hash     varchar(64)     NOT NULL DEFAULT '',
  hash          varchar(64)     NOT NULL DEFAULT '',
  created_at    timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_hash ON audit_logs (hash);

-- -- operation_logs --------------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_op_user_time ON operation_logs (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_operation_logs_username ON operation_logs (username);
CREATE INDEX IF NOT EXISTS idx_operation_logs_method ON operation_logs (method);
CREATE INDEX IF NOT EXISTS idx_operation_logs_module ON operation_logs (module);
CREATE INDEX IF NOT EXISTS idx_operation_logs_action ON operation_logs (action);
CREATE INDEX IF NOT EXISTS idx_operation_logs_cluster_id ON operation_logs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_success ON operation_logs (success);

-- -- system_settings -------------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_system_settings_deleted_at ON system_settings (deleted_at);

-- -- argocd_configs --------------------------------------------------------
CREATE TABLE IF NOT EXISTS argocd_configs (
  id                    bigserial       PRIMARY KEY,
  cluster_id            bigint          NOT NULL,
  enabled               boolean         NOT NULL DEFAULT false,
  server_url            varchar(255)    NOT NULL DEFAULT '',
  auth_type             varchar(20)     NOT NULL DEFAULT '',
  token                 text,
  username              varchar(100)    NOT NULL DEFAULT '',
  password              text,
  insecure              boolean         NOT NULL DEFAULT false,
  git_repo_url          varchar(500)    NOT NULL DEFAULT '',
  git_branch            varchar(100)    NOT NULL DEFAULT 'main',
  git_path              varchar(255)    NOT NULL DEFAULT '',
  git_auth_type         varchar(20)     NOT NULL DEFAULT '',
  git_username          varchar(100)    NOT NULL DEFAULT '',
  git_password          text,
  git_ssh_key           text,
  argo_cd_cluster_name  varchar(100)    NOT NULL DEFAULT '',
  argo_cd_project       varchar(100)    NOT NULL DEFAULT 'default',
  connection_status     varchar(20)     NOT NULL DEFAULT '',
  last_test_at          timestamptz(3)  NULL,
  error_message         text,
  created_at            timestamptz(3)  NOT NULL,
  updated_at            timestamptz(3)  NOT NULL,
  deleted_at            timestamptz(3)  NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_argocd_configs_cluster_id ON argocd_configs (cluster_id);
CREATE INDEX IF NOT EXISTS idx_argocd_configs_deleted_at ON argocd_configs (deleted_at);

-- -- ai_configs ------------------------------------------------------------
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

-- -- helm_repositories -----------------------------------------------------
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_helm_repositories_name ON helm_repositories (name);
CREATE INDEX IF NOT EXISTS idx_helm_repositories_deleted_at ON helm_repositories (deleted_at);

-- -- event_alert_rules -----------------------------------------------------
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

-- -- event_alert_histories -------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_rule_id ON event_alert_histories (rule_id);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_cluster_id ON event_alert_histories (cluster_id);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_is_read ON event_alert_histories (is_read);
CREATE INDEX IF NOT EXISTS idx_event_alert_histories_triggered_at ON event_alert_histories (triggered_at);

-- -- cost_configs ----------------------------------------------------------
CREATE TABLE IF NOT EXISTS cost_configs (
  id                 bigserial        PRIMARY KEY,
  cluster_id         bigint           NOT NULL,
  cpu_price_per_core double precision NOT NULL DEFAULT 0.048,
  mem_price_per_gi_b double precision NOT NULL DEFAULT 0.006,
  currency           varchar(10)      NOT NULL DEFAULT 'USD',
  updated_at         timestamptz(3)   NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cost_configs_cluster_id ON cost_configs (cluster_id);

-- -- resource_snapshots ----------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_resource_snapshots_date ON resource_snapshots (date);

-- -- cluster_occupancy_snapshots -------------------------------------------
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

-- -- cloud_billing_configs -------------------------------------------------
CREATE TABLE IF NOT EXISTS cloud_billing_configs (
  id                       bigserial       PRIMARY KEY,
  cluster_id               bigint          NOT NULL,
  provider                 varchar(20)     NOT NULL DEFAULT 'disabled',
  aws_access_key_id        varchar(128)    NOT NULL DEFAULT '',
  aws_secret_access_key    varchar(256)    NOT NULL DEFAULT '',
  aws_region               varchar(32)     NOT NULL DEFAULT 'us-east-1',
  aws_linked_account_id    varchar(20)     NOT NULL DEFAULT '',
  gcp_project_id           varchar(128)    NOT NULL DEFAULT '',
  gcp_billing_account_id   varchar(64)     NOT NULL DEFAULT '',
  gcp_service_account_json text,
  last_synced_at           timestamptz(3)  NULL,
  last_error               varchar(512)    NOT NULL DEFAULT '',
  created_at               timestamptz(3)  NOT NULL,
  updated_at               timestamptz(3)  NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cloud_billing_configs_cluster_id ON cloud_billing_configs (cluster_id);

-- -- cloud_billing_records -------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_cloud_billing_records_month ON cloud_billing_records (month);

-- -- image_scan_results ----------------------------------------------------
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
  created_at     timestamptz(3)  NOT NULL,
  updated_at     timestamptz(3)  NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_image_scan_results_cluster_id ON image_scan_results (cluster_id);

-- -- bench_results ---------------------------------------------------------
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

-- -- sync_policies ---------------------------------------------------------
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

-- -- sync_histories --------------------------------------------------------
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

-- -- config_versions -------------------------------------------------------
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

-- -- namespace_protections -------------------------------------------------
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_ns_protection ON namespace_protections (cluster_id, namespace);
CREATE INDEX IF NOT EXISTS idx_namespace_protections_deleted_at ON namespace_protections (deleted_at);

-- -- approval_requests -----------------------------------------------------
CREATE TABLE IF NOT EXISTS approval_requests (
  id             bigserial       PRIMARY KEY,
  created_at     timestamptz(3)  NOT NULL,
  updated_at     timestamptz(3)  NOT NULL,
  deleted_at     timestamptz(3)  NULL,
  cluster_id     bigint          NOT NULL,
  cluster_name   varchar(255)    NOT NULL DEFAULT '',
  namespace      varchar(253)    NOT NULL,
  resource_kind  varchar(255)    NOT NULL,
  resource_name  varchar(255)    NOT NULL,
  action         varchar(255)    NOT NULL,
  requester_id   bigint          NOT NULL,
  requester_name varchar(255)    NOT NULL DEFAULT '',
  approver_id    bigint          NULL,
  approver_name  varchar(255)    NOT NULL DEFAULT '',
  status         varchar(255)    NOT NULL DEFAULT 'pending',
  payload        text,
  reason         text,
  expires_at     timestamptz(3)  NOT NULL,
  approved_at    timestamptz(3)  NULL
);
CREATE INDEX IF NOT EXISTS idx_approval_requests_cluster_id ON approval_requests (cluster_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_namespace ON approval_requests (namespace);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests (status);
CREATE INDEX IF NOT EXISTS idx_approval_requests_deleted_at ON approval_requests (deleted_at);

-- -- port_forward_sessions -------------------------------------------------
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

-- -- siem_webhook_configs --------------------------------------------------
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

-- -- api_tokens ------------------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens (user_id);

-- -- notify_channels -------------------------------------------------------
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_notify_channels_name ON notify_channels (name);
CREATE INDEX IF NOT EXISTS idx_notify_channels_deleted_at ON notify_channels (deleted_at);

-- -- log_source_configs ----------------------------------------------------
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

-- -- image_indices ---------------------------------------------------------
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
CREATE INDEX IF NOT EXISTS idx_image_indices_cluster_id ON image_indices (cluster_id);
CREATE INDEX IF NOT EXISTS idx_image_indices_namespace ON image_indices (namespace);
CREATE INDEX IF NOT EXISTS idx_image_indices_image ON image_indices (image);
CREATE INDEX IF NOT EXISTS idx_image_indices_image_name ON image_indices (image_name);
CREATE INDEX IF NOT EXISTS idx_image_indices_deleted_at ON image_indices (deleted_at);

-- -- token_blacklists ------------------------------------------------------
CREATE TABLE IF NOT EXISTS token_blacklists (
  id         bigserial       PRIMARY KEY,
  jti        varchar(64)     NOT NULL,
  user_id    bigint          NOT NULL DEFAULT 0,
  reason     varchar(64)     NOT NULL DEFAULT '',
  expires_at timestamptz(3)  NOT NULL,
  created_at timestamptz(3)  NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_token_blacklists_jti ON token_blacklists (jti);
CREATE INDEX IF NOT EXISTS idx_token_blacklists_user_id ON token_blacklists (user_id);
CREATE INDEX IF NOT EXISTS idx_token_blacklists_expires_at ON token_blacklists (expires_at);

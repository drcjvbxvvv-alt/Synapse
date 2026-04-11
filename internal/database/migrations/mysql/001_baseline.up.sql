-- Migration 001: Baseline schema snapshot.
-- Every statement uses CREATE TABLE IF NOT EXISTS so this is safe to run on
-- databases already bootstrapped by GORM AutoMigrate (existing rows are kept).

SET NAMES utf8mb4;
SET foreign_key_checks = 0;

-- -- users -----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `users` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `username`      varchar(50)     NOT NULL,
  `password_hash` varchar(255)    NOT NULL DEFAULT '',
  `salt`          varchar(32)     NOT NULL DEFAULT '',
  `email`         varchar(100)    NOT NULL DEFAULT '',
  `display_name`  varchar(100)    NOT NULL DEFAULT '',
  `phone`         varchar(20)     NOT NULL DEFAULT '',
  `auth_type`     varchar(20)     NOT NULL DEFAULT 'local',
  `status`        varchar(20)     NOT NULL DEFAULT 'active',
  `system_role`   varchar(32)     NOT NULL DEFAULT 'user',
  `last_login_at` datetime(3)     NULL,
  `last_login_ip` varchar(50)     NOT NULL DEFAULT '',
  `created_at`    datetime(3)     NOT NULL,
  `updated_at`    datetime(3)     NOT NULL,
  `deleted_at`    datetime(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_username` (`username`),
  KEY `idx_users_system_role` (`system_role`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- user_groups -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user_groups` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`        varchar(50)     NOT NULL,
  `description` varchar(255)    NOT NULL DEFAULT '',
  `created_at`  datetime(3)     NOT NULL,
  `updated_at`  datetime(3)     NOT NULL,
  `deleted_at`  datetime(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_groups_name` (`name`),
  KEY `idx_user_groups_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- user_group_members (many2many join table) -----------------------------
CREATE TABLE IF NOT EXISTS `user_group_members` (
  `user_id`       bigint unsigned NOT NULL,
  `user_group_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`user_id`, `user_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- clusters --------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `clusters` (
  `id`                   bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`                 varchar(100)    NOT NULL,
  `api_server`           varchar(255)    NOT NULL,
  `kubeconfig_enc`       longtext,
  `ca_enc`               longtext,
  `sa_token_enc`         longtext,
  `version`              varchar(50)     NOT NULL DEFAULT '',
  `status`               varchar(20)     NOT NULL DEFAULT 'unknown',
  `labels`               json            DEFAULT NULL,
  `cert_expire_at`       datetime(3)     NULL,
  `last_heartbeat`       datetime(3)     NULL,
  `created_by`           bigint unsigned NOT NULL DEFAULT 0,
  `monitoring_config`    json            DEFAULT NULL,
  `alertmanager_config`  json            DEFAULT NULL,
  `created_at`           datetime(3)     NOT NULL,
  `updated_at`           datetime(3)     NOT NULL,
  `deleted_at`           datetime(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_clusters_name` (`name`),
  KEY `idx_clusters_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- cluster_metrics -------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cluster_metrics` (
  `cluster_id`    bigint unsigned NOT NULL,
  `node_count`    bigint          NOT NULL DEFAULT 0,
  `ready_nodes`   bigint          NOT NULL DEFAULT 0,
  `pod_count`     bigint          NOT NULL DEFAULT 0,
  `running_pods`  bigint          NOT NULL DEFAULT 0,
  `cpu_usage`     double          NOT NULL DEFAULT 0,
  `memory_usage`  double          NOT NULL DEFAULT 0,
  `storage_usage` double          NOT NULL DEFAULT 0,
  `updated_at`    datetime(3)     NOT NULL,
  PRIMARY KEY (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- cluster_permissions ---------------------------------------------------
CREATE TABLE IF NOT EXISTS `cluster_permissions` (
  `id`              bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`      bigint unsigned NOT NULL,
  `user_id`         bigint unsigned NULL,
  `user_group_id`   bigint unsigned NULL,
  `permission_type` varchar(50)     NOT NULL,
  `namespaces`      text,
  `custom_role_ref` varchar(200)    NOT NULL DEFAULT '',
  `created_at`      datetime(3)     NOT NULL,
  `updated_at`      datetime(3)     NOT NULL,
  `deleted_at`      datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_cluster_permissions_cluster_id` (`cluster_id`),
  KEY `idx_cluster_permissions_user_id` (`user_id`),
  KEY `idx_cluster_permissions_user_group_id` (`user_group_id`),
  KEY `idx_cluster_permissions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- terminal_sessions -----------------------------------------------------
CREATE TABLE IF NOT EXISTS `terminal_sessions` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`     bigint unsigned NOT NULL,
  `cluster_id`  bigint unsigned NOT NULL,
  `target_type` varchar(20)     NOT NULL,
  `target_ref`  json            DEFAULT NULL,
  `namespace`   varchar(100)    NOT NULL DEFAULT '',
  `pod`         varchar(100)    NOT NULL DEFAULT '',
  `container`   varchar(100)    NOT NULL DEFAULT '',
  `node`        varchar(100)    NOT NULL DEFAULT '',
  `start_at`    datetime(3)     NOT NULL,
  `end_at`      datetime(3)     NULL,
  `input_size`  bigint          NOT NULL DEFAULT 0,
  `status`      varchar(20)     NOT NULL DEFAULT 'active',
  `created_at`  datetime(3)     NOT NULL,
  `updated_at`  datetime(3)     NOT NULL,
  `deleted_at`  datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_terminal_sessions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- terminal_commands -----------------------------------------------------
CREATE TABLE IF NOT EXISTS `terminal_commands` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `timestamp`  datetime(3)     NOT NULL,
  `raw_input`  text,
  `parsed_cmd` varchar(1024)   NOT NULL DEFAULT '',
  `exit_code`  bigint          NULL,
  `created_at` datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_terminal_commands_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- audit_logs ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `audit_logs` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`       bigint unsigned NOT NULL,
  `action`        varchar(100)    NOT NULL,
  `resource_type` varchar(50)     NOT NULL,
  `resource_ref`  json            DEFAULT NULL,
  `result`        varchar(20)     NOT NULL,
  `ip`            varchar(45)     NOT NULL DEFAULT '',
  `user_agent`    varchar(500)    NOT NULL DEFAULT '',
  `details`       text,
  `created_at`    datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_audit_logs_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- operation_logs --------------------------------------------------------
CREATE TABLE IF NOT EXISTS `operation_logs` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`       bigint unsigned NULL,
  `username`      varchar(100)    NOT NULL DEFAULT '',
  `method`        varchar(10)     NOT NULL DEFAULT '',
  `path`          varchar(500)    NOT NULL DEFAULT '',
  `query`         varchar(1000)   NOT NULL DEFAULT '',
  `module`        varchar(50)     NOT NULL DEFAULT '',
  `action`        varchar(100)    NOT NULL DEFAULT '',
  `cluster_id`    bigint unsigned NULL,
  `cluster_name`  varchar(100)    NOT NULL DEFAULT '',
  `namespace`     varchar(100)    NOT NULL DEFAULT '',
  `resource_type` varchar(50)     NOT NULL DEFAULT '',
  `resource_name` varchar(200)    NOT NULL DEFAULT '',
  `request_body`  text,
  `status_code`   bigint          NOT NULL DEFAULT 0,
  `success`       tinyint(1)      NOT NULL DEFAULT 0,
  `error_message` varchar(1000)   NOT NULL DEFAULT '',
  `client_ip`     varchar(45)     NOT NULL DEFAULT '',
  `user_agent`    varchar(500)    NOT NULL DEFAULT '',
  `duration`      bigint          NOT NULL DEFAULT 0,
  `created_at`    datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_op_user_time` (`user_id`, `created_at`),
  KEY `idx_operation_logs_username` (`username`),
  KEY `idx_operation_logs_method` (`method`),
  KEY `idx_operation_logs_module` (`module`),
  KEY `idx_operation_logs_action` (`action`),
  KEY `idx_operation_logs_cluster_id` (`cluster_id`),
  KEY `idx_operation_logs_success` (`success`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- system_settings -------------------------------------------------------
CREATE TABLE IF NOT EXISTS `system_settings` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `config_key` varchar(100)    NOT NULL,
  `value`      text,
  `type`       varchar(50)     NOT NULL DEFAULT '',
  `created_at` datetime(3)     NOT NULL,
  `updated_at` datetime(3)     NOT NULL,
  `deleted_at` datetime(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_system_settings_config_key` (`config_key`),
  KEY `idx_system_settings_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- argocd_configs --------------------------------------------------------
CREATE TABLE IF NOT EXISTS `argocd_configs` (
  `id`                    bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`            bigint unsigned NOT NULL,
  `enabled`               tinyint(1)      NOT NULL DEFAULT 0,
  `server_url`            varchar(255)    NOT NULL DEFAULT '',
  `auth_type`             varchar(20)     NOT NULL DEFAULT '',
  `token`                 text,
  `username`              varchar(100)    NOT NULL DEFAULT '',
  `password`              text,
  `insecure`              tinyint(1)      NOT NULL DEFAULT 0,
  `git_repo_url`          varchar(500)    NOT NULL DEFAULT '',
  `git_branch`            varchar(100)    NOT NULL DEFAULT 'main',
  `git_path`              varchar(255)    NOT NULL DEFAULT '',
  `git_auth_type`         varchar(20)     NOT NULL DEFAULT '',
  `git_username`          varchar(100)    NOT NULL DEFAULT '',
  `git_password`          text,
  `git_ssh_key`           text,
  `argo_cd_cluster_name`  varchar(100)    NOT NULL DEFAULT '',
  `argo_cd_project`       varchar(100)    NOT NULL DEFAULT 'default',
  `connection_status`     varchar(20)     NOT NULL DEFAULT '',
  `last_test_at`          datetime(3)     NULL,
  `error_message`         text,
  `created_at`            datetime(3)     NOT NULL,
  `updated_at`            datetime(3)     NOT NULL,
  `deleted_at`            datetime(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_argocd_configs_cluster_id` (`cluster_id`),
  KEY `idx_argocd_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- ai_configs ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `ai_configs` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `provider`    varchar(50)     NOT NULL DEFAULT 'openai',
  `endpoint`    varchar(255)    NOT NULL DEFAULT '',
  `api_key`     text,
  `model`       varchar(100)    NOT NULL DEFAULT '',
  `api_version` varchar(50)     NOT NULL DEFAULT '',
  `enabled`     tinyint(1)      NOT NULL DEFAULT 0,
  `created_at`  datetime(3)     NOT NULL,
  `updated_at`  datetime(3)     NOT NULL,
  `deleted_at`  datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ai_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- helm_repositories -----------------------------------------------------
CREATE TABLE IF NOT EXISTS `helm_repositories` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3)     NOT NULL,
  `updated_at` datetime(3)     NOT NULL,
  `deleted_at` datetime(3)     NULL,
  `name`       varchar(128)    NOT NULL DEFAULT '',
  `url`        varchar(512)    NOT NULL DEFAULT '',
  `username`   varchar(256)    NOT NULL DEFAULT '',
  `password`   varchar(256)    NOT NULL DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_helm_repositories_name` (`name`),
  KEY `idx_helm_repositories_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- event_alert_rules -----------------------------------------------------
CREATE TABLE IF NOT EXISTS `event_alert_rules` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`   bigint unsigned NOT NULL,
  `name`         varchar(100)    NOT NULL,
  `description`  varchar(255)    NOT NULL DEFAULT '',
  `namespace`    varchar(100)    NOT NULL DEFAULT '',
  `event_reason` varchar(100)    NOT NULL DEFAULT '',
  `event_type`   varchar(20)     NOT NULL DEFAULT '',
  `min_count`    bigint          NOT NULL DEFAULT 1,
  `notify_type`  varchar(20)     NOT NULL DEFAULT '',
  `notify_url`   varchar(500)    NOT NULL DEFAULT '',
  `enabled`      tinyint(1)      NOT NULL DEFAULT 1,
  `created_at`   datetime(3)     NOT NULL,
  `updated_at`   datetime(3)     NOT NULL,
  `deleted_at`   datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_event_alert_rules_cluster_id` (`cluster_id`),
  KEY `idx_event_alert_rules_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- event_alert_histories -------------------------------------------------
CREATE TABLE IF NOT EXISTS `event_alert_histories` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `rule_id`       bigint unsigned NOT NULL,
  `cluster_id`    bigint unsigned NOT NULL,
  `rule_name`     varchar(100)    NOT NULL DEFAULT '',
  `namespace`     varchar(100)    NOT NULL DEFAULT '',
  `event_reason`  varchar(100)    NOT NULL DEFAULT '',
  `event_type`    varchar(20)     NOT NULL DEFAULT '',
  `message`       text,
  `involved_obj`  varchar(200)    NOT NULL DEFAULT '',
  `notify_result` varchar(50)     NOT NULL DEFAULT '',
  `is_read`       tinyint(1)      NOT NULL DEFAULT 0,
  `triggered_at`  datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_event_alert_histories_rule_id` (`rule_id`),
  KEY `idx_event_alert_histories_cluster_id` (`cluster_id`),
  KEY `idx_event_alert_histories_is_read` (`is_read`),
  KEY `idx_event_alert_histories_triggered_at` (`triggered_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- cost_configs ----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cost_configs` (
  `id`                 bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`         bigint unsigned NOT NULL,
  `cpu_price_per_core` double          NOT NULL DEFAULT 0.048,
  `mem_price_per_gi_b` double          NOT NULL DEFAULT 0.006,
  `currency`           varchar(10)     NOT NULL DEFAULT 'USD',
  `updated_at`         datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cost_configs_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- resource_snapshots ----------------------------------------------------
CREATE TABLE IF NOT EXISTS `resource_snapshots` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`  bigint unsigned NOT NULL,
  `namespace`   varchar(128)    NOT NULL,
  `workload`    varchar(256)    NOT NULL,
  `date`        datetime(3)     NOT NULL,
  `cpu_request` double          NOT NULL DEFAULT 0,
  `cpu_usage`   double          NOT NULL DEFAULT 0,
  `mem_request` double          NOT NULL DEFAULT 0,
  `mem_usage`   double          NOT NULL DEFAULT 0,
  `pod_count`   bigint          NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_resource_snapshots_cluster_id` (`cluster_id`),
  KEY `idx_resource_snapshots_date` (`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- cluster_occupancy_snapshots -------------------------------------------
CREATE TABLE IF NOT EXISTS `cluster_occupancy_snapshots` (
  `id`                 bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`         bigint unsigned NOT NULL,
  `date`               datetime(3)     NOT NULL,
  `allocatable_cpu`    double          NOT NULL DEFAULT 0,
  `allocatable_memory` double          NOT NULL DEFAULT 0,
  `requested_cpu`      double          NOT NULL DEFAULT 0,
  `requested_memory`   double          NOT NULL DEFAULT 0,
  `node_count`         bigint          NOT NULL DEFAULT 0,
  `pod_count`          bigint          NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_date` (`cluster_id`, `date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- cloud_billing_configs -------------------------------------------------
CREATE TABLE IF NOT EXISTS `cloud_billing_configs` (
  `id`                       bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`               bigint unsigned NOT NULL,
  `provider`                 varchar(20)     NOT NULL DEFAULT 'disabled',
  `aws_access_key_id`        varchar(128)    NOT NULL DEFAULT '',
  `aws_secret_access_key`    varchar(256)    NOT NULL DEFAULT '',
  `aws_region`               varchar(32)     NOT NULL DEFAULT 'us-east-1',
  `aws_linked_account_id`    varchar(20)     NOT NULL DEFAULT '',
  `gcp_project_id`           varchar(128)    NOT NULL DEFAULT '',
  `gcp_billing_account_id`   varchar(64)     NOT NULL DEFAULT '',
  `gcp_service_account_json` text,
  `last_synced_at`           datetime(3)     NULL,
  `last_error`               varchar(512)    NOT NULL DEFAULT '',
  `created_at`               datetime(3)     NOT NULL,
  `updated_at`               datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cloud_billing_configs_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- cloud_billing_records -------------------------------------------------
CREATE TABLE IF NOT EXISTS `cloud_billing_records` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL,
  `month`      varchar(7)      NOT NULL,
  `provider`   varchar(20)     NOT NULL DEFAULT '',
  `service`    varchar(256)    NOT NULL DEFAULT '',
  `amount`     double          NOT NULL DEFAULT 0,
  `currency`   varchar(10)     NOT NULL DEFAULT 'USD',
  `created_at` datetime(3)     NOT NULL,
  `updated_at` datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_cloud_billing_records_cluster_id` (`cluster_id`),
  KEY `idx_cloud_billing_records_month` (`month`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- image_scan_results ----------------------------------------------------
CREATE TABLE IF NOT EXISTS `image_scan_results` (
  `id`             bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`     bigint unsigned NOT NULL,
  `namespace`      varchar(100)    NOT NULL DEFAULT '',
  `pod_name`       varchar(255)    NOT NULL DEFAULT '',
  `container_name` varchar(255)    NOT NULL DEFAULT '',
  `image`          varchar(512)    NOT NULL,
  `status`         varchar(20)     NOT NULL DEFAULT 'pending',
  `critical`       bigint          NOT NULL DEFAULT 0,
  `high`           bigint          NOT NULL DEFAULT 0,
  `medium`         bigint          NOT NULL DEFAULT 0,
  `low`            bigint          NOT NULL DEFAULT 0,
  `unknown`        bigint          NOT NULL DEFAULT 0,
  `result_json`    longtext,
  `error`          varchar(512)    NOT NULL DEFAULT '',
  `scanned_at`     datetime(3)     NULL,
  `created_at`     datetime(3)     NOT NULL,
  `updated_at`     datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_image_scan_results_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- bench_results ---------------------------------------------------------
CREATE TABLE IF NOT EXISTS `bench_results` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`  bigint unsigned NOT NULL,
  `status`      varchar(20)     NOT NULL DEFAULT 'pending',
  `pass`        bigint          NOT NULL DEFAULT 0,
  `fail`        bigint          NOT NULL DEFAULT 0,
  `warn`        bigint          NOT NULL DEFAULT 0,
  `info`        bigint          NOT NULL DEFAULT 0,
  `score`       double          NOT NULL DEFAULT 0,
  `result_json` longtext,
  `error`       varchar(512)    NOT NULL DEFAULT '',
  `job_name`    varchar(255)    NOT NULL DEFAULT '',
  `run_at`      datetime(3)     NULL,
  `created_at`  datetime(3)     NOT NULL,
  `updated_at`  datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_bench_results_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- sync_policies ---------------------------------------------------------
CREATE TABLE IF NOT EXISTS `sync_policies` (
  `id`                bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`              varchar(128)    NOT NULL,
  `description`       varchar(512)    NOT NULL DEFAULT '',
  `source_cluster_id` bigint unsigned NOT NULL,
  `source_namespace`  varchar(128)    NOT NULL,
  `resource_type`     varchar(32)     NOT NULL,
  `resource_names`    text,
  `target_clusters`   text,
  `conflict_policy`   varchar(16)     NOT NULL DEFAULT 'skip',
  `schedule`          varchar(64)     NOT NULL DEFAULT '',
  `enabled`           tinyint(1)      NOT NULL DEFAULT 1,
  `last_sync_at`      datetime(3)     NULL,
  `last_sync_status`  varchar(16)     NOT NULL DEFAULT '',
  `created_at`        datetime(3)     NOT NULL,
  `updated_at`        datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_sync_policies_source_cluster_id` (`source_cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- sync_histories --------------------------------------------------------
CREATE TABLE IF NOT EXISTS `sync_histories` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `policy_id`    bigint unsigned NOT NULL,
  `triggered_by` varchar(64)     NOT NULL DEFAULT '',
  `status`       varchar(16)     NOT NULL DEFAULT '',
  `message`      text,
  `details`      text,
  `started_at`   datetime(3)     NOT NULL,
  `finished_at`  datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_sync_histories_policy_id` (`policy_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- config_versions -------------------------------------------------------
CREATE TABLE IF NOT EXISTS `config_versions` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`    bigint unsigned NOT NULL,
  `resource_type` varchar(20)     NOT NULL,
  `namespace`     varchar(255)    NOT NULL,
  `name`          varchar(255)    NOT NULL,
  `version`       bigint          NOT NULL,
  `content_json`  text,
  `changed_by`    varchar(100)    NOT NULL DEFAULT '',
  `changed_at`    datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_config_ver` (`cluster_id`, `resource_type`, `namespace`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- namespace_protections -------------------------------------------------
CREATE TABLE IF NOT EXISTS `namespace_protections` (
  `id`               bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at`       datetime(3)     NOT NULL,
  `updated_at`       datetime(3)     NOT NULL,
  `deleted_at`       datetime(3)     NULL,
  `cluster_id`       bigint unsigned NOT NULL,
  `namespace`        varchar(253)    NOT NULL,
  `require_approval` tinyint(1)      NOT NULL DEFAULT 0,
  `description`      text,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ns_protection` (`cluster_id`, `namespace`),
  KEY `idx_namespace_protections_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- approval_requests -----------------------------------------------------
CREATE TABLE IF NOT EXISTS `approval_requests` (
  `id`             bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at`     datetime(3)     NOT NULL,
  `updated_at`     datetime(3)     NOT NULL,
  `deleted_at`     datetime(3)     NULL,
  `cluster_id`     bigint unsigned NOT NULL,
  `cluster_name`   varchar(255)    NOT NULL DEFAULT '',
  `namespace`      varchar(253)    NOT NULL,
  `resource_kind`  varchar(255)    NOT NULL,
  `resource_name`  varchar(255)    NOT NULL,
  `action`         varchar(255)    NOT NULL,
  `requester_id`   bigint unsigned NOT NULL,
  `requester_name` varchar(255)    NOT NULL DEFAULT '',
  `approver_id`    bigint unsigned NULL,
  `approver_name`  varchar(255)    NOT NULL DEFAULT '',
  `status`         varchar(255)    NOT NULL DEFAULT 'pending',
  `payload`        text,
  `reason`         text,
  `expires_at`     datetime(3)     NOT NULL,
  `approved_at`    datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_approval_requests_cluster_id` (`cluster_id`),
  KEY `idx_approval_requests_namespace` (`namespace`),
  KEY `idx_approval_requests_status` (`status`),
  KEY `idx_approval_requests_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- port_forward_sessions -------------------------------------------------
CREATE TABLE IF NOT EXISTS `port_forward_sessions` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at`   datetime(3)     NOT NULL,
  `updated_at`   datetime(3)     NOT NULL,
  `deleted_at`   datetime(3)     NULL,
  `cluster_id`   bigint unsigned NOT NULL,
  `cluster_name` varchar(255)    NOT NULL DEFAULT '',
  `namespace`    varchar(255)    NOT NULL,
  `pod_name`     varchar(255)    NOT NULL,
  `pod_port`     bigint          NOT NULL,
  `local_port`   bigint          NOT NULL,
  `user_id`      bigint unsigned NOT NULL,
  `username`     varchar(255)    NOT NULL DEFAULT '',
  `status`       varchar(255)    NOT NULL DEFAULT 'active',
  `stopped_at`   datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_port_forward_sessions_cluster_id` (`cluster_id`),
  KEY `idx_port_forward_sessions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- siem_webhook_configs --------------------------------------------------
CREATE TABLE IF NOT EXISTS `siem_webhook_configs` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at`    datetime(3)     NOT NULL,
  `updated_at`    datetime(3)     NOT NULL,
  `deleted_at`    datetime(3)     NULL,
  `enabled`       tinyint(1)      NOT NULL DEFAULT 0,
  `webhook_url`   varchar(255)    NOT NULL,
  `secret_header` varchar(255)    NOT NULL DEFAULT '',
  `secret_value`  varchar(255)    NOT NULL DEFAULT '',
  `batch_size`    bigint          NOT NULL DEFAULT 100,
  PRIMARY KEY (`id`),
  KEY `idx_siem_webhook_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- api_tokens ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `api_tokens` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`      bigint unsigned NOT NULL,
  `name`         varchar(100)    NOT NULL,
  `token_hash`   varchar(64)     NOT NULL,
  `scopes`       varchar(200)    NOT NULL DEFAULT '',
  `expires_at`   datetime(3)     NULL,
  `last_used_at` datetime(3)     NULL,
  `created_at`   datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_api_tokens_token_hash` (`token_hash`),
  KEY `idx_api_tokens_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- notify_channels -------------------------------------------------------
CREATE TABLE IF NOT EXISTS `notify_channels` (
  `id`               bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`             varchar(100)    NOT NULL,
  `type`             varchar(20)     NOT NULL,
  `webhook_url`      varchar(1000)   NOT NULL,
  `telegram_chat_id` varchar(200)    NOT NULL DEFAULT '',
  `description`      varchar(255)    NOT NULL DEFAULT '',
  `enabled`          tinyint(1)      NOT NULL DEFAULT 1,
  `created_at`       datetime(3)     NOT NULL,
  `updated_at`       datetime(3)     NOT NULL,
  `deleted_at`       datetime(3)     NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_notify_channels_name` (`name`),
  KEY `idx_notify_channels_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- log_source_configs ----------------------------------------------------
CREATE TABLE IF NOT EXISTS `log_source_configs` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL,
  `type`       varchar(20)     NOT NULL DEFAULT '',
  `name`       varchar(100)    NOT NULL DEFAULT '',
  `url`        varchar(255)    NOT NULL DEFAULT '',
  `username`   varchar(100)    NOT NULL DEFAULT '',
  `password`   varchar(255)    NOT NULL DEFAULT '',
  `api_key`    varchar(255)    NOT NULL DEFAULT '',
  `enabled`    tinyint(1)      NOT NULL DEFAULT 1,
  `created_at` datetime(3)     NOT NULL,
  `updated_at` datetime(3)     NOT NULL,
  `deleted_at` datetime(3)     NULL,
  PRIMARY KEY (`id`),
  KEY `idx_log_source_configs_cluster_id` (`cluster_id`),
  KEY `idx_log_source_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- image_indices ---------------------------------------------------------
CREATE TABLE IF NOT EXISTS `image_indices` (
  `id`             bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at`     datetime(3)     NOT NULL,
  `updated_at`     datetime(3)     NOT NULL,
  `deleted_at`     datetime(3)     NULL,
  `cluster_id`     bigint unsigned NOT NULL,
  `cluster_name`   varchar(255)    NOT NULL DEFAULT '',
  `namespace`      varchar(253)    NOT NULL,
  `workload_kind`  varchar(64)     NOT NULL,
  `workload_name`  varchar(253)    NOT NULL,
  `container_name` varchar(253)    NOT NULL,
  `image`          varchar(512)    NOT NULL,
  `image_name`     varchar(512)    NOT NULL,
  `image_tag`      varchar(255)    NOT NULL DEFAULT '',
  `last_sync_at`   datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_image_indices_cluster_id` (`cluster_id`),
  KEY `idx_image_indices_namespace` (`namespace`),
  KEY `idx_image_indices_image` (`image`(191)),
  KEY `idx_image_indices_image_name` (`image_name`(191)),
  KEY `idx_image_indices_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -- token_blacklists ------------------------------------------------------
CREATE TABLE IF NOT EXISTS `token_blacklists` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `jti`        varchar(64)     NOT NULL,
  `user_id`    bigint unsigned NOT NULL DEFAULT 0,
  `reason`     varchar(64)     NOT NULL DEFAULT '',
  `expires_at` datetime(3)     NOT NULL,
  `created_at` datetime(3)     NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_token_blacklists_jti` (`jti`),
  KEY `idx_token_blacklists_user_id` (`user_id`),
  KEY `idx_token_blacklists_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET foreign_key_checks = 1;

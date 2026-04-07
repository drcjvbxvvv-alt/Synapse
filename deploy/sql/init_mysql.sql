-- =============================================================================
-- Synapse — MySQL 初始化 SQL
-- 適用：MySQL 8.0+（utf8mb4_unicode_ci）
-- 用法：mysql -h <host> -u <user> -p <database> < init_mysql.sql
--
-- 注意：
--   - 首次部署建議由 DBA 審查後執行
--   - 後續 schema 異動由 GORM AutoMigrate 負責
--   - 預設管理員：admin / Synapse@2026（首次登入後請立即修改密碼）
-- =============================================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;
SET time_zone = '+00:00';

-- -----------------------------------------------------------------------------
-- 1. users
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `users` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `username`      varchar(50)  NOT NULL,
  `password_hash` varchar(255) NOT NULL DEFAULT '',
  `salt`          varchar(32)  NOT NULL DEFAULT '',
  `email`         varchar(100) NOT NULL DEFAULT '',
  `display_name`  varchar(100) NOT NULL DEFAULT '',
  `phone`         varchar(20)  NOT NULL DEFAULT '',
  `auth_type`     varchar(20)  NOT NULL DEFAULT 'local',
  `status`        varchar(20)  NOT NULL DEFAULT 'active',
  `last_login_at` datetime(3)  DEFAULT NULL,
  `last_login_ip` varchar(50)  NOT NULL DEFAULT '',
  `created_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`    datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_username` (`username`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 2. clusters
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `clusters` (
  `id`                   bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`                 varchar(100) NOT NULL,
  `api_server`           varchar(255) NOT NULL,
  `kubeconfig_enc`       longtext,
  `ca_enc`               longtext,
  `sa_token_enc`         longtext,
  `version`              varchar(50)  NOT NULL DEFAULT '',
  `status`               varchar(20)  NOT NULL DEFAULT 'unknown',
  `labels`               json         DEFAULT NULL,
  `cert_expire_at`       datetime(3)  DEFAULT NULL,
  `last_heartbeat`       datetime(3)  DEFAULT NULL,
  `created_by`           bigint unsigned NOT NULL DEFAULT 0,
  `monitoring_config`    json         DEFAULT NULL,
  `alertmanager_config`  json         DEFAULT NULL,
  `created_at`           datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`           datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`           datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_clusters_name` (`name`),
  KEY `idx_clusters_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 3. cluster_metrics
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cluster_metrics` (
  `cluster_id`    bigint unsigned NOT NULL,
  `node_count`    int NOT NULL DEFAULT 0,
  `ready_nodes`   int NOT NULL DEFAULT 0,
  `pod_count`     int NOT NULL DEFAULT 0,
  `running_pods`  int NOT NULL DEFAULT 0,
  `cpu_usage`     double NOT NULL DEFAULT 0,
  `memory_usage`  double NOT NULL DEFAULT 0,
  `storage_usage` double NOT NULL DEFAULT 0,
  `updated_at`    datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 4. terminal_sessions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `terminal_sessions` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`     bigint unsigned NOT NULL,
  `cluster_id`  bigint unsigned NOT NULL,
  `target_type` varchar(20)  NOT NULL,
  `target_ref`  json         DEFAULT NULL,
  `namespace`   varchar(100) NOT NULL DEFAULT '',
  `pod`         varchar(100) NOT NULL DEFAULT '',
  `container`   varchar(100) NOT NULL DEFAULT '',
  `node`        varchar(100) NOT NULL DEFAULT '',
  `start_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `end_at`      datetime(3)  DEFAULT NULL,
  `input_size`  bigint       NOT NULL DEFAULT 0,
  `status`      varchar(20)  NOT NULL DEFAULT 'active',
  `created_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`  datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_terminal_sessions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 5. terminal_commands
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `terminal_commands` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `timestamp`  datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `raw_input`  longtext,
  `parsed_cmd` varchar(1024) NOT NULL DEFAULT '',
  `exit_code`  int          DEFAULT NULL,
  `created_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_terminal_commands_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 6. audit_logs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `audit_logs` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`       bigint unsigned NOT NULL,
  `action`        varchar(100) NOT NULL,
  `resource_type` varchar(50)  NOT NULL,
  `resource_ref`  json         DEFAULT NULL,
  `result`        varchar(20)  NOT NULL,
  `ip`            varchar(45)  NOT NULL DEFAULT '',
  `user_agent`    varchar(500) NOT NULL DEFAULT '',
  `details`       longtext,
  `created_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_audit_logs_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 7. operation_logs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `operation_logs` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`       bigint unsigned DEFAULT NULL,
  `username`      varchar(100) NOT NULL DEFAULT '',
  `method`        varchar(10)  NOT NULL DEFAULT '',
  `path`          varchar(500) NOT NULL DEFAULT '',
  `query`         varchar(1000) NOT NULL DEFAULT '',
  `module`        varchar(50)  NOT NULL DEFAULT '',
  `action`        varchar(100) NOT NULL DEFAULT '',
  `cluster_id`    bigint unsigned DEFAULT NULL,
  `cluster_name`  varchar(100) NOT NULL DEFAULT '',
  `namespace`     varchar(100) NOT NULL DEFAULT '',
  `resource_type` varchar(50)  NOT NULL DEFAULT '',
  `resource_name` varchar(200) NOT NULL DEFAULT '',
  `request_body`  longtext,
  `status_code`   int          NOT NULL DEFAULT 0,
  `success`       tinyint(1)   NOT NULL DEFAULT 0,
  `error_message` varchar(1000) NOT NULL DEFAULT '',
  `client_ip`     varchar(45)  NOT NULL DEFAULT '',
  `user_agent`    varchar(500) NOT NULL DEFAULT '',
  `duration`      bigint       NOT NULL DEFAULT 0,
  `created_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_op_user_time` (`user_id`, `created_at`),
  KEY `idx_operation_logs_username` (`username`),
  KEY `idx_operation_logs_method` (`method`),
  KEY `idx_operation_logs_module` (`module`),
  KEY `idx_operation_logs_action` (`action`),
  KEY `idx_operation_logs_cluster_id` (`cluster_id`),
  KEY `idx_operation_logs_success` (`success`),
  KEY `idx_operation_logs_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 8. system_settings
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `system_settings` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `config_key` varchar(100) NOT NULL,
  `value`      longtext,
  `type`       varchar(50)  NOT NULL DEFAULT '',
  `created_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_system_settings_config_key` (`config_key`),
  KEY `idx_system_settings_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 9. argocd_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `argocd_configs` (
  `id`                   bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`           bigint unsigned NOT NULL,
  `enabled`              tinyint(1)   NOT NULL DEFAULT 0,
  `server_url`           varchar(255) NOT NULL DEFAULT '',
  `auth_type`            varchar(20)  NOT NULL DEFAULT '',
  `token`                longtext,
  `username`             varchar(100) NOT NULL DEFAULT '',
  `password`             longtext,
  `insecure`             tinyint(1)   NOT NULL DEFAULT 0,
  `git_repo_url`         varchar(500) NOT NULL DEFAULT '',
  `git_branch`           varchar(100) NOT NULL DEFAULT 'main',
  `git_path`             varchar(255) NOT NULL DEFAULT '',
  `git_auth_type`        varchar(20)  NOT NULL DEFAULT '',
  `git_username`         varchar(100) NOT NULL DEFAULT '',
  `git_password`         longtext,
  `git_ssh_key`          longtext,
  `argo_cd_cluster_name` varchar(100) NOT NULL DEFAULT '',
  `argo_cd_project`      varchar(100) NOT NULL DEFAULT 'default',
  `connection_status`    varchar(20)  NOT NULL DEFAULT '',
  `last_test_at`         datetime(3)  DEFAULT NULL,
  `error_message`        longtext,
  `created_at`           datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`           datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`           datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_argocd_configs_cluster_id` (`cluster_id`),
  KEY `idx_argocd_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 10. user_groups
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user_groups` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`        varchar(50)  NOT NULL,
  `description` varchar(255) NOT NULL DEFAULT '',
  `created_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`  datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_groups_name` (`name`),
  KEY `idx_user_groups_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 11. user_group_members
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user_group_members` (
  `user_id`       bigint unsigned NOT NULL,
  `user_group_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`user_id`, `user_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 12. cluster_permissions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cluster_permissions` (
  `id`              bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`      bigint unsigned NOT NULL,
  `user_id`         bigint unsigned DEFAULT NULL,
  `user_group_id`   bigint unsigned DEFAULT NULL,
  `permission_type` varchar(50)  NOT NULL,
  `namespaces`      longtext,
  `custom_role_ref` varchar(200) NOT NULL DEFAULT '',
  `created_at`      datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`      datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`      datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_cluster_permissions_cluster_id` (`cluster_id`),
  KEY `idx_cluster_permissions_user_id` (`user_id`),
  KEY `idx_cluster_permissions_user_group_id` (`user_group_id`),
  KEY `idx_cluster_permissions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 13. ai_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `ai_configs` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `provider`    varchar(50)  NOT NULL DEFAULT 'openai',
  `endpoint`    varchar(255) NOT NULL DEFAULT '',
  `api_key`     longtext,
  `model`       varchar(100) NOT NULL DEFAULT '',
  `api_version` varchar(50)  NOT NULL DEFAULT '',
  `enabled`     tinyint(1)   NOT NULL DEFAULT 0,
  `created_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`  datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ai_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 14. helm_repositories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `helm_repositories` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`       varchar(128) NOT NULL,
  `url`        varchar(512) NOT NULL DEFAULT '',
  `username`   varchar(256) NOT NULL DEFAULT '',
  `password`   varchar(256) NOT NULL DEFAULT '',
  `created_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_helm_repositories_name` (`name`),
  KEY `idx_helm_repositories_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 15. event_alert_rules
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `event_alert_rules` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`   bigint unsigned NOT NULL,
  `name`         varchar(100) NOT NULL,
  `description`  varchar(255) NOT NULL DEFAULT '',
  `namespace`    varchar(100) NOT NULL DEFAULT '',
  `event_reason` varchar(100) NOT NULL DEFAULT '',
  `event_type`   varchar(20)  NOT NULL DEFAULT '',
  `min_count`    int          NOT NULL DEFAULT 1,
  `notify_type`  varchar(20)  NOT NULL DEFAULT '',
  `notify_url`   varchar(500) NOT NULL DEFAULT '',
  `enabled`      tinyint(1)   NOT NULL DEFAULT 1,
  `created_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`   datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_event_alert_rules_cluster_id` (`cluster_id`),
  KEY `idx_event_alert_rules_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 16. event_alert_histories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `event_alert_histories` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `rule_id`       bigint unsigned NOT NULL,
  `cluster_id`    bigint unsigned NOT NULL,
  `rule_name`     varchar(100) NOT NULL DEFAULT '',
  `namespace`     varchar(100) NOT NULL DEFAULT '',
  `event_reason`  varchar(100) NOT NULL DEFAULT '',
  `event_type`    varchar(20)  NOT NULL DEFAULT '',
  `message`       longtext,
  `involved_obj`  varchar(200) NOT NULL DEFAULT '',
  `notify_result` varchar(50)  NOT NULL DEFAULT '',
  `triggered_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_event_alert_histories_rule_id` (`rule_id`),
  KEY `idx_event_alert_histories_cluster_id` (`cluster_id`),
  KEY `idx_event_alert_histories_triggered_at` (`triggered_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 17. cost_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cost_configs` (
  `id`                bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`        bigint unsigned NOT NULL,
  `cpu_price_per_core` double NOT NULL DEFAULT 0.048,
  `mem_price_per_gi_b` double NOT NULL DEFAULT 0.006,
  `currency`          varchar(10) NOT NULL DEFAULT 'USD',
  `updated_at`        datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cost_configs_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 18. resource_snapshots
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `resource_snapshots` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`  bigint unsigned NOT NULL,
  `namespace`   varchar(128) NOT NULL,
  `workload`    varchar(256) NOT NULL,
  `date`        datetime(3)  NOT NULL,
  `cpu_request` double NOT NULL DEFAULT 0,
  `cpu_usage`   double NOT NULL DEFAULT 0,
  `mem_request` double NOT NULL DEFAULT 0,
  `mem_usage`   double NOT NULL DEFAULT 0,
  `pod_count`   int    NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_resource_snapshots_cluster_id` (`cluster_id`),
  KEY `idx_resource_snapshots_date` (`date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 19. cluster_occupancy_snapshots
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cluster_occupancy_snapshots` (
  `id`                 bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`         bigint unsigned NOT NULL,
  `date`               datetime(3)  NOT NULL,
  `allocatable_cpu`    double NOT NULL DEFAULT 0,
  `allocatable_memory` double NOT NULL DEFAULT 0,
  `requested_cpu`      double NOT NULL DEFAULT 0,
  `requested_memory`   double NOT NULL DEFAULT 0,
  `node_count`         int    NOT NULL DEFAULT 0,
  `pod_count`          int    NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_date` (`cluster_id`, `date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 20. cloud_billing_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cloud_billing_configs` (
  `id`                      bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`              bigint unsigned NOT NULL,
  `provider`                varchar(20)  NOT NULL DEFAULT 'disabled',
  `aws_access_key_id`       varchar(128) NOT NULL DEFAULT '',
  `aws_secret_access_key`   varchar(256) NOT NULL DEFAULT '',
  `aws_region`              varchar(32)  NOT NULL DEFAULT 'us-east-1',
  `aws_linked_account_id`   varchar(20)  NOT NULL DEFAULT '',
  `gcp_project_id`          varchar(128) NOT NULL DEFAULT '',
  `gcp_billing_account_id`  varchar(64)  NOT NULL DEFAULT '',
  `gcp_service_account_json` longtext,
  `last_synced_at`          datetime(3)  DEFAULT NULL,
  `last_error`              varchar(512) NOT NULL DEFAULT '',
  `created_at`              datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`              datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cloud_billing_configs_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 21. cloud_billing_records
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `cloud_billing_records` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL,
  `month`      varchar(7)   NOT NULL,
  `provider`   varchar(20)  NOT NULL DEFAULT '',
  `service`    varchar(256) NOT NULL DEFAULT '',
  `amount`     double       NOT NULL DEFAULT 0,
  `currency`   varchar(10)  NOT NULL DEFAULT 'USD',
  `created_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_cloud_billing_records_cluster_id` (`cluster_id`),
  KEY `idx_cloud_billing_records_month` (`month`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 22. image_scan_results
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `image_scan_results` (
  `id`             bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`     bigint unsigned NOT NULL,
  `namespace`      varchar(100) NOT NULL DEFAULT '',
  `pod_name`       varchar(255) NOT NULL DEFAULT '',
  `container_name` varchar(255) NOT NULL DEFAULT '',
  `image`          varchar(512) NOT NULL,
  `status`         varchar(20)  NOT NULL DEFAULT 'pending',
  `critical`       int          NOT NULL DEFAULT 0,
  `high`           int          NOT NULL DEFAULT 0,
  `medium`         int          NOT NULL DEFAULT 0,
  `low`            int          NOT NULL DEFAULT 0,
  `unknown`        int          NOT NULL DEFAULT 0,
  `result_json`    longtext,
  `error`          varchar(512) NOT NULL DEFAULT '',
  `scanned_at`     datetime(3)  DEFAULT NULL,
  `created_at`     datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`     datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_image_scan_results_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 23. bench_results
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `bench_results` (
  `id`          bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`  bigint unsigned NOT NULL,
  `status`      varchar(20)  NOT NULL DEFAULT 'pending',
  `pass`        int          NOT NULL DEFAULT 0,
  `fail`        int          NOT NULL DEFAULT 0,
  `warn`        int          NOT NULL DEFAULT 0,
  `info`        int          NOT NULL DEFAULT 0,
  `score`       double       NOT NULL DEFAULT 0,
  `result_json` longtext,
  `error`       varchar(512) NOT NULL DEFAULT '',
  `job_name`    varchar(255) NOT NULL DEFAULT '',
  `run_at`      datetime(3)  DEFAULT NULL,
  `created_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`  datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_bench_results_cluster_id` (`cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 24. sync_policies
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `sync_policies` (
  `id`               bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`             varchar(128) NOT NULL,
  `description`      varchar(512) NOT NULL DEFAULT '',
  `source_cluster_id` bigint unsigned NOT NULL,
  `source_namespace` varchar(128) NOT NULL,
  `resource_type`    varchar(32)  NOT NULL,
  `resource_names`   longtext,
  `target_clusters`  longtext,
  `conflict_policy`  varchar(16)  NOT NULL DEFAULT 'skip',
  `schedule`         varchar(64)  NOT NULL DEFAULT '',
  `enabled`          tinyint(1)   NOT NULL DEFAULT 1,
  `last_sync_at`     datetime(3)  DEFAULT NULL,
  `last_sync_status` varchar(16)  NOT NULL DEFAULT '',
  `created_at`       datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`       datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_sync_policies_source_cluster_id` (`source_cluster_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 25. sync_histories
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `sync_histories` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `policy_id`    bigint unsigned NOT NULL,
  `triggered_by` varchar(64)  NOT NULL DEFAULT '',
  `status`       varchar(16)  NOT NULL DEFAULT '',
  `message`      longtext,
  `details`      longtext,
  `started_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `finished_at`  datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_sync_histories_policy_id` (`policy_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 26. config_versions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `config_versions` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`    bigint unsigned NOT NULL,
  `resource_type` varchar(20)  NOT NULL,
  `namespace`     varchar(255) NOT NULL,
  `name`          varchar(255) NOT NULL,
  `version`       int          NOT NULL,
  `content_json`  longtext,
  `changed_by`    varchar(100) NOT NULL DEFAULT '',
  `changed_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_config_ver` (`cluster_id`, `resource_type`, `namespace`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 27. log_source_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `log_source_configs` (
  `id`         bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id` bigint unsigned NOT NULL,
  `type`       varchar(20)  NOT NULL DEFAULT '',
  `name`       varchar(100) NOT NULL DEFAULT '',
  `url`        varchar(255) NOT NULL DEFAULT '',
  `username`   varchar(100) NOT NULL DEFAULT '',
  `password`   varchar(255) NOT NULL DEFAULT '',
  `api_key`    varchar(255) NOT NULL DEFAULT '',
  `enabled`    tinyint(1)   NOT NULL DEFAULT 1,
  `created_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_log_source_configs_cluster_id` (`cluster_id`),
  KEY `idx_log_source_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 28. namespace_protections
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `namespace_protections` (
  `id`               bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`       bigint unsigned NOT NULL,
  `namespace`        varchar(253) NOT NULL,
  `require_approval` tinyint(1)   NOT NULL DEFAULT 0,
  `description`      longtext,
  `created_at`       datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`       datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`       datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ns_protection` (`cluster_id`, `namespace`),
  KEY `idx_namespace_protections_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 29. approval_requests
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `approval_requests` (
  `id`             bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`     bigint unsigned NOT NULL,
  `cluster_name`   longtext,
  `namespace`      varchar(253) NOT NULL,
  `resource_kind`  longtext     NOT NULL,
  `resource_name`  longtext     NOT NULL,
  `action`         longtext     NOT NULL,
  `requester_id`   bigint unsigned NOT NULL,
  `requester_name` longtext,
  `approver_id`    bigint unsigned DEFAULT NULL,
  `approver_name`  longtext,
  `status`         longtext     NOT NULL DEFAULT 'pending',
  `payload`        longtext,
  `reason`         longtext,
  `expires_at`     datetime(3)  NOT NULL,
  `approved_at`    datetime(3)  DEFAULT NULL,
  `created_at`     datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`     datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`     datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_approval_requests_cluster_id` (`cluster_id`),
  KEY `idx_approval_requests_namespace` (`namespace`),
  KEY `idx_approval_requests_status` (`status`(191)),
  KEY `idx_approval_requests_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 30. image_indices
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `image_indices` (
  `id`             bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`     bigint unsigned NOT NULL,
  `cluster_name`   longtext,
  `namespace`      varchar(253) NOT NULL,
  `workload_kind`  varchar(64)  NOT NULL,
  `workload_name`  varchar(253) NOT NULL,
  `container_name` varchar(253) NOT NULL,
  `image`          varchar(512) NOT NULL,
  `image_name`     varchar(512) NOT NULL,
  `image_tag`      longtext,
  `last_sync_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `created_at`     datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`     datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`     datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_image_indices_cluster_id` (`cluster_id`),
  KEY `idx_image_indices_namespace` (`namespace`),
  KEY `idx_image_indices_image` (`image`(255)),
  KEY `idx_image_indices_image_name` (`image_name`(255)),
  KEY `idx_image_indices_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 31. port_forward_sessions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `port_forward_sessions` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `cluster_id`   bigint unsigned NOT NULL,
  `cluster_name` longtext,
  `namespace`    longtext     NOT NULL,
  `pod_name`     longtext     NOT NULL,
  `pod_port`     int          NOT NULL,
  `local_port`   int          NOT NULL,
  `user_id`      bigint unsigned NOT NULL,
  `username`     longtext,
  `status`       longtext     NOT NULL DEFAULT 'active',
  `stopped_at`   datetime(3)  DEFAULT NULL,
  `created_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`   datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_port_forward_sessions_cluster_id` (`cluster_id`),
  KEY `idx_port_forward_sessions_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 32. siem_webhook_configs
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `siem_webhook_configs` (
  `id`            bigint unsigned NOT NULL AUTO_INCREMENT,
  `enabled`       tinyint(1)   NOT NULL DEFAULT 0,
  `webhook_url`   longtext     NOT NULL,
  `secret_header` longtext,
  `secret_value`  longtext,
  `batch_size`    int          NOT NULL DEFAULT 100,
  `created_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`    datetime(3)  DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_siem_webhook_configs_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 33. api_tokens
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `api_tokens` (
  `id`           bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id`      bigint unsigned NOT NULL,
  `name`         varchar(100) NOT NULL,
  `token_hash`   varchar(64)  NOT NULL,
  `scopes`       varchar(200) NOT NULL DEFAULT '',
  `expires_at`   datetime(3)  DEFAULT NULL,
  `last_used_at` datetime(3)  DEFAULT NULL,
  `created_at`   datetime(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_api_tokens_token_hash` (`token_hash`),
  KEY `idx_api_tokens_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 34. notify_channels
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `notify_channels` (
  `id`               bigint unsigned NOT NULL AUTO_INCREMENT,
  `name`             varchar(100)  NOT NULL,
  `type`             varchar(20)   NOT NULL,
  `webhook_url`      varchar(1000) NOT NULL,
  `telegram_chat_id` varchar(200)  NOT NULL DEFAULT '',
  `description`      varchar(255)  NOT NULL DEFAULT '',
  `enabled`          tinyint(1)    NOT NULL DEFAULT 1,
  `created_at`       datetime(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`       datetime(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at`       datetime(3)   DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_notify_channels_name` (`name`),
  KEY `idx_notify_channels_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;

-- =============================================================================
-- 初始資料（Seed Data）
-- =============================================================================

-- 預設管理員（admin / Synapse@2026）
-- 密碼為 bcrypt(cost=10) of "Synapse@2026synapse_salt"
-- ⚠️  首次登入後請立即至「個人設定」修改密碼
INSERT INTO `users`
  (`username`, `password_hash`, `salt`, `email`, `display_name`, `auth_type`, `status`, `created_at`, `updated_at`)
VALUES (
  'admin',
  '$2a$10$4./Tk094GlfLRsNU4nhYQ.FrxyxW/91G2ajMAY/oD0GgGkxEqKSG.',
  'synapse_salt',
  'admin@synapse.io',
  '管理員',
  'local',
  'active',
  NOW(3),
  NOW(3)
) ON DUPLICATE KEY UPDATE `id` = `id`;

-- 預設 LDAP 設定（停用狀態，需在 UI 中啟用）
INSERT INTO `system_settings`
  (`config_key`, `value`, `type`, `created_at`, `updated_at`)
VALUES (
  'ldap_config',
  '{"enabled":false,"server":"","port":389,"use_tls":false,"skip_tls_verify":false,"bind_dn":"","bind_password":"","base_dn":"","user_filter":"(uid=%s)","username_attr":"uid","email_attr":"mail","display_name_attr":"cn","group_filter":"(memberUid=%s)","group_attr":"cn"}',
  'ldap',
  NOW(3),
  NOW(3)
) ON DUPLICATE KEY UPDATE `id` = `id`;

-- 預設安全設定
INSERT INTO `system_settings`
  (`config_key`, `value`, `type`, `created_at`, `updated_at`)
VALUES (
  'security_config',
  '{"session_ttl_minutes":480,"login_fail_lock_threshold":5,"lock_duration_minutes":30,"password_min_length":8}',
  'security',
  NOW(3),
  NOW(3)
) ON DUPLICATE KEY UPDATE `id` = `id`;

-- 預設 Grafana 設定（空，需在 UI 中填入）
INSERT INTO `system_settings`
  (`config_key`, `value`, `type`, `created_at`, `updated_at`)
VALUES (
  'grafana_config',
  '{"url":"","api_key":""}',
  'grafana',
  NOW(3),
  NOW(3)
) ON DUPLICATE KEY UPDATE `id` = `id`;

-- 預設使用者組
INSERT INTO `user_groups` (`name`, `description`, `created_at`, `updated_at`)
VALUES
  ('運維組', '運維團隊成員，擁有運維權限', NOW(3), NOW(3)),
  ('開發組', '開發團隊成員，擁有開發權限', NOW(3), NOW(3)),
  ('只讀組', '只讀權限使用者組',           NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE `id` = `id`;

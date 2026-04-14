-- Migration 001 rollback: drop all tables in reverse dependency order.
-- WARNING: This permanently deletes ALL data. Use only in development.

DROP TABLE IF EXISTS token_blacklists;
DROP TABLE IF EXISTS image_indices;
DROP TABLE IF EXISTS log_source_configs;
DROP TABLE IF EXISTS notify_channels;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS siem_webhook_configs;
DROP TABLE IF EXISTS port_forward_sessions;
DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS namespace_protections;
DROP TABLE IF EXISTS config_versions;
DROP TABLE IF EXISTS sync_histories;
DROP TABLE IF EXISTS sync_policies;
DROP TABLE IF EXISTS bench_results;
DROP TABLE IF EXISTS image_scan_results;
DROP TABLE IF EXISTS cloud_billing_records;
DROP TABLE IF EXISTS cloud_billing_configs;
DROP TABLE IF EXISTS cluster_occupancy_snapshots;
DROP TABLE IF EXISTS resource_snapshots;
DROP TABLE IF EXISTS cost_configs;
DROP TABLE IF EXISTS event_alert_histories;
DROP TABLE IF EXISTS event_alert_rules;
DROP TABLE IF EXISTS helm_repositories;
DROP TABLE IF EXISTS ai_configs;
DROP TABLE IF EXISTS argocd_configs;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS operation_logs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS terminal_commands;
DROP TABLE IF EXISTS terminal_sessions;
DROP TABLE IF EXISTS cluster_permissions;
DROP TABLE IF EXISTS cluster_metrics;
DROP TABLE IF EXISTS clusters;
DROP TABLE IF EXISTS user_group_members;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS users;

-- P2-6: Feature Flag DB-backed store
-- Creates feature_flags table and seeds all known flag keys so the admin
-- UI immediately shows every flag even before it has been toggled.

CREATE TABLE IF NOT EXISTS `feature_flags` (
  `key`         varchar(100) NOT NULL,
  `enabled`     tinyint(1)   NOT NULL DEFAULT 0,
  `description` varchar(500) NOT NULL DEFAULT '',
  `updated_by`  varchar(100) NOT NULL DEFAULT '',
  `created_at`  datetime(3)  NOT NULL,
  `updated_at`  datetime(3)  NOT NULL,
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Seed known flags with their default (disabled) state.
-- INSERT IGNORE keeps existing rows intact if a re-migration runs.
INSERT IGNORE INTO `feature_flags` (`key`, `enabled`, `description`, `created_at`, `updated_at`) VALUES
  ('use_repo_layer',        0, 'P0-4: Route DB access through Repository layer instead of raw *gorm.DB in services', NOW(3), NOW(3)),
  ('use_split_router',      0, 'P1-2: Use the split router module structure',                                         NOW(3), NOW(3)),
  ('enable_otel_tracing',   0, 'P1-10: Enable OpenTelemetry distributed tracing',                                     NOW(3), NOW(3)),
  ('use_redis_ratelimit',   0, 'P1-8: Use Redis-backed rate limiter for multi-pod deployments',                       NOW(3), NOW(3)),
  ('use_zustand_store',     1, 'P2-5: Zustand frontend global state (session/cluster/UI)',                            NOW(3), NOW(3)),
  ('enable_audit_hashchain',0, 'P2-2: Enable SHA-256 audit log hash-chain verification',                             NOW(3), NOW(3));

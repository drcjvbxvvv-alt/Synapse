-- P2-6: Feature Flag DB-backed store
-- Creates feature_flags table and seeds all known flag keys so the admin
-- UI immediately shows every flag even before it has been toggled.

CREATE TABLE IF NOT EXISTS feature_flags (
  key         varchar(100) NOT NULL,
  enabled     boolean      NOT NULL DEFAULT false,
  description varchar(500) NOT NULL DEFAULT '',
  updated_by  varchar(100) NOT NULL DEFAULT '',
  created_at  timestamptz(3)  NOT NULL,
  updated_at  timestamptz(3)  NOT NULL,
  PRIMARY KEY (key)
);

-- Seed known flags with their default (disabled) state.
-- ON CONFLICT DO NOTHING keeps existing rows intact if a re-migration runs.
INSERT INTO feature_flags (key, enabled, description, created_at, updated_at) VALUES
  ('use_repo_layer',        false, 'P0-4: Route DB access through Repository layer instead of raw *gorm.DB in services', NOW(), NOW()),
  ('use_split_router',      false, 'P1-2: Use the split router module structure',                                         NOW(), NOW()),
  ('enable_otel_tracing',   false, 'P1-10: Enable OpenTelemetry distributed tracing',                                     NOW(), NOW()),
  ('use_redis_ratelimit',   false, 'P1-8: Use Redis-backed rate limiter for multi-pod deployments',                       NOW(), NOW()),
  ('use_zustand_store',     true,  'P2-5: Zustand frontend global state (session/cluster/UI)',                            NOW(), NOW()),
  ('enable_audit_hashchain',false, 'P2-2: Enable SHA-256 audit log hash-chain verification',                             NOW(), NOW())
ON CONFLICT DO NOTHING;

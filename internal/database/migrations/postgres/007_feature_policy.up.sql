-- 007: Add feature_policy column to cluster_permissions
-- NULL means "use permission_type defaults" — fully backward-compatible.
ALTER TABLE cluster_permissions
  ADD COLUMN IF NOT EXISTS feature_policy text NULL;

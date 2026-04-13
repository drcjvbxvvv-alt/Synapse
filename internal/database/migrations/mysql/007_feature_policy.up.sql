-- 007: Add feature_policy column to cluster_permissions
-- NULL means "use permission_type defaults" — fully backward-compatible.
-- IF NOT EXISTS makes this idempotent (MySQL 8.0+).
ALTER TABLE `cluster_permissions`
  ADD COLUMN IF NOT EXISTS `feature_policy` TEXT NULL AFTER `custom_role_ref`;

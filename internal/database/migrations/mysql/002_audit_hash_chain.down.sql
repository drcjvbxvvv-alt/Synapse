-- Reverse P2-2: remove hash-chain columns from audit_logs
ALTER TABLE `audit_logs`
  DROP INDEX  `idx_audit_logs_hash`,
  DROP COLUMN `hash`,
  DROP COLUMN `prev_hash`;

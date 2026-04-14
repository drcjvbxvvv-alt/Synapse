-- Reverse P2-2: remove hash-chain columns from audit_logs
DROP INDEX IF EXISTS idx_audit_logs_hash;
ALTER TABLE audit_logs
  DROP COLUMN IF EXISTS hash,
  DROP COLUMN IF EXISTS prev_hash;

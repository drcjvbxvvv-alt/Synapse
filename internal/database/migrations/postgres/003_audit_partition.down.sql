-- Migration 003 rollback: Collapse the partitioned audit_logs back to a
-- plain table. Use only for emergency rollback — data written to partitions
-- after the up migration will be preserved but the partitioned structure
-- and its partition-pruning benefits are lost.

-- Step 1: Snapshot all partition data into a plain table.
CREATE TABLE audit_logs_rollback AS SELECT * FROM audit_logs;

-- Step 2: Drop the partitioned table (cascades to all child partitions).
DROP TABLE audit_logs;

-- Step 3: Promote the snapshot to the canonical table.
ALTER TABLE audit_logs_rollback RENAME TO audit_logs;

-- Step 4: Restore the original primary key and indexes.
ALTER TABLE audit_logs ADD PRIMARY KEY (id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id    ON audit_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_hash       ON audit_logs (hash);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at);

-- Re-attach the sequence to the restored table.
ALTER SEQUENCE audit_logs_id_seq OWNED BY audit_logs.id;

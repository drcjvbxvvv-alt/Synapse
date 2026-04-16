-- Migration 003: Convert audit_logs to a monthly range-partitioned table
--
-- Rationale: a single un-partitioned audit_logs table degrades on large
-- deployments because every query requires a full-table or single-index scan.
-- Monthly RANGE partitioning lets PostgreSQL prune irrelevant partitions and
-- allows zero-I/O removal of old data via DROP TABLE on a child partition.
--
-- After this migration the application must call
-- AuditService.EnsureNextMonthPartition() on startup (and monthly) to
-- create future partitions ahead of time.
--
-- ⚠ IMPORTANT: Run this during a maintenance window.
--   The INSERT … SELECT step holds an AccessShareLock on the legacy table
--   and an AccessExclusiveLock on the new partitioned table for its duration.

-- Step 1: Detach the auto-increment sequence so it survives the rename.
ALTER SEQUENCE audit_logs_id_seq OWNED BY NONE;

-- Step 2: Rename the existing table to a backup so we can recover data.
ALTER TABLE audit_logs RENAME TO audit_logs_pre_partition;

-- Step 3: Create the new partitioned parent table.
CREATE TABLE audit_logs (
    id            bigint       NOT NULL DEFAULT nextval('audit_logs_id_seq'),
    user_id       bigint       NOT NULL,
    action        varchar(100) NOT NULL,
    resource_type varchar(50)  NOT NULL DEFAULT '',
    resource_ref  text         DEFAULT NULL,
    result        varchar(10)  DEFAULT NULL,
    ip            varchar(45)  DEFAULT NULL,
    user_agent    text,
    details       text,
    prev_hash     varchar(64)  NOT NULL DEFAULT '',
    hash          varchar(64)  NOT NULL DEFAULT '',
    created_at    timestamptz  NOT NULL DEFAULT now(),
    -- Partition key (created_at) must be part of every unique constraint.
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Re-attach the sequence to the new table.
ALTER SEQUENCE audit_logs_id_seq OWNED BY audit_logs.id;

-- Step 4: Create initial monthly partitions.
-- The application will extend these via EnsureNextMonthPartition.
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_logs_2026_05 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE audit_logs_2026_06 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- Catch-all partition for any out-of-range rows (safety net).
CREATE TABLE audit_logs_default PARTITION OF audit_logs DEFAULT;

-- Step 5: Create indexes on the parent table.
-- PostgreSQL 11+ automatically propagates these to all current and future
-- child partitions, so EnsureNextMonthPartition does not need to add them.
CREATE INDEX idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC);
CREATE INDEX idx_audit_logs_hash         ON audit_logs (hash);

-- Step 6: Migrate existing data from the backup table.
INSERT INTO audit_logs SELECT * FROM audit_logs_pre_partition;

-- Step 7: Drop the backup table.
DROP TABLE audit_logs_pre_partition;

-- Undo Migration 002: remove Pipeline Rollback support

DROP INDEX IF EXISTS idx_pipeline_runs_rollback_of_run_id;

ALTER TABLE pipeline_runs
  DROP COLUMN IF EXISTS rollback_of_run_id;

-- Migration 002: Pipeline Rollback support
-- Adds rollback_of_run_id column to pipeline_runs so a rollback run can
-- reference the successful run whose artifact images it will re-deploy.

ALTER TABLE pipeline_runs
  ADD COLUMN IF NOT EXISTS rollback_of_run_id bigint;

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_rollback_of_run_id
  ON pipeline_runs (rollback_of_run_id)
  WHERE rollback_of_run_id IS NOT NULL;

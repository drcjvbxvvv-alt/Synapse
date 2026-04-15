-- 008 rollback: drop pipeline tables + revert extensions

-- Revert approval_requests extensions
DROP INDEX IF EXISTS idx_approval_requests_pipeline_run;
ALTER TABLE approval_requests
    DROP COLUMN IF EXISTS to_environment,
    DROP COLUMN IF EXISTS from_environment,
    DROP COLUMN IF EXISTS pipeline_run_id;

-- Revert image_scan_results extensions
DROP INDEX IF EXISTS idx_scan_results_pipeline_run;
ALTER TABLE image_scan_results
    DROP COLUMN IF EXISTS step_run_id,
    DROP COLUMN IF EXISTS pipeline_run_id,
    DROP COLUMN IF EXISTS scan_source;

-- Drop pipeline tables (reverse dependency order)
DROP TABLE IF EXISTS pipeline_logs;
DROP TABLE IF EXISTS pipeline_artifacts;
DROP TABLE IF EXISTS pipeline_secrets;
DROP TABLE IF EXISTS step_runs;

-- Drop FK before dropping referenced table
ALTER TABLE pipelines DROP CONSTRAINT IF EXISTS fk_pipelines_current_version;
DROP INDEX IF EXISTS idx_pipeline_runs_env;
DROP TABLE IF EXISTS pipeline_runs;
DROP TABLE IF EXISTS pipeline_versions;
DROP TABLE IF EXISTS pipelines;

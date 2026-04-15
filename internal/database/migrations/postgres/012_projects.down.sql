-- 012_projects.down.sql
ALTER TABLE pipelines DROP COLUMN IF EXISTS project_id;
DROP TABLE IF EXISTS projects;

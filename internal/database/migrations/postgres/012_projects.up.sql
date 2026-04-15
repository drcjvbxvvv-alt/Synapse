-- 012_projects.up.sql
-- M14.1: Project layer — maps Git repos to Pipelines for precise webhook matching.

CREATE TABLE IF NOT EXISTS projects (
    id              BIGSERIAL PRIMARY KEY,
    git_provider_id BIGINT        NOT NULL REFERENCES git_providers(id) ON DELETE CASCADE,
    name            VARCHAR(255)  NOT NULL,
    repo_url        VARCHAR(512)  NOT NULL,
    default_branch  VARCHAR(255)  NOT NULL DEFAULT 'main',
    description     TEXT,
    created_by      BIGINT        NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (repo_url)
);

CREATE INDEX IF NOT EXISTS idx_projects_git_provider_id ON projects (git_provider_id);
CREATE INDEX IF NOT EXISTS idx_projects_deleted_at       ON projects (deleted_at);

-- Add nullable project_id FK to pipelines (gradual migration — existing rows stay NULL)
ALTER TABLE pipelines
    ADD COLUMN IF NOT EXISTS project_id BIGINT REFERENCES projects(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_pipelines_project_id ON pipelines (project_id);

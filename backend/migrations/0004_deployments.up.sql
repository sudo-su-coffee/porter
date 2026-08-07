-- 0004_deployments: version history / rollback for v0.3 Application Platform.
CREATE TABLE IF NOT EXISTS deployments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision      BIGSERIAL,
    git_url       TEXT,
    git_commit    TEXT,
    build_status  TEXT NOT NULL DEFAULT 'pending'
                  CHECK (build_status IN ('pending','building','ready','failed')),
    image_digest  TEXT,
    manifest      JSONB NOT NULL DEFAULT '{}',
    rollback_to   UUID REFERENCES deployments(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_deployments_project ON deployments(project_id);

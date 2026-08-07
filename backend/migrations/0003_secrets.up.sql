-- 0003_secrets: per-project secrets for v0.2 (env/secrets management).
-- value is stored as an opaque blob; the application encrypts before write
-- (e.g. with a key derived from the porter.toml api token).
CREATE TABLE IF NOT EXISTS secrets (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    value_encrypted  BYTEA NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_secrets_project ON secrets(project_id);

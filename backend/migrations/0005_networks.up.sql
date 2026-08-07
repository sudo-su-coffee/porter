-- 0005_networks: per-project overlay/bridge networks for v0.6 Networking.
CREATE TABLE IF NOT EXISTS networks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    cidr       CIDR,
    plugin     JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_networks_project ON networks(project_id);

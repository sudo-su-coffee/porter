-- 0012: columns the store INSERTs that earlier migrations (0009/0010) omitted.
--   * orgs          -> owner_id (TEXT: the owning username — the auth model keys
--                       users by username, e.g. the bootstrap "admin"; a UUID FK
--                       to users.id breaks that), is_default
--   * health_events -> project_id (AddHealthEvent / ListHealthEvents)
-- Idempotent and safe on existing data.

ALTER TABLE orgs DROP COLUMN IF EXISTS owner_id;
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS owner_id TEXT;
ALTER TABLE orgs ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE health_events ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_orgs_owner         ON orgs(owner_id);
CREATE INDEX IF NOT EXISTS idx_health_events_proj ON health_events(project_id);

-- Rollback 0012.
ALTER TABLE orgs DROP COLUMN IF EXISTS owner_id;
ALTER TABLE orgs DROP COLUMN IF EXISTS is_default;
ALTER TABLE health_events DROP COLUMN IF EXISTS project_id;

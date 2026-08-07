-- Rollback 0013 (re-add NOT NULL; only safe when no rows lack these values).
ALTER TABLE projects ALTER COLUMN bridge_subnet SET NOT NULL;
ALTER TABLE replicas ALTER COLUMN service_id SET NOT NULL;
ALTER TABLE domains ALTER COLUMN service_id SET NOT NULL;

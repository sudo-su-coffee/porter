-- 0013: legacy NOT NULL columns (from 0001/0010) that the v3 store never
-- populates — its inserts omit them (state lives in the row's `data` JSON
-- blob). Drop the NOT NULL so inserts succeed:
--   * projects.bridge_subnet   (subnet is allocated by the handler, stored in data)
--   * replicas.service_id      (legacy v1 services FK; store uses project_id)
--   * domains.service_id       (legacy v1 services FK; store uses project_id)
-- Idempotent and safe on existing data.

ALTER TABLE projects ALTER COLUMN bridge_subnet DROP NOT NULL;
ALTER TABLE replicas ALTER COLUMN service_id DROP NOT NULL;
ALTER TABLE domains ALTER COLUMN service_id DROP NOT NULL;

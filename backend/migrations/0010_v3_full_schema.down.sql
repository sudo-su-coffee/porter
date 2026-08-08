-- 0010_v3_full_schema.down.sql — removes v3 schema objects. 0011 depends on
-- pieces of 0010, so this is best-effort for a fresh-rollback scenario only.
ALTER TABLE projects DROP COLUMN IF EXISTS env;
ALTER TABLE projects DROP COLUMN IF EXISTS healthcheck;
ALTER TABLE projects DROP COLUMN IF EXISTS restart_policy;
ALTER TABLE projects DROP COLUMN IF EXISTS replicas_desired;
ALTER TABLE projects DROP COLUMN IF EXISTS host_mount_path;
ALTER TABLE projects DROP COLUMN IF EXISTS image;
ALTER TABLE projects DROP COLUMN IF EXISTS org_id;

DROP TABLE IF EXISTS group_projects;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS orgs;
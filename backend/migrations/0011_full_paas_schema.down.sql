-- 0011_full_paas_schema: rollback tables/columns added in the .up migration.

ALTER TABLE projects DROP COLUMN IF EXISTS stack_id;
ALTER TABLE projects DROP COLUMN IF EXISTS source;
ALTER TABLE projects DROP COLUMN IF EXISTS model;
ALTER TABLE projects DROP COLUMN IF EXISTS gpu;
ALTER TABLE projects DROP COLUMN IF EXISTS compose_service;
ALTER TABLE projects DROP COLUMN IF EXISTS networks;
ALTER TABLE replicas DROP COLUMN IF EXISTS console_port;
ALTER TABLE replicas DROP COLUMN IF EXISTS ssh_enabled;

DROP TABLE IF EXISTS networks;
DROP TABLE IF EXISTS builds;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS project_settings;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS firewall_rules;
DROP TABLE IF EXISTS redirects;
DROP TABLE IF EXISTS drains;
DROP TABLE IF EXISTS crons;
DROP TABLE IF EXISTS hooks;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS volumes;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS stacks;
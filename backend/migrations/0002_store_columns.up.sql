-- 0002_store_columns: adds a JSONB `data` blob to every core table so the Go
-- store can round-trip domain structs faithfully while the typed columns from
-- 0001 remain populated for SQL queries. Also adds the `users` and `services`
-- tables the store and API need (0001 covered ssh_keys/servers only).

ALTER TABLE projects ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}';
ALTER TABLE vms      ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}';
ALTER TABLE domains  ADD COLUMN IF NOT EXISTS vm_id UUID;
ALTER TABLE domains  ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}';
ALTER TABLE volumes  ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}';
ALTER TABLE servers  ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL DEFAULT 'admin',
    password_hash TEXT NOT NULL,
    salt          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    data          JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS services (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    data       JSONB NOT NULL DEFAULT '{}',
    UNIQUE (project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_vms_data_state      ON vms(state);
CREATE INDEX IF NOT EXISTS idx_domains_vm_id       ON domains(vm_id);
CREATE INDEX IF NOT EXISTS idx_services_project_id ON services(project_id);

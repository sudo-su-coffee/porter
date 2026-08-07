-- 0001_init: Porter v1 PostgreSQL schema (authoritative — Section 4.2).
-- Projects (compose or single_image), services, microVMs, domains, volumes,
-- ssh_keys, and the Phase-8 servers scaffold.

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for gen_random_uuid()

CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    kind            TEXT NOT NULL CHECK (kind IN ('compose', 'single_image')),
    compose_yaml    TEXT,                      -- null for single_image
    bridge_subnet   CIDR NOT NULL,              -- e.g. 10.42.3.0/24, allocated at create time
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','deploying','running','degraded','stopped','failed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    image           TEXT NOT NULL,
    vcpus           INT NOT NULL DEFAULT 1,
    mem_mib         INT NOT NULL DEFAULT 256,
    replicas_desired INT NOT NULL DEFAULT 1 CHECK (replicas_desired >= 1),
    restart_policy  TEXT NOT NULL DEFAULT 'on-failure',
    healthcheck     JSONB,                      -- {type: http|tcp, path, port, interval_s}
    env             JSONB NOT NULL DEFAULT '{}',
    depends_on      TEXT[] NOT NULL DEFAULT '{}',
    UNIQUE (project_id, name)
);

CREATE TABLE vms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id      UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    replica_index   INT NOT NULL DEFAULT 0,
    container_id    TEXT,                       -- containerd container ID, null until boot starts
    task_id         TEXT,                       -- containerd task ID
    state           TEXT NOT NULL DEFAULT 'pending'
                    CHECK (state IN ('pending','booting','running','stopping','stopped','failed')),
    health_status   TEXT NOT NULL DEFAULT 'checking'
                    CHECK (health_status IN ('healthy','unhealthy','checking')),
    ip_address      INET,
    mac_address     MACADDR,
    ports           JSONB NOT NULL DEFAULT '[]', -- [{container_port, host_port, protocol}]
    started_at      TIMESTAMPTZ,
    crashed         BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (service_id, replica_index)
);

CREATE TABLE domains (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id      UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    domain          TEXT NOT NULL UNIQUE,
    kind            TEXT NOT NULL CHECK (kind IN ('stable','preview','custom')),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','failed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE volumes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    size_mib        INT NOT NULL,
    attached_vm_id  UUID REFERENCES vms(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ssh_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    label           TEXT NOT NULL,
    public_key      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ
);

CREATE TABLE servers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname        TEXT NOT NULL UNIQUE,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vms_service_id    ON vms(service_id);
CREATE INDEX IF NOT EXISTS idx_vms_state         ON vms(state);
CREATE INDEX IF NOT EXISTS idx_services_project  ON services(project_id);
CREATE INDEX IF NOT EXISTS idx_domains_service   ON domains(service_id);

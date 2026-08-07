-- 0010_v3_full_schema: the complete v3 model (API Endpoint Catalog v3).
-- Project = microVM app. Replicas live inside their project. Every user has
-- an auto-created default org; groups are folders inside an org. No managed
-- volumes (persistence is a host_mount_path on the project); no image catalog
-- (image is a plain OCI ref).
--
-- This file is idempotent over the earlier migrations (0001-0009): it creates
-- every table the v3 API needs and adds v3 columns to any table the old schema
-- already created.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- --- orgs: every user auto-gets one (is_default=true); extras are opt-in ---
CREATE TABLE IF NOT EXISTS orgs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    owner_id    UUID NOT NULL,                          -- the user the default org was created for
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, is_default)                        -- at most one default org per owner
);

CREATE TABLE IF NOT EXISTS org_members (
    org_id  UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role    TEXT NOT NULL DEFAULT 'member',
    PRIMARY KEY (org_id, user_id)
);

-- --- groups: lightweight folders for related projects, scoped to an org ---
CREATE TABLE IF NOT EXISTS groups (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id  UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS group_projects (
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, project_id)
);

-- --- projects = the microVM app (services/volumes tables from 0001 are obsolete) ---
ALTER TABLE projects ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES orgs(id) ON DELETE CASCADE;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS image TEXT;               -- set for single_image
ALTER TABLE projects ADD COLUMN IF NOT EXISTS host_mount_path TEXT;     -- optional bind mount (no managed volumes)
ALTER TABLE projects ADD COLUMN IF NOT EXISTS replicas_desired INT NOT NULL DEFAULT 1 CHECK (replicas_desired >= 1);
ALTER TABLE projects ADD COLUMN IF NOT EXISTS restart_policy TEXT NOT NULL DEFAULT 'on-failure';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS healthcheck JSONB;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS env JSONB NOT NULL DEFAULT '{}';

-- --- replicas: one row per Firecracker microVM (renamed from 0001's vms) ---
ALTER TABLE vms RENAME TO replicas;
ALTER TABLE replicas ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
CREATE UNIQUE INDEX IF NOT EXISTS uq_replicas_project_index ON replicas(project_id, replica_index);

-- --- domains: attached to a project (routes to its replica pool) ---
ALTER TABLE domains ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- --- users (bootstrap admin + additional accounts) ---
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL DEFAULT 'admin',
    password_hash TEXT NOT NULL,
    salt          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- deployments: version history / rollback ---
CREATE TABLE IF NOT EXISTS deployments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision      BIGSERIAL,
    build_status  TEXT NOT NULL DEFAULT 'pending' CHECK (build_status IN ('pending','building','ready','failed')),
    image_digest  TEXT,
    rollback_to   UUID REFERENCES deployments(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- secrets: per-project, stored encrypted (app-side) ---
CREATE TABLE IF NOT EXISTS secrets (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    value_encrypted  BYTEA NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

-- --- networks: per-project bridge/overlay networks ---
CREATE TABLE IF NOT EXISTS networks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    cidr       CIDR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

-- --- dns_records: per project (project = VM) ---
CREATE TABLE IF NOT EXISTS dns_records (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL DEFAULT 'A',
    value      TEXT NOT NULL,
    ttl        INT NOT NULL DEFAULT 300,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

-- --- build_logs: rolling per-project build log ---
CREATE TABLE IF NOT EXISTS build_logs (
    id         BIGSERIAL PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    line       TEXT NOT NULL,
    ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- golden_images: image library entries (image is an OCI ref/URL) ---
CREATE TABLE IF NOT EXISTS golden_images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    image       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    vcpus       INT NOT NULL DEFAULT 1,
    mem_mib     INT NOT NULL DEFAULT 256,
    ports       JSONB NOT NULL DEFAULT '[]',
    env         JSONB NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',
    logo        TEXT NOT NULL DEFAULT '',
    version     TEXT NOT NULL DEFAULT 'latest',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- metrics + health_events: v0.9 observability ---
CREATE TABLE IF NOT EXISTS metrics_samples (
    id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vm_id  UUID REFERENCES replicas(id) ON DELETE CASCADE,
    metric TEXT NOT NULL,
    value  DOUBLE PRECISION NOT NULL,
    ts     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS health_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vm_id      UUID REFERENCES replicas(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    status     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    ts         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- servers: Phase-8 multi-host scaffold ---
CREATE TABLE IF NOT EXISTS servers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname      TEXT NOT NULL UNIQUE,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- --- index support ---
CREATE INDEX IF NOT EXISTS idx_replicas_project   ON replicas(project_id);
CREATE INDEX IF NOT EXISTS idx_replicas_state     ON replicas(state);
CREATE INDEX IF NOT EXISTS idx_domains_project    ON domains(project_id);
CREATE INDEX IF NOT EXISTS idx_domains_domain     ON domains(domain);
CREATE INDEX IF NOT EXISTS idx_dns_project        ON dns_records(project_id);
CREATE INDEX IF NOT EXISTS idx_build_logs_project ON build_logs(project_id, id);
CREATE INDEX IF NOT EXISTS idx_deployments_project ON deployments(project_id);
CREATE INDEX IF NOT EXISTS idx_metrics_vm_ts      ON metrics_samples(vm_id, metric, ts);
CREATE INDEX IF NOT EXISTS idx_groups_org         ON groups(org_id);
CREATE INDEX IF NOT EXISTS idx_gp_project         ON group_projects(project_id);

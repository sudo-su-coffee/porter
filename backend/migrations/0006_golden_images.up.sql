-- 0006_golden_images: reusable VM templates / image library for v0.4.
-- Images are referenced by OCI ref (URL or registry image), never local dirs.
CREATE TABLE IF NOT EXISTS golden_images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    image       TEXT NOT NULL,          -- OCI ref, e.g. redis:7-alpine
    description TEXT NOT NULL DEFAULT '',
    vcpus       INT NOT NULL DEFAULT 1,
    mem_mib     INT NOT NULL DEFAULT 256,
    ports       JSONB NOT NULL DEFAULT '[]',
    env         JSONB NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',
    logo        TEXT NOT NULL DEFAULT '', -- image URL for the dashboard tile
    version     TEXT NOT NULL DEFAULT 'latest',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_golden_images_name ON golden_images(name);

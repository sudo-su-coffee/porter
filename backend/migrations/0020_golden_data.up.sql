-- 0020_golden_data.up.sql
-- store.PutGoldenImage persists the full manifest as a JSONB `data` blob; the
-- original golden_images table lacked that column. Add it additively.
ALTER TABLE golden_images ADD COLUMN IF NOT EXISTS data JSONB NOT NULL DEFAULT '{}';

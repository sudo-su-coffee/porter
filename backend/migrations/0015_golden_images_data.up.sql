-- 0015: ListGoldenImages reads the row's `data` JSON blob (the store's
-- convention), but the golden_images table never got a data column.
ALTER TABLE golden_images ADD COLUMN IF NOT EXISTS data JSONB;

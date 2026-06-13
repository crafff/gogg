ALTER TABLE matches ADD COLUMN IF NOT EXISTS timeline_retry_count int NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS items_retry_count    int NOT NULL DEFAULT 0;

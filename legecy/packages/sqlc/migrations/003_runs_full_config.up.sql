ALTER TABLE runs ADD COLUMN IF NOT EXISTS version            text;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS rank_prefetch_tiers text[]                    NOT NULL DEFAULT '{}';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS queue               text                      NOT NULL DEFAULT 'RANKED_SOLO_5x5';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS execution           text                      NOT NULL DEFAULT 'pipeline';

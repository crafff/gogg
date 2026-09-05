-- Mark boots in completed items for fast filtering without JOIN.
ALTER TABLE match_completed_items ADD COLUMN IF NOT EXISTS is_boots boolean NOT NULL DEFAULT false;

-- Starter items: non-consumable, non-trinket items bought before laning phase.
CREATE TABLE IF NOT EXISTS match_starter_items (
    match_id       text     NOT NULL REFERENCES matches(match_id),
    participant_id smallint NOT NULL,
    item_id        int      NOT NULL,
    timestamp_ms   int      NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_starter_items_match ON match_starter_items(match_id, participant_id);

-- Extend item_catalog to track skippable items (consumable/trinket/vision).
ALTER TABLE item_catalog ADD COLUMN IF NOT EXISTS is_skippable boolean NOT NULL DEFAULT false;

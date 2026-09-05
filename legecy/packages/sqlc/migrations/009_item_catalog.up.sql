-- Item catalog per CDragon patch (e.g. "15.1").
-- Populated once per patch by phase55.
CREATE TABLE IF NOT EXISTS item_catalog (
    item_id      int  NOT NULL,
    patch        text NOT NULL,   -- e.g. "15.1"
    name         text,
    price_total  int,
    is_completed boolean NOT NULL DEFAULT false,  -- terminal item with recipe
    is_boots     boolean NOT NULL DEFAULT false,
    from_ids     int[],
    to_ids       int[],
    PRIMARY KEY (item_id, patch)
);

-- Completed items (大件) per participant in buy order.
-- slot 1 = first completed item, 2 = second, ... up to 6.
CREATE TABLE IF NOT EXISTS match_completed_items (
    match_id       text     NOT NULL REFERENCES matches(match_id),
    participant_id smallint NOT NULL,
    slot           smallint NOT NULL,
    item_id        int      NOT NULL,
    timestamp_ms   int      NOT NULL,
    PRIMARY KEY (match_id, participant_id, slot)
);
CREATE INDEX IF NOT EXISTS idx_completed_items_match ON match_completed_items(match_id);

-- Track item classification status on matches.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS items_status text NOT NULL DEFAULT 'pending';

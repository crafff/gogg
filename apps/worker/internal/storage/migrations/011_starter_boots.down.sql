ALTER TABLE item_catalog DROP COLUMN IF EXISTS is_skippable;
DROP TABLE IF EXISTS match_starter_items;
ALTER TABLE match_completed_items DROP COLUMN IF EXISTS is_boots;

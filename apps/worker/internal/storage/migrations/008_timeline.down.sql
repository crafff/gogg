DROP TABLE IF EXISTS match_participant_snapshots;
DROP TABLE IF EXISTS match_skill_events;
DROP TABLE IF EXISTS match_item_events;
DROP INDEX IF EXISTS idx_matches_timeline;
ALTER TABLE matches DROP COLUMN IF EXISTS timeline_status;
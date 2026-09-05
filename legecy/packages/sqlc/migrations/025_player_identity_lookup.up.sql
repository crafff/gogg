-- Imported collector bundles already contain Riot IDs in players even when
-- their optional on-demand summoner profile has never been refreshed. Keep
-- those historical identities directly searchable at production data volume.
CREATE INDEX IF NOT EXISTS players_identity_lookup_idx
    ON players (region, lower(game_name), upper(tag_line), updated_at DESC)
    WHERE game_name IS NOT NULL AND tag_line IS NOT NULL;

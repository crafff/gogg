ALTER TABLE tft_match_discoveries
    DROP CONSTRAINT tft_match_discoveries_pkey;

DELETE FROM tft_match_discoveries older
USING tft_match_discoveries preferred
WHERE older.run_id = preferred.run_id
  AND older.seed_puuid = preferred.seed_puuid
  AND older.match_id = preferred.match_id
  AND older.cohort > preferred.cohort;

ALTER TABLE tft_match_discoveries
    ADD PRIMARY KEY (run_id, seed_puuid, match_id);

DROP TABLE IF EXISTS tft_run_player_match_sync;
DROP TABLE IF EXISTS tft_run_platform_sampling;
DROP TABLE IF EXISTS tft_run_match_sampling;

DROP TABLE IF EXISTS tft_lineup_family_members;
DROP TABLE IF EXISTS tft_lineup_families;
DROP TABLE IF EXISTS tft_lineup_publications;
DROP TABLE IF EXISTS tft_lineup_exact_rollups;
ALTER TABLE tft_matches DROP CONSTRAINT IF EXISTS tft_matches_static_revision_fk;
DROP TABLE IF EXISTS tft_static_asset_jobs;
DROP TABLE IF EXISTS tft_static_assets;
DROP TABLE IF EXISTS tft_static_objects;
DROP TABLE IF EXISTS tft_static_snapshots;

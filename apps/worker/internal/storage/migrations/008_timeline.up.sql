-- Track timeline fetch status on each match.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS timeline_status text NOT NULL DEFAULT 'pending';
CREATE INDEX IF NOT EXISTS idx_matches_timeline
    ON matches(region, version, timeline_status) WHERE fetch_status = 'done';

-- Item purchase sequence per participant (used for build-order analysis).
-- No primary key: the same item can be bought multiple times.
CREATE TABLE IF NOT EXISTS match_item_events (
    match_id       text     NOT NULL REFERENCES matches(match_id),
    participant_id smallint NOT NULL,
    timestamp_ms   int      NOT NULL,
    item_id        int      NOT NULL,
    removal_type   text                         -- null=active | 'undo'=refunded | 'sold'=sold
);
CREATE INDEX IF NOT EXISTS idx_item_events_match ON match_item_events(match_id, participant_id);

-- Skill level-up sequence per participant.
CREATE TABLE IF NOT EXISTS match_skill_events (
    match_id       text     NOT NULL REFERENCES matches(match_id),
    participant_id smallint NOT NULL,
    timestamp_ms   int      NOT NULL,
    skill_slot     smallint NOT NULL,   -- 1=Q 2=W 3=E 4=R
    level_up_type  text                 -- 'NORMAL' | 'EVOLVE'
);
CREATE INDEX IF NOT EXISTS idx_skill_events_match ON match_skill_events(match_id, participant_id);

-- Per-participant stats at key minute snapshots (5/10/15/20/25/30 min).
CREATE TABLE IF NOT EXISTS match_participant_snapshots (
    match_id       text     NOT NULL REFERENCES matches(match_id),
    participant_id smallint NOT NULL,
    minute         smallint NOT NULL,
    total_gold     int,
    current_gold   int,
    cs             int,
    jungle_cs      int,
    level          smallint,
    xp             int,
    pos_x          int,
    pos_y          int,
    time_enemy_cc  int,
    wards_placed      smallint,
    wards_killed      smallint,
    -- cumulative damage at this minute (from damageStats)
    dmg_total         int,
    dmg_to_champs     int,
    dmg_magic_champs  int,
    dmg_phys_champs   int,
    dmg_true_champs   int,
    dmg_taken         int,
    PRIMARY KEY (match_id, participant_id, minute)
);
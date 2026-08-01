CREATE TABLE champion_detail_rollup (
    queue_id       int      NOT NULL CHECK (queue_id = 420),
    version        text     NOT NULL CHECK (btrim(version) <> ''),
    region         text     NOT NULL CHECK (btrim(region) <> ''),
    tier_bucket    text     NOT NULL,
    champion_id    int      NOT NULL CHECK (champion_id > 0),
    champion_name  text     NOT NULL CHECK (btrim(champion_name) <> ''),
    team_position  text     NOT NULL CHECK (team_position IN ('TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY')),
    category       text     NOT NULL CHECK (category IN ('RUNES','SPELLS','STARTER','BOOTS','ITEMS')),
    stage          smallint NOT NULL DEFAULT 0 CHECK (
        (category = 'ITEMS' AND stage BETWEEN 3 AND 6) OR
        (category <> 'ITEMS' AND stage = 0)
    ),
    signature      int[]    NOT NULL CHECK (cardinality(signature) > 0),
    games          bigint   NOT NULL CHECK (games > 0),
    wins           bigint   NOT NULL CHECK (wins >= 0 AND wins <= games),
    PRIMARY KEY (queue_id, version, region, tier_bucket, champion_id, team_position, category, stage, signature)
);

CREATE INDEX champion_detail_rollup_lookup
    ON champion_detail_rollup (champion_id, queue_id, version, region, tier_bucket, team_position, category, stage);

CREATE TABLE champion_detail_rollup_state (
    singleton        boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    refreshed_at     timestamptz NOT NULL,
    source_matches   bigint NOT NULL CHECK (source_matches >= 0),
    aggregate_rows   bigint NOT NULL CHECK (aggregate_rows >= 0)
);

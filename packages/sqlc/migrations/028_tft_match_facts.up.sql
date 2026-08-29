CREATE TABLE tft_matches (
    match_id            text PRIMARY KEY,
    data_version        text,
    platform            text NOT NULL,
    routing_region      text NOT NULL,
    queue_id            int NOT NULL,
    game_version        text NOT NULL,
    patch               text NOT NULL,
    game_datetime       timestamptz NOT NULL,
    game_length_seconds double precision,
    map_id              int,
    tft_game_type       text,
    set_core_name       text,
    set_number          int,
    end_of_game_result  text,
    participant_count   smallint NOT NULL,
    eligible            boolean NOT NULL DEFAULT false,
    exclusion_reason    text,
    static_revision_id  bigint,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX tft_matches_analysis_slice
    ON tft_matches (platform, patch, set_number, queue_id, game_datetime DESC)
    WHERE eligible;

CREATE TABLE tft_match_participants (
    match_id                  text NOT NULL REFERENCES tft_matches(match_id) ON DELETE CASCADE,
    puuid                     text NOT NULL,
    placement                 smallint NOT NULL,
    level                     smallint,
    gold_left                 int,
    last_round                int,
    players_eliminated        int,
    time_eliminated_seconds   double precision,
    total_damage_to_players   int,
    companion_content_id      text,
    companion_item_id         int,
    companion_skin_id         int,
    companion_species         text,
    mapped_unit_count         smallint NOT NULL DEFAULT 0,
    lineup_signature          text,
    abnormal                  boolean NOT NULL DEFAULT false,
    created_at                timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (match_id, puuid)
);

CREATE INDEX tft_match_participants_history
    ON tft_match_participants (puuid, match_id);

CREATE TABLE tft_match_augments (
    match_id      text NOT NULL,
    puuid         text NOT NULL,
    slot          smallint NOT NULL,
    augment_id    text NOT NULL,
    PRIMARY KEY (match_id, puuid, slot),
    FOREIGN KEY (match_id, puuid) REFERENCES tft_match_participants(match_id, puuid) ON DELETE CASCADE
);

CREATE TABLE tft_match_traits (
    match_id       text NOT NULL,
    puuid          text NOT NULL,
    trait_index    smallint NOT NULL,
    trait_id       text NOT NULL,
    num_units      smallint,
    style          smallint,
    tier_current   smallint,
    tier_total     smallint,
    PRIMARY KEY (match_id, puuid, trait_index),
    FOREIGN KEY (match_id, puuid) REFERENCES tft_match_participants(match_id, puuid) ON DELETE CASCADE
);

CREATE TABLE tft_match_units (
    match_id       text NOT NULL,
    puuid          text NOT NULL,
    unit_index     smallint NOT NULL,
    character_id   text NOT NULL,
    name           text,
    rarity         smallint,
    tier           smallint,
    mapped         boolean NOT NULL DEFAULT false,
    PRIMARY KEY (match_id, puuid, unit_index),
    FOREIGN KEY (match_id, puuid) REFERENCES tft_match_participants(match_id, puuid) ON DELETE CASCADE
);

CREATE TABLE tft_match_unit_items (
    match_id       text NOT NULL,
    puuid          text NOT NULL,
    unit_index     smallint NOT NULL,
    item_slot      smallint NOT NULL,
    item_id        text NOT NULL,
    PRIMARY KEY (match_id, puuid, unit_index, item_slot),
    FOREIGN KEY (match_id, puuid, unit_index) REFERENCES tft_match_units(match_id, puuid, unit_index) ON DELETE CASCADE
);

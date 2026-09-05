CREATE TABLE tft_static_team_planner_units (
    snapshot_id   bigint NOT NULL REFERENCES tft_static_snapshots(id) ON DELETE CASCADE,
    set_id        text NOT NULL,
    character_id  text NOT NULL,
    planner_code  integer NOT NULL CHECK (planner_code BETWEEN 1 AND 4095),
    cost          integer CHECK (cost > 0),
    payload       jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (snapshot_id, set_id, character_id),
    UNIQUE (snapshot_id, set_id, planner_code)
);

CREATE INDEX tft_static_team_planner_units_character
    ON tft_static_team_planner_units (snapshot_id, character_id, set_id);

CREATE TABLE IF NOT EXISTS match_boots (
    match_id       text     NOT NULL REFERENCES matches(match_id),
    participant_id smallint NOT NULL,
    item_id        int      NOT NULL,
    timestamp_ms   int      NOT NULL,
    PRIMARY KEY (match_id, participant_id)
);

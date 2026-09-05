-- Design draft only. Promote to packages/sqlc/migrations after online go/no-go.
CREATE TABLE player_champion_recommendations (
    puuid text NOT NULL,
    position text NOT NULL,
    champion_id int NOT NULL,
    rank smallint NOT NULL CHECK (rank BETWEEN 1 AND 20),
    slot_type text NOT NULL,
    final_score double precision NOT NULL,
    fit_confidence double precision NOT NULL CHECK (fit_confidence BETWEEN 0 AND 1),
    patch_score double precision,
    reason_codes text[] NOT NULL,
    model_version text NOT NULL,
    feature_as_of timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (puuid, position, model_version, rank),
    UNIQUE (puuid, position, model_version, champion_id)
);

CREATE INDEX idx_champion_recommendations_active
    ON player_champion_recommendations (puuid, position, expires_at DESC);

CREATE TABLE recommendation_requests (
    request_id uuid PRIMARY KEY,
    puuid text NOT NULL,
    position text NOT NULL,
    model_version text NOT NULL,
    context jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE recommendation_impressions (
    request_id uuid NOT NULL REFERENCES recommendation_requests(request_id),
    champion_id int NOT NULL,
    rank smallint NOT NULL,
    slot_type text NOT NULL,
    score double precision NOT NULL,
    propensity double precision,
    reason_codes text[] NOT NULL,
    PRIMARY KEY (request_id, champion_id)
);

CREATE TABLE recommendation_actions (
    request_id uuid NOT NULL REFERENCES recommendation_requests(request_id),
    champion_id int NOT NULL,
    action_type text NOT NULL CHECK (action_type IN ('view', 'save', 'dismiss', 'queue_intent', 'played', 'played_again')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (request_id, champion_id, action_type, created_at)
);

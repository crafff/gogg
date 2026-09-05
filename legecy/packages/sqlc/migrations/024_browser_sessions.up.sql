-- Browser login sessions and one-shot OAuth attempts.
--
-- The SPA never receives an OAuth, access, or refresh token. It only holds an
-- HttpOnly cookie containing an opaque session secret; PostgreSQL stores the
-- SHA-256 hash so a database leak cannot be replayed as a browser session.

CREATE TABLE user_sessions (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,
    user_agent   text,
    ip           inet,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX user_sessions_user_id_idx ON user_sessions (user_id);
CREATE INDEX user_sessions_expires_at_idx ON user_sessions (expires_at);

-- OAuth attempts bind state, provider, PKCE, browser, and the eventual local
-- redirect. UPDATE ... RETURNING consumes a row exactly once during callback.
CREATE TABLE oauth_login_attempts (
    state_hash           bytea       PRIMARY KEY,
    provider             text        NOT NULL,
    code_verifier        text        NOT NULL,
    return_to            text        NOT NULL,
    browser_binding_hash bytea       NOT NULL,
    expires_at           timestamptz NOT NULL,
    consumed_at          timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX oauth_login_attempts_expires_at_idx
    ON oauth_login_attempts (expires_at);

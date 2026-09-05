-- Users + OAuth identities + refresh tokens.
--
-- The user system is intentionally minimal in V1: an account is just
-- an internal id + a display label, with one or more OAuth identities
-- bound to it. Profile data (favourites, settings) lands in later
-- migrations once the resolver surface for those exists.
--
-- Schema choices worth noting:
--   * users.id is uuid v7 (sortable, time-ordered) rather than serial
--     so user-scoped paths can be made shardable later without
--     renumbering.
--   * (provider, provider_user_id) on user_oauth_identities is the
--     external identity key — a Discord account can bind to exactly
--     one gogg user.
--   * user_refresh_tokens stores only sha-256 hashes of the opaque
--     token; the cleartext lives only in the cookie / Authorization
--     header. Compromise of the DB does not let the attacker mint
--     access tokens.

CREATE TABLE IF NOT EXISTS users (
    id            uuid        PRIMARY KEY,
    display_name  text        NOT NULL,
    email         text,
    locale        text        NOT NULL DEFAULT 'zh-CN',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz
);

CREATE TABLE IF NOT EXISTS user_oauth_identities (
    id                uuid        PRIMARY KEY,
    user_id           uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          text        NOT NULL,
    provider_user_id  text        NOT NULL,
    provider_email    text,
    provider_username text,
    avatar_url        text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS user_oauth_identities_user_id_idx
    ON user_oauth_identities (user_id);

CREATE TABLE IF NOT EXISTS user_refresh_tokens (
    id          uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    user_agent  text,
    ip          inet,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS user_refresh_tokens_user_id_idx
    ON user_refresh_tokens (user_id);

CREATE INDEX IF NOT EXISTS user_refresh_tokens_expires_at_idx
    ON user_refresh_tokens (expires_at);

-- Queries on users + user_oauth_identities + user_refresh_tokens.
-- These power the OAuth callback flow, token refresh, and the
-- /me resolver that lands in Phase E.

-- name: CreateUser :one
INSERT INTO users (id, display_name, email, locale)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByOAuthIdentity :one
-- Find the gogg user bound to an external identity. Used on every
-- OAuth callback: if the (provider, provider_user_id) tuple is known,
-- log the existing user in; otherwise fall through to CreateUser +
-- LinkOAuthIdentity.
SELECT u.*
FROM users u
JOIN user_oauth_identities i ON i.user_id = u.id
WHERE i.provider = $1 AND i.provider_user_id = $2;

-- name: TouchUserLastLogin :exec
UPDATE users
SET last_login_at = now(), updated_at = now()
WHERE id = $1;

-- name: UpsertOAuthIdentity :one
-- Bind an external identity to a user. Re-runs of the OAuth flow
-- refresh the cached label fields (email, username, avatar) without
-- changing the user_id binding. ON CONFLICT key matches the migration's
-- UNIQUE (provider, provider_user_id).
INSERT INTO user_oauth_identities (
    id, user_id, provider, provider_user_id,
    provider_email, provider_username, avatar_url
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (provider, provider_user_id) DO UPDATE
SET provider_email    = EXCLUDED.provider_email,
    provider_username = EXCLUDED.provider_username,
    avatar_url        = EXCLUDED.avatar_url,
    updated_at        = now()
RETURNING *;

-- name: CreateRefreshToken :one
-- token_hash is the sha-256 of the opaque refresh token; the cleartext
-- never lands in the DB. expires_at is computed by the auth package
-- (created_at + refresh ttl) so this query stays generic.
INSERT INTO user_refresh_tokens (
    id, user_id, token_hash, expires_at, user_agent, ip
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRefreshTokenByHash :one
-- Look up a refresh token by its hash. Caller is responsible for
-- checking revoked_at and expires_at; we keep both so audit queries
-- can see rotation history.
SELECT * FROM user_refresh_tokens WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE user_refresh_tokens
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllRefreshTokensForUser :exec
-- Hard logout: invalidate every active refresh token for the user.
-- Called from /auth/logout?everywhere=1 and from /me on password /
-- profile-change paths once those land.
UPDATE user_refresh_tokens
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :execrows
-- House-keeping. Run from a cron Activity once Phase C wires Temporal;
-- for now the manual `gogg admin gc-refresh-tokens` (not built yet)
-- can call it. Returns rowcount so the caller can log it.
DELETE FROM user_refresh_tokens
WHERE expires_at < now();

-- name: ListUserOAuthIdentities :many
SELECT *
FROM user_oauth_identities
WHERE user_id = $1
ORDER BY provider;

-- name: AcquireOAuthIdentityLock :exec
-- Serialize first-login transactions for the same provider subject without
-- holding a database transaction open during the external OAuth exchange.
SELECT pg_advisory_xact_lock(hashtextextended(
    sqlc.arg(provider)::text || ':' || sqlc.arg(provider_user_id)::text,
    0
));

-- name: CreateOAuthLoginAttempt :one
INSERT INTO oauth_login_attempts (
    state_hash, provider, code_verifier, return_to,
    browser_binding_hash, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ConsumeOAuthLoginAttempt :one
UPDATE oauth_login_attempts
SET consumed_at = now()
WHERE state_hash = $1
  AND provider = $2
  AND browser_binding_hash = $3
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING *;

-- name: CreateUserSession :one
INSERT INTO user_sessions (
    id, user_id, token_hash, expires_at, user_agent, ip
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetActiveUserSessionByHash :one
SELECT *
FROM user_sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now();

-- name: RevokeUserSessionByHash :execrows
UPDATE user_sessions
SET revoked_at = now()
WHERE token_hash = $1
  AND revoked_at IS NULL;

-- name: DeleteExpiredOAuthLoginAttempts :execrows
DELETE FROM oauth_login_attempts
WHERE expires_at < now();

-- name: DeleteExpiredUserSessions :execrows
DELETE FROM user_sessions
WHERE expires_at < now();

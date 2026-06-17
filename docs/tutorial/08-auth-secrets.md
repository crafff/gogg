# Chapter 08 · Auth + secrets

> Goal: by the end of this chapter you can explain the OAuth flow that lights up the "Sign in with Discord" link, you understand JWT access vs refresh tokens, and you can decrypt + edit + re-encrypt the SOPS secrets file without leaking anything to git.

## Part A — Auth

### Why two tokens?

When a user signs in, they expect to stay signed in for weeks without re-authenticating. But every request shouldn't carry a 30-day session token over the wire — if it's stolen, the attacker has long-term access.

The standard trade is:

- **Access token** — short-lived (15 min). Carried in every request as `Authorization: Bearer <token>`. If stolen, expires fast.
- **Refresh token** — long-lived (30 days). Stored in an HttpOnly cookie or sent only over `/auth/refresh`. Used to mint new access tokens when the current one expires. Revocable.

Both are JSON Web Tokens (JWT) — Base64-encoded triples of `header.payload.signature`. The signature is HS256 (HMAC-SHA-256) with a server-side secret. Anyone can read the payload (it's not encrypted), but only the server can produce a valid signature.

### Read the issuer

```bash
cat apps/api/internal/auth/jwt.go | head -80
```

`Issuer` has two main methods:

- `IssuePair(userID)` → returns `{Access, Refresh}` JWT strings.
- `Verify(token)` → parses + validates a token, returns the claims or an error.

Inside `IssuePair`:

1. Generate a random `jti` (JWT ID) for the refresh token. This is the revocation handle.
2. Build the access token: claims `{sub, iat, exp (now + 15m), iss}`. Sign with HS256.
3. Build the refresh token: claims `{sub, jti, iat, exp (now + 30d), iss}`. Sign.
4. Persist the refresh `jti` into `user_refresh_tokens` so we can revoke it later.

The signing secret is loaded from config (originally `deploy/secrets/dev.enc.yaml`'s `jwt_secret` field) and must be at least 32 bytes. The test secret in `jwt_test.go` is `"not-a-real-secret-for-tests-only!"` — exactly 32 ASCII bytes, low-entropy, with `//gitleaks:allow` so the secret scanner stops complaining.

### The OAuth flow

We have three OAuth providers planned:

- **Discord** (live in Phase B chunk 6)
- **Google** (live in Phase B chunk 6)
- **Riot RSO** (build-tag-gated, gated on Riot approval; expected in Phase E)

All three implement the same `Provider` interface. Look:

```bash
ls apps/api/internal/auth/provider/
cat apps/api/internal/auth/provider/discord.go | head -60
```

The interface:

```go
type Provider interface {
    Name() string
    AuthorizeURL(state string) string
    Exchange(ctx context.Context, code string) (*Profile, error)
}
```

Three methods. `AuthorizeURL` builds the provider's `/authorize` URL. `Exchange` swaps the OAuth `code` for a profile (using the provider's token + user-info endpoints).

### The HTTP surface

```bash
cat apps/api/internal/transport/rest/auth/auth.go | head -80
```

Four routes:

```
GET  /oauth/start/{provider}     → 302 to provider's authorize URL, sets state cookie
GET  /oauth/callback/{provider}  → exchanges code, upserts user + identity, issues JWT pair
POST /auth/refresh               → rotates refresh, issues new access
POST /auth/logout                → revokes refresh (deletes row)
```

### Trace one sign-in, end-to-end

User clicks "Sign in with Discord" in the UI (currently the placeholder LoginPage links to `/oauth/start/discord`):

1. Browser GETs `/oauth/start/discord`.
2. Handler generates a random `state`, sets it as an HttpOnly cookie (`oauth_state`), redirects to `https://discord.com/api/oauth2/authorize?client_id=...&redirect_uri=...&state=<state>&scope=identify+email`.
3. User authorizes on Discord's page.
4. Discord redirects back: `GET /oauth/callback/discord?code=ABC&state=<state>`.
5. Handler verifies the `state` cookie matches, then calls `provider.Exchange(ctx, code)`:
   - POST `https://discord.com/api/oauth2/token` with `code` → `{access_token, refresh_token, expires_in}`.
   - GET `https://discord.com/api/users/@me` with that token → `{id, username, email}`.
   - Returns a `Profile{ID: "discord-user-id", Email: "...", Name: "..."}`.
6. Handler looks up `users` by the OAuth identity:
   ```sql
   SELECT u.* FROM users u
   JOIN user_oauth_identities i ON i.user_id = u.id
   WHERE i.provider = 'discord' AND i.subject = 'discord-user-id';
   ```
7. If found, that's the user. If not, INSERT a new `users` row + `user_oauth_identities` row.
8. Call `Issuer.IssuePair(user.ID)` → access + refresh tokens.
9. Set the refresh as an HttpOnly cookie, return JSON `{accessToken: "..."}` to the browser.
10. Frontend stores the access token (in memory or sessionStorage, NOT localStorage) and uses it in `Authorization` headers.

### Refresh flow

Access token expires after 15 min. The frontend sees a 401 from `/graphql` or `/api/v1/...`, posts to `/auth/refresh` (the browser auto-sends the refresh cookie), gets a new access token, retries the original request.

If the refresh token has been revoked (the `jti` is gone from `user_refresh_tokens`), `/auth/refresh` returns 401 and the frontend bounces the user to `/login`.

### Logout

`POST /auth/logout` reads the refresh cookie, parses out the `jti`, deletes the matching row from `user_refresh_tokens`, clears the cookie. All future `/auth/refresh` calls with that token fail.

### Try this

🛠️ **Exercise**: hit `/oauth/start/discord` in your browser. You'll get a 302 to Discord with a `state` query param. Without a registered Discord app, the actual sign-in won't work, but you can:

1. Inspect the cookie set in the 302 response — it's named `oauth_state` and matches the URL's `state` param.
2. Visit `/oauth/callback/discord?code=fake&state=<the-cookie-value>` manually — the handler should fail with "discord exchange failed" because there's no real OAuth code, but it proves the route is wired.

🛠️ **Exercise**: with no real auth set up, you can still play with JWT manually:

```bash
# Decode a JWT (no verification — just base64 decode)
echo 'eyJhbGc...' | cut -d. -f2 | base64 -d
```

That shows you the claims. Anyone can read them; they're not encrypted. Only the signature step needs the secret.

---

## Part B — Secrets via SOPS

### Why SOPS

The legacy stack had a `config.yaml` with the Riot API key in plaintext. That key was reachable via `git log` even after rotation. SOPS fixes this:

- **Files are encrypted at rest**, including in git.
- **The encryption key is per-recipient** (via age public keys). Adding a teammate is updating `.sops.yaml` + re-encrypting.
- **Decryption is automatic in dev tools** — `make run-api` calls `sops -d ...` transparently.
- **CI's gitleaks scan can't trip on encrypted blobs** — they look like base64, no recognizable patterns.

### The pieces

```bash
cat .sops.yaml
```

```yaml
creation_rules:
  - path_regex: deploy/secrets/.*\.enc\.yaml$
    age:
      - <crafff's age public key>
```

That tells SOPS: "files matching `deploy/secrets/*.enc.yaml` should be encrypted to these age recipients."

```bash
cat deploy/secrets/dev.enc.yaml | head -30
```

What you see is the encrypted form:

```yaml
riot_api_key: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
jwt_secret: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
...
sops:
    age:
        - recipient: age1...
          enc: |
              -----BEGIN AGE ENCRYPTED FILE-----
              ...
              -----END AGE ENCRYPTED FILE-----
```

Each value is independently encrypted with a per-file AES256-GCM key. That key is itself encrypted to each recipient's age public key in the `sops.age` block. Decryption: your private age key decrypts the AES key, the AES key decrypts the values.

### Decrypt + read

```bash
sops -d deploy/secrets/dev.enc.yaml | head -20
```

You should see plain YAML:

```yaml
riot_api_key: RGAPI-xxxx-xxxx-xxxx
jwt_secret: not-a-real-secret-for-tests-only!
postgres_dsn: postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable
...
```

If `sops` fails with "no key found that can decrypt", your local age private key isn't in the recipient list. Workflows:

- On a fresh machine, generate an age key: `age-keygen -o ~/.config/sops/age/keys.txt`.
- Add the public key (`age1...`) to `.sops.yaml`.
- An existing recipient re-encrypts: `sops -r -i deploy/secrets/dev.enc.yaml` (rewrite-in-place after `.sops.yaml` changes).
- Commit the updated `dev.enc.yaml`.

### Edit a secret

```bash
sops deploy/secrets/dev.enc.yaml
```

That opens an editor on the *decrypted* content. Edit, save, quit. SOPS re-encrypts to the recipient list. The file on disk is encrypted again — `cat` shows the ciphertext.

🛠️ **Exercise**: open `deploy/secrets/dev.enc.yaml` with `sops`. Add a new dummy key:

```yaml
test_value: hello-from-the-tutorial
```

Save, quit. Run `git diff deploy/secrets/dev.enc.yaml` — you'll see the ciphertext changed. Run `sops -d deploy/secrets/dev.enc.yaml | grep test_value` to confirm the plaintext is there. Remove via another `sops` edit when done.

### How the binaries use it

```bash
grep -A 10 'sops --decrypt' Makefile
```

`make run-api` and `make run-worker` do roughly:

```bash
tmp=$(mktemp -t gogg-api.XXXXXX.yaml)
trap "rm -f $tmp" EXIT
sops --decrypt deploy/secrets/dev.enc.yaml > $tmp
APP_CONFIG_PATH=$tmp go run ./apps/api/cmd/api
```

The decrypted YAML lives in `/tmp` for the lifetime of the process; the `trap` cleans it up when the process exits.

### Rotating a leaked key

If you accidentally commit a plaintext secret (it has happened):

1. **Immediately rotate** the secret at its source (Riot dashboard, Discord developer portal, etc.). The committed value is now public.
2. Rewrite git history to remove the leaked commit: `git filter-repo --replace-text replacements.txt`.
3. Force-push (with team coordination).
4. Add a new entry to `.gitleaksignore` *only* if the file is a known false positive (e.g. a test fixture). Real leaks shouldn't be allowlisted — they should be rotated.

### CI integration

```bash
cat .github/workflows/security-scan.yml 2>/dev/null | head -30
```

The `gitleaks (secret scan)` job runs on every PR. It scans the diff for high-entropy strings, API key patterns, etc. The `.gitleaksignore` file holds fingerprints of *known* false positives — see the historical jwt_test entry, which was kept after a follow-up commit replaced it with a low-entropy phrase.

## Up next

You've finished **Part I** — you understand GOGG end-to-end.

**Part II** generalizes the patterns. If you want broader Go / React / how-to-read-codebases knowledge, continue with [Chapter 10 — Go essentials](./10-go-essentials.md). If you'd rather see what's next on the roadmap, jump to [Chapter 09 — Next steps](./09-next-steps.md).

# Chapter 08 · Auth + secrets

> Goal: by the end of this chapter you can explain GOGG's Google OAuth browser flow, its server-side session boundary, and how to decrypt + edit + re-encrypt the SOPS secrets file without leaking anything to git.

## Part A — Auth

### Browser session model

The React application never receives a Google access token, GOGG JWT, or refresh token. After Google authenticates the user, the API creates a random opaque session secret, stores only its SHA-256 digest in PostgreSQL, and sends the cleartext only as an HttpOnly cookie.

- The development cookie is `gogg_session`.
- HTTPS deployments use the `__Host-gogg_session` prefix.
- The cookie is host-only, `Path=/`, `HttpOnly`, and `SameSite=Strict`.
- The server resolves the session on each authenticated request; TanStack Query's nullable `Me` result is the frontend session state.
- `POST /auth/logout` revokes the server row before clearing the cookie.

The existing JWT issuer remains available for explicitly authenticated non-browser clients, but the web login flow does not issue or store browser JWTs.

### The OAuth flow

Google is the only provider registered in the first browser-login release. The provider abstraction still leaves room for Discord later; Riot RSO remains separate and requires Riot approval.

Look at the provider boundary:

```bash
ls apps/api/internal/auth/provider/
cat apps/api/internal/auth/provider/google.go | head -80
```

The interface:

```go
type Provider interface {
    Name() string
    AuthCodeURL(state, codeVerifier string) string
    Exchange(ctx context.Context, code, codeVerifier string) (UserInfo, error)
}
```

`AuthCodeURL` includes a one-shot state value and an S256 PKCE challenge. `Exchange` sends the matching verifier while swapping the authorization code, then reads Google's user-info endpoint. The OAuth access token stays inside that server-side request.

### The HTTP surface

```bash
cat apps/api/internal/transport/rest/auth/auth.go | head -80
```

Three routes plus two GraphQL queries:

```
GET  /oauth/start/google         → stores state + PKCE attempt, 302 to Google
GET  /oauth/callback/google      → consumes attempt, upserts identity, sets session cookie
POST /auth/logout                → revokes the current browser session
query Me                         → nullable current user for session restoration
query AuthProviders              → providers enabled in this deployment
```

### Trace one sign-in, end-to-end

User clicks “Continue with Google”:

1. The browser opens `/oauth/start/google?returnTo=/me`.
2. The API generates random state, PKCE verifier, and a browser-binding secret. It stores only state/binding hashes plus the verifier in `oauth_login_attempts`, then redirects to Google with the state and S256 challenge.
3. Google redirects to `/oauth/callback/google?code=...&state=...`.
4. The API atomically consumes the unexpired attempt using state, provider, and browser-binding hash. A replay returns an expired/invalid result.
5. The API exchanges the code with the PKCE verifier and fetches the stable Google subject plus verified basic profile.
6. A short database transaction serializes the provider identity, creates or updates the GOGG user, and creates `user_sessions`.
7. The callback sends the opaque HttpOnly session cookie and redirects only to a validated local path. No token is written into redirect parameters, HTML, React state, `sessionStorage`, or `localStorage`.

When Redis is configured, `/oauth/start/google` is limited per client IP. Each
start also removes expired attempts and sessions. The callback uses
`Referrer-Policy: no-referrer`, and production nginx does not access-log the
authorization-code query string.

### Logout

The web app sends `POST /auth/logout` with `X-GOGG-CSRF: 1`. Unsafe requests carrying the session cookie require this custom header. The API marks the matching session revoked and only then expires the browser cookie.

### Try this

1. Create a Google OAuth web client and register `http://localhost:5173/oauth/callback/google` for local development.
2. Configure `oauth.google.client_id`, `client_secret`, and `redirect_url` together. The API rejects partial provider configuration at startup.
3. Keep `cookie_secure: false` only for local HTTP. Production callbacks must use HTTPS and production cookies must set `cookie_secure: true`.
4. Run the migration and both services, then open `http://localhost:5173/login`.

---

## Part B — Secrets via SOPS

### Why SOPS

Plaintext local config files are useful for development but should
never be committed. SOPS gives us the committed equivalent for shared
environments:

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

# deploy/secrets

SOPS-encrypted secrets per environment. **You** (the human
operator) generate keys; nothing in this directory should ever
contain plaintext credentials that get committed.

## Why SOPS

- Encrypts only values, not keys → diffs are reviewable
- Works across cloud providers (no vendor lock-in to AWS Secrets
  Manager or Aliyun KMS)
- CI decrypts with a single env-var-provided private key; no
  network call to a secret store on cold start

## One-time setup

```bash
# 1. Install tools
brew install sops age          # macOS
# or
sudo apt install sops age-tools # debian/ubuntu

# 2. Generate an age key (or per-developer keys)
age-keygen -o ~/.config/sops/age/keys.txt

# 3. Note the public key from stdout (looks like age1xxx...) and
#    add it to .sops.yaml at the repo root.
```

## Working with a secrets file

```bash
# Create / edit the dev secrets file. SOPS auto-decrypts in the
# editor and re-encrypts on save.
sops deploy/secrets/dev.enc.yaml

# Print plaintext to stdout (e.g. to pipe into kubectl)
sops --decrypt deploy/secrets/dev.enc.yaml

# Re-encrypt after rotating recipients (.sops.yaml change)
sops updatekeys deploy/secrets/dev.enc.yaml
```

## File layout

```
dev.enc.yaml      — local-stack secrets (Riot key, JWT signing, OAuth client IDs)
staging.enc.yaml  — staging env (separate Riot key, separate JWT issuer)
prod.enc.yaml     — production
```

Each file follows the same schema as `config/dev.example.yaml`, with
real secret values encrypted by SOPS:

```yaml
temporal:
  host_port: localhost:7233
  namespace: default
  task_queues: [crawl-kr, crawl-na1]
logging:
  level: info
  format: json
riot:
  api_key: RGAPI-...
regions:
  - name: KR
    base_url: https://kr.api.riotgames.com
  - name: NA1
    base_url: https://na1.api.riotgames.com
database:
  dsn: postgres://user:pass@host:5432/gogg
redis:
  url: redis://host:6379/0
schedule:
  - cron: "0 4 * * *"
    profile: daily_kr
run_profiles:
  daily_kr:
    region: KR
    mode: incremental
    target_tiers: [CHALLENGER, GRANDMASTER, MASTER]
    rank_prefetch_tiers: [CHALLENGER, GRANDMASTER, MASTER, DIAMOND]
    queue: RANKED_SOLO_5x5
    execution: pipeline
jwt:
  access_private_key: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
  refresh_signing_secret: ...
oauth:
  discord:
    client_id:     ...
    client_secret: ...
  google:
    client_id:     ...
    client_secret: ...
  riot_rso:
    client_id:     ...
    client_secret: ...
```

## CI integration

The CI workflows load the **SOPS_AGE_KEY** repo secret (containing
the age private key) and call:

```bash
export SOPS_AGE_KEY="$(echo "$SOPS_AGE_KEY_B64" | base64 -d)"
sops --decrypt deploy/secrets/staging.enc.yaml > /tmp/staging.yaml
```

Each environment has its own age key recipient so leaking the
staging key does not expose prod.

## Bootstrap checklist

1. Generate an age key (`age-keygen`).
2. Add the public key to `.sops.yaml`.
3. Rotate or create the Riot API key at <https://developer.riotgames.com>.
4. Edit `deploy/secrets/dev.enc.yaml` with `sops` and paste in the key.
5. Update CI repo secrets: `SOPS_AGE_KEY` (dev/staging/prod).

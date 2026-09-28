# SECRETS

Canonical home for SOPS encryption/decryption, AGE key validation,
SOPS-driven template rendering, and Kubernetes Secret manifest generation.
Other blueprints modules depend on this one rather than implementing
SOPS workflows directly. Created in #143 to consolidate three previous
implementations across `configuration`, `vm`, and `kubernetes-deployment`.

```bash
# DECRYPT a SOPS-encrypted file with an AGE private key
dagger call -m secrets decrypt \
  --sops-key env:SOPS_AGE_KEY \
  --encrypted-file tests/vm/terraform.tfvars.enc.json \
  --progress plain
```

```bash
# ENCRYPT a plaintext file with an AGE public key
dagger call -m secrets encrypt-file \
  --age-public-key env:AGE_PUB \
  --plaintext-file ./secret.yaml \
  --file-extension yaml \
  --progress plain
```

```bash
# ENCRYPT an in-memory string with an AGE public key
dagger call -m secrets encrypt-string \
  --age-public-key env:AGE_PUB \
  --plaintext "$(cat secret.yaml)" \
  --file-extension yaml \
  --progress plain
```

```bash
# RENDER a Go template against decrypted SOPS data, then optionally re-encrypt
dagger call -m secrets render-template \
  --age-key env:SOPS_AGE_KEY \
  --encrypted-data-file tests/data.sops.json \
  --template-file ./secret.json.tmpl \
  --age-recipient env:AGE_PUB \
  --file-extension json \
  --encrypt=true \
  export --path ./rendered.enc.json
```

```bash
# CREATE a SOPS-encrypted Kubernetes Secret manifest from key=value pairs
dagger call -m secrets create-kubernetes-secret \
  --name my-secret --namespace default \
  --key-values "user=admin,password=s3cret" \
  --age-public-key env:AGE_PUB \
  export --path ./secret.enc.yaml

# String-returning variant (manifest as stdout)
dagger call -m secrets create-kubernetes-secret-string \
  --name my-secret --namespace default \
  --key-values "user=admin,password=s3cret" \
  --age-public-key env:AGE_PUB
```

```bash
# VALIDATE that an AGE private key matches a given AGE public key
dagger call -m secrets validate-age-key-pair \
  --sops-age-key env:SOPS_AGE_KEY \
  --age-public-key env:AGE_PUB \
  --progress plain
```

## CLUSTER SECRETS

`generate-cluster-secrets` renders every Secret a cluster needs from a YAML
profile and encrypts them with an AGE key **of that cluster's own**. Values are
generated, copied from a central SOPS store, or read from Vault. Vault is
contacted only when a profile references it, so an edge cluster without Vault
needs nothing but the master key.

```
ClusterSecrets + SecretProfiles
        │
        ▼
 cluster key ── existing output? decrypt sops-age.enc.yaml with the master key
        │                          : generate a new key (Go, never cached)
        ▼
 previous values ── decrypt existing secrets/ with the cluster key
        │
        ▼
 resolve ── generate:        crypto/rand, kept across runs unless --rotate
            value:           literal
            ref+sops://      central store, decrypted with the master key
            ref+vault://     KV v1/v2, token or AppRole (optional)
        │
        ▼
 encrypt ── sops-age.enc.yaml   → master + escrow   (never the cluster itself)
            secrets/**.enc.yaml → cluster + escrow
            unchanged files are returned byte-for-byte (empty diff on re-run)
```

### Profiles

```yaml
# profiles/secrets/platform.yaml – reusable
kind: SecretProfile
metadata: { name: platform }
spec:
  secrets:
    - name: harbor-secrets                 # namespace defaults to flux-system
      data:
        HARBOR_ADMIN_PASSWORD: { generate: { type: password, length: 24 } }
        HARBOR_SECRET_KEY:     { generate: { type: alnum, length: 16 } }
    - name: velero-secrets
      namespace: velero
      data:
        AWS_ACCESS_KEY_ID: ref+sops://secrets/minio.enc.yaml#/accessKey
        AWS_SECRET_ACCESS_KEY: ref+vault://secret/minio/labul#/secretKey
        BUCKET: velero                     # literal
```

```yaml
# clusters/<env>/<cluster>/cluster-secrets.yaml – one per cluster
kind: ClusterSecrets
metadata: { name: edge-01 }
spec:
  profiles: [platform]                     # applied in order
  secrets:                                 # cluster-only secrets / per-key overrides
    - name: harbor-secrets
      data:
        HARBOR_ADMIN_PASSWORD: { generate: { type: password, length: 32 } }
```

| Source | Syntax |
|---|---|
| random | `{ generate: { type: password\|alnum\|hex\|base64\|uuid, length: N, charset: "..." } }` |
| literal | `{ value: "..." }` or a bare scalar |
| SOPS | `ref+sops://<path below --sops-ref-dir>#/<json/pointer>` |
| Vault KV v2 | `ref+vault://<mount>/<path>#/<field>` |
| Vault KV v1 | `ref+vault://<mount>/<path>?kv=1#/<field>` |

Unknown fields fail the run, so a typo like `genrate:` cannot silently drop a key.

### Keys

| Key | Held by | Can decrypt |
|---|---|---|
| master (`--master-age-key`) | CI / key custodian, e.g. a GitHub environment secret per environment | `sops-age.enc.yaml` (→ the cluster key), the central `ref+sops` store |
| escrow (`--escrow-recipients`) | offline / hardware key | everything – break-glass and re-keying |
| cluster (generated) | the cluster (`flux-system/sops-age`) | its own `secrets/` only |

The cluster key lives in Git, encrypted for master + escrow, so no external
store is needed to rebuild a cluster. The cluster cannot read its own key
file nor the central store.

### Run

```bash
dagger call -m secrets generate-cluster-secrets \
  --cluster-profile clusters/labul/edge-01/cluster-secrets.yaml \
  --profile-dir profiles/secrets \
  --master-age-key file:$HOME/.config/sops/age/master.txt \
  --escrow-recipients age1... \
  --existing clusters/labul/edge-01/cluster-secrets \
  --sops-ref-dir . \
  --vault-addr https://vault.example.com:8200 --vault-token env:VAULT_TOKEN \
  --progress plain \
  export --path clusters/labul/edge-01/cluster-secrets --wipe
```

- Omit `--existing` on the very first run only; afterwards always pass the
  previous output, or every password and the cluster key are replaced.
- `--wipe` removes secrets that were dropped from the profile.
- `--rotate harbor-secrets:HARBOR_ADMIN_PASSWORD` (or `--rotate harbor-secrets`)
  regenerates specific values; a rotate entry matching nothing fails the run.
- Vault auth: `--vault-token`, or `--vault-role-id` + `--vault-secret-id`
  (AppRole). `--vault-ca-cert` for a private PKI.

Hand the cluster key to `flux bootstrap`:

```bash
dagger call -m secrets cluster-age-key \
  --existing clusters/labul/edge-01/cluster-secrets \
  --master-age-key file:$HOME/.config/sops/age/master.txt \
  plaintext
```

Point a Flux `Kustomization` with `decryption: { provider: sops, secretRef: { name: sops-age } }`
at `<output>/secrets`; `sops-age.enc.yaml` stays outside that path.

Decrypted intermediates (the previous run, `ref+sops` files, Vault
responses) pass through files in the engine cache, as with `decrypt`;
they never go to stdout, so `--progress plain` does not print them. Plaintext
for encryption is mounted as a Dagger secret and never written to a layer.

### Tests

```bash
# engine-free: profile/merge/generate/refs/render + real sops/age round trip
cd secrets && go test ./clustersecrets/

# same, plus the Vault fetch script against a local Vault (KV v1/v2, AppRole)
docker run -d --rm --name vault-test -p 8200:8200 \
  -e VAULT_DEV_ROOT_TOKEN_ID=root -e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
  -e SKIP_SETCAP=1 hashicorp/vault:1.20 server -dev
docker exec -i -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=root \
  vault-test sh < tests/vault-seed.sh
CLUSTERSECRETS_VAULT_ADDR=http://127.0.0.1:8200 go test ./clustersecrets/

# end to end through Dagger: `vault server -dev` as a service, seeded from a
# second container, reached both via its endpoint (--vault-addr + AppRole)
# and as a bound service (--vault-service + token)
dagger call -m secrets/tests all --progress plain
```

## Migrated from

| Old call | New call |
|---|---|
| `dagger call -m vm decrypt-sops` | `dagger call -m secrets decrypt` |
| `dagger call -m vm encrypt-file` | `dagger call -m secrets encrypt-file` |
| `dagger call -m configuration create-secrets-file` | `dagger call -m secrets render-template` |
| `dagger call -m kubernetes-deployment create-sops-secret` | `dagger call -m secrets create-kubernetes-secret` |
| `dagger call -m kubernetes-deployment create-sops-secret-string` | `dagger call -m secrets create-kubernetes-secret-string` |
| `dagger call -m flux validate-age-key-pair` | `dagger call -m secrets validate-age-key-pair` |
| `dagger call -m flux flux-encrypt-secrets --secret-content $X` | `dagger call -m secrets encrypt-string --plaintext $X` |

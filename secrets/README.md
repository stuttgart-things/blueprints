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

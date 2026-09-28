#!/bin/sh
# Starts a Vault dev server and seeds the fixtures the cluster-secrets tests
# read. Used by the Dagger test module (Vault as a service) and for running
# the Go integration tests against a local container:
#
#   docker run -d --name vault-test -p 8200:8200 -e SKIP_SETCAP=1 \
#     -v "$PWD/secrets/tests/vault-seed.sh:/seed.sh:ro" \
#     --entrypoint sh hashicorp/vault:1.20 /seed.sh
#   CLUSTERSECRETS_VAULT_ADDR=http://127.0.0.1:8200 go test ./secrets/clustersecrets/
#
# The last write is secret/ready; poll for it before using the server.
set -eu

vault server -dev -dev-root-token-id=root -dev-listen-address=0.0.0.0:8200 &
server=$!

export VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root
until vault status >/dev/null 2>&1; do sleep 0.2; done

# KV v2 at secret/ (dev default)
vault kv put secret/edge/teams webhook=https://example.invalid/hook >/dev/null
vault kv put secret/edge/minio secretKey=minio-from-vault >/dev/null

# KV v1
vault secrets enable -path=kv1 -version=1 kv >/dev/null
vault kv put kv1/legacy token=kv1-token >/dev/null

# AppRole with fixed credentials, read-only on both mounts
printf '%s\n' \
  'path "secret/data/*" { capabilities = ["read"] }' \
  'path "kv1/*" { capabilities = ["read"] }' | vault policy write cluster-secrets-read - >/dev/null
vault auth enable approle >/dev/null
vault write auth/approle/role/cluster-secrets token_policies=cluster-secrets-read >/dev/null
vault write auth/approle/role/cluster-secrets/role-id role_id=test-role >/dev/null
vault write auth/approle/role/cluster-secrets/custom-secret-id secret_id=test-secret >/dev/null

vault kv put secret/ready ok=true >/dev/null
echo "vault seeded"
wait "$server"

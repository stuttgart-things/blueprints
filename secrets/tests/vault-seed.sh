#!/bin/sh
# Seeds the fixtures the cluster-secrets tests read into a Vault dev server at
# $VAULT_ADDR (root token in $VAULT_TOKEN). Needs the vault CLI, so run it in
# the hashicorp/vault image. TEAMS_WEBHOOK overrides the KV v2 fixture, which
# lets a test prove a value is read live rather than from a cache.
#
# Dagger: the test module starts `vault server -dev` as a service and runs
# this script in a second container against it.
#
# Local, for the Go integration tests:
#
#   docker run -d --rm --name vault-test -p 8200:8200 \
#     -e VAULT_DEV_ROOT_TOKEN_ID=root -e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
#     -e SKIP_SETCAP=1 hashicorp/vault:1.20 server -dev
#   docker exec -i -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN=root \
#     vault-test sh < secrets/tests/vault-seed.sh
#   CLUSTERSECRETS_VAULT_ADDR=http://127.0.0.1:8200 go test ./secrets/clustersecrets/
set -eu
: "${VAULT_ADDR:?VAULT_ADDR is not set}"
: "${VAULT_TOKEN:?VAULT_TOKEN is not set}"
export VAULT_ADDR VAULT_TOKEN

i=0
until vault status >/dev/null 2>&1; do
  i=$((i + 1))
  [ "$i" -lt 150 ] || { echo "vault at $VAULT_ADDR not reachable" >&2; exit 1; }
  sleep 0.2
done

# KV v2 at secret/ (dev default)
vault kv put secret/edge/teams webhook="${TEAMS_WEBHOOK:-https://example.invalid/hook}" >/dev/null
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

echo "vault seeded"

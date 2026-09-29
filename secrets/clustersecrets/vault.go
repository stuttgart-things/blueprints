package clustersecrets

import (
	"encoding/json"
	"fmt"
)

// VaultFetchScript reads one Vault API path per line from $1 and prints a
// single JSON object {"<path>": <response body>, ...}.
//
// Auth: VAULT_TOKEN, or an AppRole login with VAULT_ROLE_ID/VAULT_SECRET_ID
// (the same variables dagger/vault's WithAppRoleEnv sets). VAULT_CACERT
// points curl at a private CA. Connection failures are retried briefly;
// HTTP errors (403, 404) are not, they fail with Vault's own error body.
//
// Kept in this package so it can be exercised against a real Vault without
// an engine; the module runs it in an alpine container.
const VaultFetchScript = `#!/bin/sh
set -eu
: "${VAULT_ADDR:?VAULT_ADDR is not set}"
VAULT_ADDR="${VAULT_ADDR%/}"

vcurl() {
  curl -sS --retry 5 --retry-connrefused --retry-delay 1 \
    ${VAULT_CACERT:+--cacert "$VAULT_CACERT"} \
    ${VAULT_NAMESPACE:+-H "X-Vault-Namespace: $VAULT_NAMESPACE"} \
    -w '\n%{http_code}' "$@"
}

# split "<body>\n<status>" from vcurl; fail on non-2xx
check() {
  status=$(printf '%s' "$1" | tail -n1)
  body=$(printf '%s' "$1" | sed '$d')
  case "$status" in
    2??) printf '%s' "$body" ;;
    *) echo "vault: $2 -> HTTP $status: $body" >&2; return 1 ;;
  esac
}

if [ -z "${VAULT_TOKEN:-}" ]; then
  : "${VAULT_ROLE_ID:?neither VAULT_TOKEN nor VAULT_ROLE_ID is set}"
  : "${VAULT_SECRET_ID:?VAULT_SECRET_ID is not set}"
  login=$(jq -n --arg r "$VAULT_ROLE_ID" --arg s "$VAULT_SECRET_ID" '{role_id:$r,secret_id:$s}')
  resp=$(vcurl -X POST -d "$login" "$VAULT_ADDR/v1/auth/approle/login")
  VAULT_TOKEN=$(check "$resp" "approle login" | jq -er .auth.client_token)
fi

out=$(mktemp)
echo '{}' > "$out"
while IFS= read -r p || [ -n "$p" ]; do
  [ -n "$p" ] || continue
  resp=$(vcurl -H "X-Vault-Token: $VAULT_TOKEN" "$VAULT_ADDR/v1/$p")
  body=$(check "$resp" "GET $p")
  printf '%s' "$body" | jq --arg p "$p" --slurpfile cur "$out" -n '$cur[0] + {($p): input}' > "$out.new"
  mv "$out.new" "$out"
done < "$1"
cat "$out"
`

// ParseVaultBatch splits VaultFetchScript's output into path -> body.
func ParseVaultBatch(out []byte) (map[string][]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse vault fetch output: %w", err)
	}
	docs := make(map[string][]byte, len(raw))
	for k, v := range raw {
		docs[k] = v
	}
	return docs, nil
}

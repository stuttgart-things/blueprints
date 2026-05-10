package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dagger/argocd/internal/dagger"

	"gopkg.in/yaml.v3"
)

// vaultEnv is the decoded shape of the SOPS-encrypted vault env yaml the
// caller provides via --vault-env-file. Only `vault_addr` + `vault_token`
// are mandatory; `vault_skip_verify` defaults to true.
type vaultEnv struct {
	VaultAddr       string `yaml:"vault_addr"`
	VaultToken      string `yaml:"vault_token"`
	VaultSkipVerify *bool  `yaml:"vault_skip_verify,omitempty"`
}

// vaultPolicyHCL is the ACL policy applied to the Vault server before
// minting the cert-manager token. Idempotent — `PUT
// /v1/sys/policies/acl/<name>` upserts.
const vaultPolicyHCL = `path "pki/issue/*" {
  capabilities = ["create", "update"]
}
path "pki/sign/*" {
  capabilities = ["create", "update"]
}
`

// CreateVaultIssuer prepares the cluster-side artefacts cert-manager needs
// to use a remote Vault PKI as a ClusterIssuer. It does NOT touch the
// target cluster — instead it talks to Vault directly (HTTP API), then
// renders + SOPS-encrypts two Kubernetes Secret manifests for the caller
// to commit to git. ArgoCD AppSets then reconcile those Secrets onto the
// target cluster and create the ClusterIssuer alongside.
//
// What it does on the Vault side (idempotent):
//  1. PUT /v1/sys/policies/acl/<policyName> — applies an ACL policy that
//     allows `create/update` on `pki/issue/*` and `pki/sign/*`.
//  2. POST /v1/auth/token/create — mints a renewable Vault token bound to
//     that policy, with display_name `cert-manager-<clusterName>`. Each
//     run produces a fresh token; the previous one is left to expire on
//     its TTL (defaulting to 1y, renewable). Caller commits the resulting
//     Secret to git; cert-manager reloads the new value on the next
//     Issue / CertificateRequest.
//  3. GET /v1/pki/ca/pem — reads the current PKI root CA bundle.
//
// What it returns (a *dagger.Directory with two files):
//   - <tokenSecretName>.yaml — SOPS-encrypted v1/Secret in
//     <targetNamespace> with data.token = <vault token>.
//   - <caSecretName>.yaml — SOPS-encrypted v1/Secret in <targetNamespace>
//     with data["ca.crt"] = <PKI CA PEM>.
//
// Closes #162.
func (m *Argocd) CreateVaultIssuer(
	ctx context.Context,
	// Target cluster name; used in the Vault token's display_name and as
	// the de-facto idempotency key for the rendered manifests' filenames.
	clusterName string,
	// SOPS-encrypted KV-YAML with `vault_addr`, `vault_token`,
	// `vault_skip_verify`.
	vaultEnvFile *dagger.File,
	// AGE private key for decrypting `--vault-env-file`.
	sopsKey *dagger.Secret,
	// AGE public key for re-encrypting the rendered Secret manifests.
	agePublicKey *dagger.Secret,
	// Vault PKI role name (matches the Vault PKI mount's pre-existing
	// role; the rendered ClusterIssuer references this role).
	// +optional
	// +default="sthings-vsphere"
	pkiRole string,
	// Vault ACL policy name to upsert. The policy grants
	// `pki/issue/*` + `pki/sign/*`.
	// +optional
	// +default="pki-issue"
	policyName string,
	// Namespace of the rendered cert-manager Secrets.
	// +optional
	// +default="cert-manager"
	targetNamespace string,
	// Name of the rendered Secret containing the Vault token.
	// +optional
	// +default="cert-manager-vault-token"
	tokenSecretName string,
	// Name of the rendered Secret containing the PKI CA bundle.
	// +optional
	// +default="vault-pki-ca"
	caSecretName string,
	// TTL for the minted Vault token. Forwarded to Vault as the `ttl`
	// field on `auth/token/create`. Reasonable upper bound; cert-manager
	// can renew the token before expiry.
	// +optional
	// +default="8760h"
	tokenTtl string,
	// SOPS config file (.sops.yaml) to use during re-encryption of the
	// rendered Secret manifests. Pass when you want recipient/regex
	// selection beyond the default `--age-public-key`.
	// +optional
	sopsConfig *dagger.File,
) (*dagger.Directory, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster-name is required")
	}
	if vaultEnvFile == nil {
		return nil, fmt.Errorf("vault-env-file is required")
	}
	if sopsKey == nil {
		return nil, fmt.Errorf("sops-key is required")
	}
	if agePublicKey == nil {
		return nil, fmt.Errorf("age-public-key is required")
	}

	// Decrypt the vault env yaml and parse the connection details out of it.
	envYaml, err := dag.Secrets().Decrypt(ctx, sopsKey, vaultEnvFile)
	if err != nil {
		return nil, fmt.Errorf("decrypt vault-env-file: %w", err)
	}
	var env vaultEnv
	if err := yaml.Unmarshal([]byte(envYaml), &env); err != nil {
		return nil, fmt.Errorf("parse vault-env-file as yaml: %w", err)
	}
	if env.VaultAddr == "" {
		return nil, fmt.Errorf("vault-env-file is missing vault_addr")
	}
	if env.VaultToken == "" {
		return nil, fmt.Errorf("vault-env-file is missing vault_token")
	}
	skipVerify := true
	if env.VaultSkipVerify != nil {
		skipVerify = *env.VaultSkipVerify
	}

	// 1. Apply the policy + 2. mint the token + 3. read the CA bundle in
	//    a single transient curl+jq container. The vault admin token is
	//    injected as a Secret; the policy HCL and CA come back over
	//    stdout-mounted files in the container's workspace.
	tokenStr, caPEM, err := m.vaultProvision(ctx, env.VaultAddr, env.VaultToken, skipVerify, policyName, clusterName, tokenTtl)
	if err != nil {
		return nil, fmt.Errorf("vault provision: %w", err)
	}

	// Render the two Kubernetes Secret manifests. Both use stringData so
	// the rendered YAML is human-diffable; SOPS will encrypt the whole
	// document anyway.
	tokenManifest := renderSecretManifest(tokenSecretName, targetNamespace, map[string]string{
		"token": tokenStr,
	})
	caManifest := renderSecretManifest(caSecretName, targetNamespace, map[string]string{
		"ca.crt": caPEM,
	})

	tokenFile := dag.Directory().WithNewFile(tokenSecretName+".yaml", tokenManifest).File(tokenSecretName + ".yaml")
	caFile := dag.Directory().WithNewFile(caSecretName+".yaml", caManifest).File(caSecretName + ".yaml")

	// SOPS-encrypt both manifests with the caller's age public key.
	// EncryptFile returns the ciphertext as a string (not a File), so we
	// drop each result back into a Directory under the canonical filename.
	encryptOpts := dagger.SecretsEncryptFileOpts{}
	if sopsConfig != nil {
		encryptOpts.SopsConfig = sopsConfig
	}
	encryptedToken, err := dag.Secrets().EncryptFile(ctx, agePublicKey, tokenFile, encryptOpts)
	if err != nil {
		return nil, fmt.Errorf("sops-encrypt token Secret: %w", err)
	}
	encryptedCA, err := dag.Secrets().EncryptFile(ctx, agePublicKey, caFile, encryptOpts)
	if err != nil {
		return nil, fmt.Errorf("sops-encrypt ca Secret: %w", err)
	}

	out := dag.Directory().
		WithNewFile(tokenSecretName+".yaml", encryptedToken).
		WithNewFile(caSecretName+".yaml", encryptedCA)
	return out, nil
}

// vaultProvision runs three sequential Vault HTTP calls in a single
// alpine+curl+jq container and returns (cert-manager token, PKI CA PEM).
func (m *Argocd) vaultProvision(
	ctx context.Context,
	vaultAddr, vaultToken string,
	skipVerify bool,
	policyName, clusterName, tokenTtl string,
) (token string, caPEM string, err error) {
	addr := strings.TrimRight(vaultAddr, "/")
	curlBase := []string{"curl", "-fsS"}
	if skipVerify {
		curlBase = append(curlBase, "-k")
	}
	curlBaseStr := strings.Join(curlBase, " ")

	// Vault expects the policy body as an *escaped string* under the
	// `policy` key of a JSON object; encoding via json.Marshal handles
	// the escaping safely.
	policyPayload, err := json.Marshal(map[string]string{"policy": vaultPolicyHCL})
	if err != nil {
		return "", "", fmt.Errorf("marshal policy payload: %w", err)
	}

	tokenPayload, err := json.Marshal(map[string]any{
		"policies":     []string{policyName},
		"display_name": "cert-manager-" + clusterName,
		"ttl":          tokenTtl,
		"renewable":    true,
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal token payload: %w", err)
	}

	// Single shell session does all three calls; intermediate JSON is
	// piped through jq to extract `auth.client_token`. Outputs land in
	// /out/token and /out/ca.pem so we can read them as Files afterwards.
	script := strings.Join([]string{
		"set -euo pipefail",
		"mkdir -p /out",
		// 1. PUT policy.
		fmt.Sprintf(`%s -X PUT -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" --data %q "%s/v1/sys/policies/acl/%s"`,
			curlBaseStr, string(policyPayload), addr, policyName),
		// 2. POST token, extract client_token.
		fmt.Sprintf(`%s -X POST -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" --data %q "%s/v1/auth/token/create" | jq -r .auth.client_token > /out/token`,
			curlBaseStr, string(tokenPayload), addr),
		// 3. GET CA PEM.
		fmt.Sprintf(`%s -H "X-Vault-Token: ${VAULT_TOKEN}" "%s/v1/pki/ca/pem" > /out/ca.pem`,
			curlBaseStr, addr),
		// Sanity: both outputs non-empty.
		`test -s /out/token`,
		`test -s /out/ca.pem`,
	}, "\n")

	vaultTokenSecret := dag.SetSecret("vault-issuer-admin-token", vaultToken)
	// Force a fresh exec across runs so we don't pull a stale token from
	// the dagger op cache (the cache key is derived from the script + env,
	// neither of which embed the live Vault state).
	cacheBuster := time.Now().UTC().Format(time.RFC3339Nano)

	ctr := dag.Container().
		From("alpine:3.21").
		WithExec([]string{"apk", "add", "--no-cache", "curl", "jq"}).
		WithSecretVariable("VAULT_TOKEN", vaultTokenSecret).
		WithEnvVariable("CACHE_BUSTER", cacheBuster).
		WithExec([]string{"sh", "-c", script})

	tokenOut, err := ctr.File("/out/token").Contents(ctx)
	if err != nil {
		return "", "", fmt.Errorf("read minted token: %w", err)
	}
	caOut, err := ctr.File("/out/ca.pem").Contents(ctx)
	if err != nil {
		return "", "", fmt.Errorf("read CA bundle: %w", err)
	}
	return strings.TrimSpace(tokenOut), strings.TrimSuffix(caOut, "\n"), nil
}

// renderSecretManifest emits a v1/Secret YAML. Values are base64-encoded
// into `data:` (rather than `stringData:`) so the manifest matches what
// sops-encrypted Secrets in this repo conventionally look like and is
// idempotent vs. round-tripping through `kubectl apply --dry-run`.
func renderSecretManifest(name, namespace string, values map[string]string) string {
	var b strings.Builder
	b.WriteString("apiVersion: v1\n")
	b.WriteString("kind: Secret\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + name + "\n")
	b.WriteString("  namespace: " + namespace + "\n")
	b.WriteString("type: Opaque\n")
	b.WriteString("data:\n")
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	// Tiny sort — len is at most 2 in practice.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		b.WriteString("  " + k + ": " + base64.StdEncoding.EncodeToString([]byte(values[k])) + "\n")
	}
	return b.String()
}

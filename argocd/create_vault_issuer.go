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
// caller provides via --vault-env-file. Only `vaultAddr` + `vaultToken`
// are mandatory; `vaultSkipVerify` defaults to true. `vaultCaBundle`,
// when set, is the base64-encoded PKI root CA PEM and short-circuits the
// live fetch from `${vaultAddr}/v1/pki/ca/pem` (useful when the CA is
// known and stable).
type vaultEnv struct {
	VaultAddr       string `yaml:"vaultAddr"`
	VaultToken      string `yaml:"vaultToken"`
	VaultSkipVerify *bool  `yaml:"vaultSkipVerify,omitempty"`
	VaultCaBundle   string `yaml:"vaultCaBundle,omitempty"`
	// Pin this Vault's hostname to an address, `host:ip` or curl's full
	// `host:port:ip`. Empty means ordinary DNS.
	//
	// It belongs to the env file rather than to a caller flag because it is a
	// property of THIS Vault's address, not of who is calling: a lab zone that
	// only a resolver inside the lab can follow stays unfollowable no matter
	// which pipeline reaches for it.
	//
	// Why it exists: these functions curl from inside a Dagger container, and
	// that container resolves through the ENGINE's resolver -- the runner VM's
	// /etc/resolv.conf, not the cluster CoreDNS the job's pod uses. A name the
	// runner shell resolves can be unresolvable one layer down, and the lab
	// recursor here answers for such a zone only intermittently: measured
	// 10/10 one minute and 0/10 the next, with SOA queries timing out.
	VaultResolve string `yaml:"vaultResolve,omitempty"`
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

// CreateVaultIssuer prepares the cluster-side prerequisites cert-manager
// needs to use a remote Vault PKI as a `ClusterIssuer`. It does NOT
// create the `ClusterIssuer` itself — that's the
// `cert-manager-vault-pki` AppSet's job, which references the two
// Secrets this function lands on the target cluster.
//
// What it does (in one Dagger session):
//  1. Decrypts the vault env yaml.
//  2. Talks to Vault directly (HTTP API) to upsert an ACL policy + mint
//     a renewable token + read the PKI CA. (No Terraform.)
//  3. Decrypts the target cluster's kubeconfig.
//  4. Applies a 3-document YAML directly to the target cluster:
//     - `Namespace/<target-namespace>` (default `cert-manager`)
//     - `Secret/<token-secret-name>` (default `cert-manager-vault-token`,
//     `data.token` = freshly-minted vault token)
//     - `Secret/<ca-secret-name>` (default `vault-pki-ca`,
//     `data["ca.crt"]` = live-fetched PKI root CA PEM)
//
// Only plain core/v1 resources are created; no cert-manager CRDs, no
// admission-webhook coupling. The AppSet that creates the ClusterIssuer
// runs whenever it's ready — order-independent.
//
// Closes #162.
func (m *Argocd) CreateVaultIssuer(
	ctx context.Context,
	// Target cluster name; used in the Vault token's display_name.
	clusterName string,
	// SOPS-encrypted kubeconfig of the target cluster.
	kubeconfigSourceFile *dagger.File,
	// SOPS-encrypted KV-YAML with `vaultAddr`, `vaultToken`,
	// optional `vaultSkipVerify` (defaults to true), and optional
	// `vaultCaBundle` (base64-encoded PKI root CA PEM; when set, the
	// function uses it directly instead of live-fetching from
	// `${vaultAddr}/v1/pki/ca/pem`).
	vaultEnvFile *dagger.File,
	// AGE private key for decrypting both files.
	sopsKey *dagger.Secret,
	// Vault PKI role name (matches the Vault PKI mount's pre-existing
	// role; the rendered ClusterIssuer references this role).
	// +optional
	// +default="sthings-vsphere"
	pkiRole string,
	// Vault ACL policy name to upsert.
	// +optional
	// +default="pki-issue"
	policyName string,
	// Namespace to create + place the cert-manager Secrets in.
	// +optional
	// +default="cert-manager"
	targetNamespace string,
	// Name of the Secret containing the Vault token.
	// +optional
	// +default="cert-manager-vault-token"
	tokenSecretName string,
	// Name of the Secret containing the PKI CA bundle.
	// +optional
	// +default="vault-pki-ca"
	caSecretName string,
	// TTL for the minted Vault token. Renewable.
	// +optional
	// +default="8760h"
	tokenTtl string,

	// Arbitrary string mixed into the function-level cache key. Pass a
	// timestamp (e.g. `date +%s%N`) from CI to force a fresh execution
	// when the function's other args are deterministic but the side
	// effects (Vault token mint, kubectl apply) must replay. Inner
	// CACHE_BUSTER env vars on individual containers don't help here —
	// Dagger short-circuits the whole function on input-hash match
	// before any container in the body is instantiated.
	// +optional
	cacheBuster string,
) (string, error) {
	_ = cacheBuster
	if clusterName == "" {
		return "", fmt.Errorf("cluster-name is required")
	}
	if kubeconfigSourceFile == nil {
		return "", fmt.Errorf("kubeconfig-source-file is required")
	}
	if vaultEnvFile == nil {
		return "", fmt.Errorf("vault-env-file is required")
	}
	if sopsKey == nil {
		return "", fmt.Errorf("sops-key is required")
	}

	// Decrypt the vault env yaml and parse the connection details.
	envYaml, err := dag.Secrets().Decrypt(ctx, sopsKey, vaultEnvFile)
	if err != nil {
		return "", fmt.Errorf("decrypt vault-env-file: %w", err)
	}
	var env vaultEnv
	if err := yaml.Unmarshal([]byte(envYaml), &env); err != nil {
		return "", fmt.Errorf("parse vault-env-file as yaml: %w", err)
	}
	if env.VaultAddr == "" {
		return "", fmt.Errorf("vault-env-file is missing vaultAddr")
	}
	if env.VaultToken == "" {
		return "", fmt.Errorf("vault-env-file is missing vaultToken")
	}
	skipVerify := true
	if env.VaultSkipVerify != nil {
		skipVerify = *env.VaultSkipVerify
	}

	// Decrypt the kubeconfig and wrap it as a Secret so dag.Kubernetes()
	// can mount it for the kubectl apply.
	kubeconfigYaml, err := dag.Secrets().Decrypt(ctx, sopsKey, kubeconfigSourceFile)
	if err != nil {
		return "", fmt.Errorf("decrypt kubeconfig: %w", err)
	}
	kubeconfigSecret := dag.SetSecret("vault-issuer-kubeconfig", kubeconfigYaml)

	// Vault-side: upsert policy, mint token. CA bundle: prefer
	// `vaultCaBundle` from the env file when present (already
	// base64-encoded PEM), else live-fetch via GET /v1/pki/ca/pem.
	tokenStr, livePEM, err := m.vaultProvision(ctx, env.VaultAddr, env.VaultToken, skipVerify, policyName, clusterName, tokenTtl, env.VaultCaBundle == "")
	if err != nil {
		return "", fmt.Errorf("vault provision: %w", err)
	}
	caB64 := env.VaultCaBundle
	if caB64 == "" {
		caB64 = base64.StdEncoding.EncodeToString([]byte(livePEM))
	}

	// Render the 3-document YAML via dag.Templating().RenderInline so the
	// inline template stays inspectable + extendable (additional fields,
	// optional documents) without touching format-string plumbing.
	manifestVars, err := json.Marshal(map[string]string{
		"namespace":       targetNamespace,
		"tokenSecretName": tokenSecretName,
		"caSecretName":    caSecretName,
		"tokenB64":        base64.StdEncoding.EncodeToString([]byte(tokenStr)),
		"caB64":           caB64,
	})
	if err != nil {
		return "", fmt.Errorf("marshal manifest vars: %w", err)
	}
	manifest, err := dag.Templating().RenderInline(ctx, vaultIssuerManifestTemplate,
		dagger.TemplatingRenderInlineOpts{Variables: string(manifestVars)})
	if err != nil {
		return "", fmt.Errorf("render manifest template: %w", err)
	}
	manifestFile := dag.Directory().
		WithNewFile("vault-issuer.yaml", manifest).
		File("vault-issuer.yaml")

	// kubectl apply via the canonical wrapper. ServerSide=true ⇒ no
	// last-applied-configuration drift if the user ever runs
	// `kubectl apply -f` by hand later. The kubernetes module sets
	// `KUBECONFIG` itself as of dagger v0.112.1, so no `--kubeconfig=`
	// override is needed here.
	output, err := dag.Kubernetes().Kubectl(ctx, dagger.KubernetesKubectlOpts{
		Operation:  "apply",
		SourceFile: manifestFile,
		KubeConfig: kubeconfigSecret,
		ServerSide: true,
	})
	if err != nil {
		return "", fmt.Errorf("kubectl apply: %w", err)
	}
	return output, nil
}

// vaultProvision runs the Vault HTTP calls in a single alpine+curl+jq
// container and returns (cert-manager token, PKI CA PEM). When
// `fetchCA=false` the CA fetch is skipped and the second return value is
// empty — the caller is expected to source the CA from the env file's
// `vaultCaBundle`.
func (m *Argocd) vaultProvision(
	ctx context.Context,
	vaultAddr, vaultToken string,
	skipVerify bool,
	policyName, clusterName, tokenTtl string,
	fetchCA bool,
) (token string, caPEM string, err error) {
	addr := strings.TrimRight(vaultAddr, "/")
	curlBase := []string{"curl", "-fsS"}
	if skipVerify {
		curlBase = append(curlBase, "-k")
	}
	curlBaseStr := strings.Join(curlBase, " ")

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

	steps := []string{
		"set -euo pipefail",
		"mkdir -p /out",
		fmt.Sprintf(`%s -X PUT -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" --data %q "%s/v1/sys/policies/acl/%s"`,
			curlBaseStr, string(policyPayload), addr, policyName),
		fmt.Sprintf(`%s -X POST -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" --data %q "%s/v1/auth/token/create" | jq -r .auth.client_token > /out/token`,
			curlBaseStr, string(tokenPayload), addr),
		`test -s /out/token`,
	}
	if fetchCA {
		steps = append(steps,
			fmt.Sprintf(`%s -H "X-Vault-Token: ${VAULT_TOKEN}" "%s/v1/pki/ca/pem" > /out/ca.pem`,
				curlBaseStr, addr),
			`test -s /out/ca.pem`,
		)
	}
	script := strings.Join(steps, "\n")

	vaultTokenSecret := dag.SetSecret("vault-issuer-admin-token", vaultToken)
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
	if !fetchCA {
		return strings.TrimSpace(tokenOut), "", nil
	}
	caOut, err := ctr.File("/out/ca.pem").Contents(ctx)
	if err != nil {
		return "", "", fmt.Errorf("read CA bundle: %w", err)
	}
	return strings.TrimSpace(tokenOut), strings.TrimSuffix(caOut, "\n"), nil
}

// vaultIssuerManifestTemplate is the multi-document YAML rendered into
// the cluster: target Namespace + two cert-manager Secrets cert-manager
// will read when reconciling the (separately-managed) `vault-pki`
// ClusterIssuer. Rendered via dag.Templating().RenderInline so future
// additions (e.g. ServiceAccount, RBAC) plug in without touching
// format-string plumbing.
const vaultIssuerManifestTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .namespace }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .tokenSecretName }}
  namespace: {{ .namespace }}
type: Opaque
data:
  token: {{ .tokenB64 }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .caSecretName }}
  namespace: {{ .namespace }}
type: Opaque
data:
  ca.crt: {{ .caB64 }}
`

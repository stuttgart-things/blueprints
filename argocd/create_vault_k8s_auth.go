package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dagger/argocd/internal/dagger"

	"gopkg.in/yaml.v3"
)

// kubeconfigShape is a minimal YAML decoder for pulling the cluster API
// server URL out of a kubeconfig. We mount the full kubeconfig into the
// container as a Secret; only `clusters[0].cluster.server` needs to come
// back to the host side (it's the `kubernetes_host` value Vault stores
// in the auth backend config).
type kubeconfigShape struct {
	Clusters []struct {
		Cluster struct {
			Server string `yaml:"server"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
}

// CreateVaultK8sAuth provisions the cluster-side prerequisites and the
// Vault-side Kubernetes auth backend an in-cluster ServiceAccount uses
// to authenticate to Vault and consume one or more pre-existing
// policies. Mirrors the layout in CreateVaultIssuer: decrypt SOPS
// inputs → `kubectl apply` core/v1 resources → drive the Vault HTTP
// API directly. No Terraform, no Helm.
//
// What it creates (in one Dagger session):
//
//   - On the target cluster:
//     `Namespace/<namespace>`,
//     `ServiceAccount/<auth-name>`,
//     non-expiring SA-token `Secret/<auth-name>` (type
//     `kubernetes.io/service-account-token`),
//     `ClusterRoleBinding/<auth-name>` → `system:auth-delegator`
//     (so Vault can call TokenReview with the SA's JWT).
//
//   - On Vault:
//     a Kubernetes auth backend mounted at `<cluster-name>-<auth-name>`,
//     configured with the kubeconfig's API server + the SA's CA +
//     reviewer JWT (`disable_iss_validation=true`,
//     `disable_local_ca_jwt=true` — matches the vault-base-setup
//     Terraform module so newer K8s won't reject the validation), and
//     one auth role bound to the SA with the given token policies.
//
// Policies are NOT created by this function — they must already exist
// in Vault (typically owned by a separate per-KV-mount pipeline like
// `vault-homerun2-secrets`). Pass `--token-policies` as a
// comma-separated list; one cluster can bind to one policy, multiple
// policies, or share a policy with other clusters.
//
// Idempotent: re-runs upsert the config + role and skip the auth-mount
// step when the path is already in use.
func (m *Argocd) CreateVaultK8sAuth(
	ctx context.Context,
	// Target cluster name; prefixes the Vault auth backend path
	// (`<cluster-name>-<auth-name>`).
	clusterName string,
	// SOPS-encrypted kubeconfig of the target cluster.
	kubeconfigSourceFile *dagger.File,
	// SOPS-encrypted env YAML with `vaultAddr`, `vaultToken`, optional
	// `vaultSkipVerify` (defaults true). Same shape as create-vault-issuer.
	vaultEnvFile *dagger.File,
	// AGE private key for decrypting both files.
	sopsKey *dagger.Secret,
	// Auth backend + role name. The Vault mount path becomes
	// `<cluster-name>-<auth-name>`. The SA, the SA-token Secret, the CRB
	// and the role on the Vault side all reuse this name.
	// +optional
	// +default="eso"
	authName string,
	// Namespace on the target cluster for the SA + SA-token Secret.
	// +optional
	// +default="external-secrets"
	namespace string,
	// Comma-separated Vault policies to bind to the role. Must already
	// exist in Vault. Per-cluster — each caller supplies its own list.
	// +optional
	// +default="read-homerun2-pr"
	tokenPolicies string,
	// TTL (seconds) for tokens minted via this role.
	// +optional
	// +default="3600"
	tokenTtl string,
	// Cache buster — pass a timestamp/run-id from CI to force a fresh
	// execution. Same reason as CreateVaultIssuer: Dagger short-circuits
	// the whole function on input-hash match before any container in
	// the body is instantiated.
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
	policies := []string{}
	for _, p := range strings.Split(tokenPolicies, ",") {
		if p = strings.TrimSpace(p); p != "" {
			policies = append(policies, p)
		}
	}
	if len(policies) == 0 {
		return "", fmt.Errorf("token-policies must be a non-empty comma-separated list")
	}

	// Decrypt the vault env yaml (reuses the vaultEnv struct defined in
	// create_vault_issuer.go — same package, same shape).
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

	// Decrypt kubeconfig once: parse the API server URL up here (need
	// it as a Vault config field) and wrap the full document as a
	// Secret for in-container kubectl.
	kubeconfigYaml, err := dag.Secrets().Decrypt(ctx, sopsKey, kubeconfigSourceFile)
	if err != nil {
		return "", fmt.Errorf("decrypt kubeconfig: %w", err)
	}
	var kc kubeconfigShape
	if err := yaml.Unmarshal([]byte(kubeconfigYaml), &kc); err != nil {
		return "", fmt.Errorf("parse kubeconfig: %w", err)
	}
	if len(kc.Clusters) == 0 || kc.Clusters[0].Cluster.Server == "" {
		return "", fmt.Errorf("kubeconfig has no clusters[0].cluster.server")
	}
	apiServer := kc.Clusters[0].Cluster.Server
	kubeconfigSecret := dag.SetSecret("vault-k8s-auth-kubeconfig", kubeconfigYaml)

	// Phase 1: render + kubectl apply Namespace + SA + SA-token Secret + CRB.
	manifestVars, err := json.Marshal(map[string]string{
		"namespace": namespace,
		"name":      authName,
	})
	if err != nil {
		return "", fmt.Errorf("marshal manifest vars: %w", err)
	}
	manifest, err := dag.Templating().RenderInline(ctx, vaultK8sAuthManifestTemplate,
		dagger.TemplatingRenderInlineOpts{Variables: string(manifestVars)})
	if err != nil {
		return "", fmt.Errorf("render manifest template: %w", err)
	}
	manifestFile := dag.Directory().
		WithNewFile("vault-k8s-auth.yaml", manifest).
		File("vault-k8s-auth.yaml")
	applyOut, err := dag.Kubernetes().Kubectl(ctx, dagger.KubernetesKubectlOpts{
		Operation:  "apply",
		SourceFile: manifestFile,
		KubeConfig: kubeconfigSecret,
		ServerSide: true,
	})
	if err != nil {
		return "", fmt.Errorf("kubectl apply: %w", err)
	}

	// Phase 2: in a kubectl+curl+jq container, wait for kubelet to
	// populate the SA-token Secret, then drive the Vault HTTP API.
	vaultOut, err := m.vaultK8sAuthConfigure(
		ctx,
		env.VaultAddr, env.VaultToken, skipVerify,
		apiServer, clusterName, authName, namespace,
		policies, tokenTtl, kubeconfigSecret,
	)
	if err != nil {
		return "", fmt.Errorf("configure vault k8s auth: %w", err)
	}
	return applyOut + "\n" + vaultOut, nil
}

// vaultK8sAuthConfigure waits for the SA-token Secret to be populated
// by kubelet, then issues three Vault HTTP calls: ensure the auth mount
// exists, upsert the backend config (host + reviewer JWT + CA), upsert
// the role. All in a single alpine/k8s container (ships kubectl + curl
// + jq).
func (m *Argocd) vaultK8sAuthConfigure(
	ctx context.Context,
	vaultAddr, vaultToken string,
	skipVerify bool,
	apiServer, clusterName, authName, namespace string,
	policies []string, tokenTtl string,
	kubeconfigSecret *dagger.Secret,
) (string, error) {
	addr := strings.TrimRight(vaultAddr, "/")
	mountPath := fmt.Sprintf("%s-%s", clusterName, authName)
	curlBase := `curl -fsS`
	if skipVerify {
		curlBase += " -k"
	}

	mountPayload, err := json.Marshal(map[string]string{"type": "kubernetes"})
	if err != nil {
		return "", fmt.Errorf("marshal mount payload: %w", err)
	}
	rolePayload, err := json.Marshal(map[string]any{
		"bound_service_account_names":      []string{authName},
		"bound_service_account_namespaces": []string{namespace},
		"token_ttl":                        tokenTtl,
		"token_policies":                   policies,
	})
	if err != nil {
		return "", fmt.Errorf("marshal role payload: %w", err)
	}

	// jsonpath uses `\.` to escape the literal `.` in `ca.crt`. Go raw
	// string + bash single-quote both preserve it byte-for-byte.
	waitForSecret := fmt.Sprintf(`for i in $(seq 1 30); do
  TOKEN=$(kubectl -n %[1]s get secret %[2]s -o jsonpath='{.data.token}' 2>/dev/null || true)
  CA=$(kubectl -n %[1]s get secret %[2]s -o jsonpath='{.data.ca\.crt}' 2>/dev/null || true)
  if [ -n "$TOKEN" ] && [ -n "$CA" ]; then break; fi
  sleep 2
done
if [ -z "$TOKEN" ] || [ -z "$CA" ]; then
  echo "SA-token Secret %[1]s/%[2]s never populated after 60s" >&2
  exit 1
fi
TOKEN_JWT=$(printf %%s "$TOKEN" | base64 -d)
CA_PEM=$(printf %%s "$CA" | base64 -d)`, namespace, authName)

	// Ensure auth mount exists. Vault returns HTTP 400 with body
	// `path is already in use` when the mount is present — treat that
	// as success (idempotent re-run).
	ensureMount := fmt.Sprintf(`HTTP=$(%s -o /tmp/mount.out -w '%%{http_code}' -X POST \
  -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" \
  --data %q "%s/v1/sys/auth/%s" || true)
case "$HTTP" in
  2*) ;;
  400) grep -q "path is already in use" /tmp/mount.out || { cat /tmp/mount.out >&2; exit 1; } ;;
  *)   cat /tmp/mount.out >&2; exit 1 ;;
esac`, curlBase, string(mountPayload), addr, mountPath)

	// Upsert backend config. Build the JSON via jq so the multi-line
	// CA PEM survives shell quoting intact.
	configBackend := fmt.Sprintf(`jq -n --arg host %q --arg ca "$CA_PEM" --arg jwt "$TOKEN_JWT" \
  '{kubernetes_host:$host,kubernetes_ca_cert:$ca,token_reviewer_jwt:$jwt,disable_iss_validation:true,disable_local_ca_jwt:true}' \
  > /tmp/config.json
%s -X POST -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" \
  --data @/tmp/config.json "%s/v1/auth/%s/config"`, apiServer, curlBase, addr, mountPath)

	// Upsert role.
	configRole := fmt.Sprintf(`%s -X POST -H "X-Vault-Token: ${VAULT_TOKEN}" -H "Content-Type: application/json" \
  --data %q "%s/v1/auth/%s/role/%s"
echo "vault k8s auth %s/%s ready (policies: %s)"`,
		curlBase, string(rolePayload), addr, mountPath, authName,
		mountPath, authName, strings.Join(policies, ","))

	script := strings.Join([]string{
		"set -euo pipefail",
		"export KUBECONFIG=/work/kubeconfig",
		waitForSecret,
		ensureMount,
		configBackend,
		configRole,
	}, "\n")

	vaultTokenSecret := dag.SetSecret("vault-k8s-auth-admin-token", vaultToken)
	cacheBuster := time.Now().UTC().Format(time.RFC3339Nano)

	// alpine/k8s ships kubectl + curl + jq + yq in one image — saves an
	// `apk add` round trip and pins the kubectl version to a known
	// K8s minor. Bump the tag in lockstep with cluster K8s versions.
	ctr := dag.Container().
		From("alpine/k8s:1.31.0").
		WithMountedSecret("/work/kubeconfig", kubeconfigSecret).
		WithSecretVariable("VAULT_TOKEN", vaultTokenSecret).
		WithEnvVariable("CACHE_BUSTER", cacheBuster).
		WithExec([]string{"sh", "-c", script})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// vaultK8sAuthManifestTemplate renders the 4-document YAML
// `kubectl apply`'d to the target cluster. Mirrors what
// `vault-base-setup/k8s.tf` creates via the kubernetes_* providers:
// Namespace, ServiceAccount (with automountServiceAccountToken so the
// SA's JWT is available to pods that use it), non-expiring SA-token
// Secret (the controller populates `data.token` + `data["ca.crt"]`
// once the SA exists), ClusterRoleBinding to system:auth-delegator
// (Vault needs this to call TokenReview).
const vaultK8sAuthManifestTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .namespace }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
automountServiceAccountToken: true
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
  annotations:
    kubernetes.io/service-account.name: {{ .name }}
    kubernetes.io/service-account.namespace: {{ .namespace }}
type: kubernetes.io/service-account-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - kind: ServiceAccount
    name: {{ .name }}
    namespace: {{ .namespace }}
`

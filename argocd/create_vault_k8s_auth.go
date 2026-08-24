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

// CreateVaultKubernetesAuth provisions the cluster-side prerequisites and the
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
//     `ServiceAccount/<reviewer-name>`,
//     non-expiring SA-token `Secret/<reviewer-name>` (type
//     `kubernetes.io/service-account-token`),
//     `ClusterRoleBinding/<reviewer-name>` → `system:auth-delegator`
//     (so Vault can call TokenReview with the SA's JWT).
//
// REVIEWER vs BOUND ServiceAccount. By default they are the SAME account:
// `<auth-name>` both reviews tokens and logs in. That is the ESO shape and stays
// the default so existing callers are unaffected. It does not fit every
// workload:
//
//   - cert-manager logs in as its OWN ServiceAccount `cert-manager`, which the
//     chart owns. Creating a second SA and pointing the ClusterIssuer at it
//     works but is a detour.
//   - `system:auth-delegator` is the right to TokenReview ANY token in the
//     cluster. Granting that to a certificate controller is more privilege than
//     the job needs.
//
// Pass `--reviewer-name` / `--reviewer-namespace` to put the reviewer somewhere
// harmless (`vault-auth-reviewer` in `kube-system`) and
// `--bound-service-account-names` / `--bound-service-account-namespaces` to bind
// the workload's real ServiceAccount. That combination is what the crossplane
// `VaultK8sAuth` path emits, so both produce identical Vault state.
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
func (m *Argocd) CreateVaultKubernetesAuth(
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
	// Name of the ServiceAccount Vault uses to call TokenReview. Empty reuses
	// auth-name, i.e. the reviewer and the workload are one account.
	// +optional
	// +default=""
	reviewerName string,
	// Namespace of the reviewer ServiceAccount. Empty reuses namespace.
	// +optional
	// +default=""
	reviewerNamespace string,
	// Comma-separated ServiceAccounts allowed to LOG IN. Empty reuses auth-name.
	// +optional
	// +default=""
	boundServiceAccountNames string,
	// Comma-separated namespaces of those ServiceAccounts. Empty reuses namespace.
	// +optional
	// +default=""
	boundServiceAccountNamespaces string,
	// Comma-separated Vault policies to bind to the role. Must already
	// exist in Vault. Per-cluster — each caller supplies its own list.
	// +optional
	// +default="read-homerun2-pr"
	tokenPolicies string,
	// TTL (seconds) for tokens minted via this role.
	// +optional
	// +default="3600"
	tokenTtl string,
	// Name of a cert-manager ClusterIssuer to create against this mount. Empty
	// creates none and the function stays what it always was: auth only.
	//
	// Set it and the cluster comes back ready to issue certificates -- the CA
	// Secret, the ClusterIssuer and the TokenRequest RBAC all land here, so a
	// caller does not have to reproduce them somewhere else.
	// +optional
	// +default=""
	issuerName string,
	// Vault sign path the issuer uses, e.g. pki/sign/4sthings.tiab.ssc.sva.de.
	// Required when issuer-name is set.
	//
	// Deliberately the SIGN path, not issue: cert-manager only ever uses sign,
	// while issue would additionally let the holder have Vault generate the
	// private key.
	// +optional
	// +default=""
	issuerPkiPath string,
	// cert-manager's cluster resource namespace -- where it looks up a
	// ClusterIssuer's secret references, and where the TokenRequest RBAC has to
	// live.
	// +optional
	// +default="cert-manager"
	issuerNamespace string,
	// Secret holding the Vault CA the issuer verifies Vault with. Fetched live
	// from ${vaultAddr}/v1/pki/ca/pem.
	// +optional
	// +default="vault-pki-ca"
	caSecretName string,
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
	policies := splitCSV(tokenPolicies)
	if len(policies) == 0 {
		return "", fmt.Errorf("token-policies must be a non-empty comma-separated list")
	}

	// Every knob below falls back to the single-account shape, so a caller that
	// passes none of them gets byte-identical behaviour to before.
	if reviewerName == "" {
		reviewerName = authName
	}
	if reviewerNamespace == "" {
		reviewerNamespace = namespace
	}
	boundNames := splitCSV(boundServiceAccountNames)
	if len(boundNames) == 0 {
		boundNames = []string{authName}
	}
	boundNamespaces := splitCSV(boundServiceAccountNamespaces)
	if len(boundNamespaces) == 0 {
		boundNamespaces = []string{namespace}
	}

	// Without a sign path the issuer would be created pointing at nothing and
	// would report Ready anyway -- cert-manager verifies only the login. Fail
	// here instead, where the cause is still visible.
	if issuerName != "" && issuerPkiPath == "" {
		return "", fmt.Errorf("issuer-pki-path is required when issuer-name is set")
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
		"namespace":         namespace,
		"name":              authName,
		"reviewerName":      reviewerName,
		"reviewerNamespace": reviewerNamespace,
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
		apiServer, clusterName, authName,
		reviewerName, reviewerNamespace,
		boundNames, boundNamespaces,
		policies, tokenTtl, kubeconfigSecret,
	)
	if err != nil {
		return "", fmt.Errorf("configure vault k8s auth: %w", err)
	}

	// Phase 3: the cert-manager side, only when asked for.
	if issuerName == "" {
		return applyOut + "\n" + vaultOut, nil
	}

	issuerOut, err := m.vaultK8sAuthIssuer(
		ctx,
		env.VaultAddr, env.VaultToken, skipVerify,
		clusterName, authName,
		issuerName, issuerPkiPath, issuerNamespace, caSecretName,
		boundNames[0], kubeconfigSecret,
	)
	if err != nil {
		return "", fmt.Errorf("create vault clusterissuer: %w", err)
	}
	return applyOut + "\n" + vaultOut + "\n" + issuerOut, nil
}

// vaultK8sAuthIssuer lands everything cert-manager needs to actually issue
// against the mount phase 2 created: the Vault CA in a Secret, the
// ClusterIssuer, and the TokenRequest RBAC.
//
// The CA travels as a FILE inside the container rather than through host-side
// templating -- a PEM is multi-line and every layer it crosses is another place
// to mangle it. `kubectl create secret --dry-run=client | kubectl apply` keeps
// it idempotent without needing to know whether the Secret already exists.
func (m *Argocd) vaultK8sAuthIssuer(
	ctx context.Context,
	vaultAddr, vaultToken string,
	skipVerify bool,
	clusterName, authName string,
	issuerName, issuerPkiPath, issuerNamespace, caSecretName string,
	serviceAccountName string,
	kubeconfigSecret *dagger.Secret,
) (string, error) {
	addr := strings.TrimRight(vaultAddr, "/")
	mountPath := fmt.Sprintf("%s-%s", clusterName, authName)
	curlBase := `curl -fsS`
	if skipVerify {
		curlBase += " -k"
	}

	manifestVars, err := json.Marshal(map[string]string{
		"issuerName":     issuerName,
		"issuerPkiPath":  issuerPkiPath,
		"namespace":      issuerNamespace,
		"caSecretName":   caSecretName,
		"serviceAccount": serviceAccountName,
		"vaultAddr":      addr,
		"authMountPath":  "/v1/auth/" + mountPath,
		"authRole":       authName,
	})
	if err != nil {
		return "", fmt.Errorf("marshal issuer manifest vars: %w", err)
	}
	manifest, err := dag.Templating().RenderInline(ctx, vaultK8sAuthIssuerTemplate,
		dagger.TemplatingRenderInlineOpts{Variables: string(manifestVars)})
	if err != nil {
		return "", fmt.Errorf("render issuer manifest: %w", err)
	}

	script := strings.Join([]string{
		"set -euo pipefail",
		"export KUBECONFIG=/work/kubeconfig",
		fmt.Sprintf(`%s -H "X-Vault-Token: ${VAULT_TOKEN}" "%s/v1/pki/ca/pem" > /tmp/ca.pem`, curlBase, addr),
		`test -s /tmp/ca.pem`,
		fmt.Sprintf(`kubectl create namespace %s --dry-run=client -o yaml | kubectl apply -f -`, issuerNamespace),
		fmt.Sprintf(`kubectl -n %s create secret generic %s --from-file=ca.crt=/tmp/ca.pem --dry-run=client -o yaml | kubectl apply -f -`,
			issuerNamespace, caSecretName),
		`kubectl apply -f /work/issuer.yaml`,
		fmt.Sprintf(`echo "clusterissuer %s -> %s/%s (sa: %s/%s)"`,
			issuerName, addr, issuerPkiPath, issuerNamespace, serviceAccountName),
	}, "\n")

	vaultTokenSecret := dag.SetSecret("vault-k8s-auth-issuer-token", vaultToken)
	cacheBuster := time.Now().UTC().Format(time.RFC3339Nano)

	ctr := dag.Container().
		From("alpine/k8s:1.31.0").
		WithMountedSecret("/work/kubeconfig", kubeconfigSecret).
		WithNewFile("/work/issuer.yaml", manifest).
		WithSecretVariable("VAULT_TOKEN", vaultTokenSecret).
		WithEnvVariable("CACHE_BUSTER", cacheBuster).
		WithExec([]string{"sh", "-c", script})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
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
	apiServer, clusterName, authName string,
	reviewerName, reviewerNamespace string,
	boundNames, boundNamespaces []string,
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
		"bound_service_account_names":      boundNames,
		"bound_service_account_namespaces": boundNamespaces,
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
CA_PEM=$(printf %%s "$CA" | base64 -d)`, reviewerNamespace, reviewerName)

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
echo "vault k8s auth %s/%s ready (reviewer: %s/%s, bound: %s, policies: %s)"`,
		curlBase, string(rolePayload), addr, mountPath, authName,
		mountPath, authName, reviewerNamespace, reviewerName,
		strings.Join(boundNames, ","), strings.Join(policies, ","))

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

// splitCSV turns a comma-separated argument into a trimmed, empty-free slice.
// Dagger has no []string argument type reachable from the CLI, so every list
// crosses the boundary as one string.
func splitCSV(in string) []string {
	out := []string{}
	for _, v := range strings.Split(in, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// vaultK8sAuthManifestTemplate renders the YAML `kubectl apply`'d to the target
// cluster. Mirrors what `vault-base-setup/k8s.tf` creates via the kubernetes_*
// providers: ServiceAccount (with automountServiceAccountToken so the SA's JWT
// is available to pods that use it), non-expiring SA-token Secret (the
// controller populates `data.token` + `data["ca.crt"]` once the SA exists),
// ClusterRoleBinding to system:auth-delegator (Vault needs this to call
// TokenReview).
//
// Only the REVIEWER is created here. The bound (workload) ServiceAccount is
// deliberately left alone: when it differs from the reviewer it belongs to
// something else — cert-manager's SA is owned by its Helm chart, and applying
// our own copy would fight the chart for it.
//
// The reviewer namespace is created only when it differs from .namespace, so
// the default single-account case renders exactly the documents it always did.
const vaultK8sAuthManifestTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .namespace }}
{{- if ne .reviewerNamespace .namespace }}
---
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .reviewerNamespace }}
{{- end }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .reviewerName }}
  namespace: {{ .reviewerNamespace }}
automountServiceAccountToken: true
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .reviewerName }}
  namespace: {{ .reviewerNamespace }}
  annotations:
    kubernetes.io/service-account.name: {{ .reviewerName }}
    kubernetes.io/service-account.namespace: {{ .reviewerNamespace }}
type: kubernetes.io/service-account-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .reviewerName }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - kind: ServiceAccount
    name: {{ .reviewerName }}
    namespace: {{ .reviewerNamespace }}
`

// vaultK8sAuthIssuerTemplate is the cert-manager side of the mount: the
// ClusterIssuer plus the TokenRequest RBAC it cannot work without.
//
// The RBAC is NOT optional and NOT redundant. cert-manager reaches a
// Kubernetes-auth mount by minting a token for the ServiceAccount through the
// TokenRequest API, which needs `create` on `serviceaccounts/token`. The chart
// rendered that Role up to v1.18.x and **v1.21.1 renders no such rule at all**,
// with no values flag to bring it back (verified by rendering both charts). An
// upgrade across that line silently disarms every issuer of this kind -- and
// silently is the word: the ClusterIssuer keeps reporting Ready, because
// cert-manager verifies only the LOGIN, never the ability to sign. The only
// proof is an issued Certificate.
//
// The CA Secret is created separately, by kubectl inside the container, so the
// multi-line PEM never crosses a template boundary.
const vaultK8sAuthIssuerTemplate = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: {{ .issuerName }}
spec:
  vault:
    server: {{ .vaultAddr }}
    path: {{ .issuerPkiPath }}
    caBundleSecretRef:
      name: {{ .caSecretName }}
      key: ca.crt
    auth:
      kubernetes:
        mountPath: {{ .authMountPath }}
        role: {{ .authRole }}
        serviceAccountRef:
          name: {{ .serviceAccount }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cert-manager-tokenrequest
  namespace: {{ .namespace }}
rules:
  - apiGroups: [""]
    resources: ["serviceaccounts/token"]
    resourceNames: ["{{ .serviceAccount }}"]
    verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cert-manager-tokenrequest
  namespace: {{ .namespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cert-manager-tokenrequest
subjects:
  - kind: ServiceAccount
    name: {{ .serviceAccount }}
    namespace: {{ .namespace }}
`

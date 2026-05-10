package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dagger/argocd/internal/dagger"

	"gopkg.in/yaml.v3"
)

// vaultIssuerTfTemplate is rendered into a single main.tf for the
// vault-base-setup invocation that creates a cert-manager `vault-pki`
// ClusterIssuer (plus its supporting AppRole, policy, and CA Secret) on a
// target downstream cluster. Templated at runtime so we can inline the
// k8s-secret backend's `secret_suffix` without an extra .tf file.
const vaultIssuerTfTemplate = `terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
  backend "kubernetes" {
    secret_suffix = "{{ .secret_suffix }}"
    config_path   = "{{ .kubeconfig_path }}"
    namespace     = "kube-system"
  }
}

variable "vault_addr"      { type = string }
variable "kubeconfig_path" { type = string }
variable "cluster_name"    { type = string }
variable "vault_ca_bundle" { type = string }

provider "kubernetes" {
  config_path = var.kubeconfig_path
}

module "vault-base-setup" {
  source          = "github.com/stuttgart-things/vault-base-setup?ref=v1.0.0"
  vault_addr      = var.vault_addr
  skip_tls_verify = true
  kubeconfig_path = var.kubeconfig_path
  cluster_name    = var.cluster_name

  csi_enabled = false
  vso_enabled = false
  pki_enabled = false

  certmanager_vault_issuer_enabled     = true
  certmanager_vault_issuer_pki_role    = "{{ .pki_role }}"
  certmanager_vault_issuer_server      = var.vault_addr
  certmanager_vault_issuer_ca_bundle   = var.vault_ca_bundle
  certmanager_vault_issuer_policy_name = "{{ .policy_name }}"
}

resource "kubernetes_secret_v1" "vault_pki_ca" {
  metadata {
    name      = "vault-pki-ca"
    namespace = "cert-manager"
  }
  data = {
    "ca.crt" = base64decode(var.vault_ca_bundle)
  }
}
`

// vaultEnv is the schema of the SOPS-encrypted KV-YAML the caller provides.
// Only `vault_addr` and `vault_token` are mandatory; `vault_skip_verify`
// defaults to true (matches existing usage in stuttgart-things/clusters/...).
type vaultEnv struct {
	VaultAddr       string `yaml:"vault_addr"`
	VaultToken      string `yaml:"vault_token"`
	VaultSkipVerify *bool  `yaml:"vault_skip_verify,omitempty"`
}

// CreateVaultIssuer bootstraps a cert-manager `vault-pki` ClusterIssuer on a
// target cluster by templating + applying the vault-base-setup Terraform
// module against the cluster's kubeconfig. The Terraform source is inlined
// in this module — no checkout of the consumer repo is required. State
// lives as a kubernetes Secret on the target cluster
// (`tfstate-default-vault-<cluster-name>` in `kube-system`). Fire-and-forget:
// the function returns the terraform apply stdout/stderr but persists no
// artefacts in the caller's filesystem.
//
// The CA bundle is fetched live from `${vault_addr}/v1/pki/ca/pem` rather
// than carried in the env file, to avoid stale CA material.
//
// Refs blueprints#162.
func (m *Argocd) CreateVaultIssuer(
	ctx context.Context,
	// Target cluster name; used as `var.cluster_name` and as the tfstate
	// secret_suffix (`vault-<cluster-name>`).
	clusterName string,
	// SOPS-encrypted kubeconfig of the target cluster.
	kubeconfigSourceFile *dagger.File,
	// SOPS-encrypted KV-YAML with `vault_addr`, `vault_token`,
	// `vault_skip_verify`.
	vaultEnvFile *dagger.File,
	// AGE private key for decrypting both files.
	sopsKey *dagger.Secret,
	// Vault PKI role name (e.g. "sthings-vsphere") to use for the issuer.
	// +optional
	// +default="sthings-vsphere"
	pkiRole string,
	// Vault policy granting the issuer permission to issue certs.
	// +optional
	// +default="pki-issue"
	policyName string,
	// Install cert-manager CRDs + ensure the `cert-manager` namespace exists
	// on the target cluster before running Terraform. The TF creates a
	// ClusterIssuer (cert-manager CRD) and a Secret in the `cert-manager`
	// namespace; both fail if cert-manager hasn't been installed yet, and
	// even with cert-manager being installed concurrently by AppSets there
	// is a race window before the validating webhook is registered.
	// Pre-applying just the CRDs (not the webhook config) makes the
	// resource type known without engaging the admission webhook, so
	// terraform succeeds regardless of whether cert-manager pods are Ready.
	// Idempotent (server-side apply); safe to leave enabled even when the
	// cert-manager-install AppSet is also active.
	// +optional
	// +default=true
	installCertManagerCrds bool,
	// cert-manager version whose upstream CRDs YAML to apply when
	// `install-cert-manager-crds=true`. Match the version installed by the
	// `cert-manager-install` AppSet to avoid schema drift.
	// +optional
	// +default="v1.19.2"
	certManagerVersion string,
) (*dagger.Directory, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster-name is required")
	}
	if kubeconfigSourceFile == nil {
		return nil, fmt.Errorf("kubeconfig-source-file is required")
	}
	if vaultEnvFile == nil {
		return nil, fmt.Errorf("vault-env-file is required")
	}
	if sopsKey == nil {
		return nil, fmt.Errorf("sops-key is required")
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

	// Decrypt the kubeconfig and turn it into a Secret so the Terraform
	// container can mount it at /root/.kube/config (matching the inlined
	// backend's config_path).
	kubeconfigYaml, err := dag.Secrets().Decrypt(ctx, sopsKey, kubeconfigSourceFile)
	if err != nil {
		return nil, fmt.Errorf("decrypt kubeconfig: %w", err)
	}
	kubeconfigSecret := dag.SetSecret("vault-issuer-kubeconfig", kubeconfigYaml)
	const kubeconfigPath = "/root/.kube/config"

	// Pre-install the bare-minimum cert-manager prerequisites so the TF
	// resources (ClusterIssuer, Secret in cert-manager namespace) are
	// guaranteed to apply cleanly — independent of any concurrent
	// AppSet-driven cert-manager install.
	if installCertManagerCrds {
		if err := m.installCertManagerPrereqs(ctx, kubeconfigSecret, certManagerVersion); err != nil {
			return nil, fmt.Errorf("install cert-manager prerequisites: %w", err)
		}
	}

	// Live-fetch the Vault PKI CA bundle (base64-encoded) so the issuer
	// always carries the current CA. We do this in a transient curl
	// container rather than over the host network.
	caBundleB64, err := m.fetchVaultCABundleB64(ctx, env.VaultAddr, env.VaultToken, skipVerify)
	if err != nil {
		return nil, fmt.Errorf("fetch vault CA bundle: %w", err)
	}

	// Render the inlined Terraform via dagger/templating.
	tfVars, err := json.Marshal(map[string]any{
		"secret_suffix":   "vault-" + clusterName,
		"kubeconfig_path": kubeconfigPath,
		"pki_role":        pkiRole,
		"policy_name":     policyName,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal template vars: %w", err)
	}
	mainTf, err := dag.Templating().RenderInline(ctx, vaultIssuerTfTemplate,
		dagger.TemplatingRenderInlineOpts{Variables: string(tfVars)})
	if err != nil {
		return nil, fmt.Errorf("render terraform template: %w", err)
	}

	// Render the JSON tfvars supplying the runtime values (cluster_name,
	// kubeconfig_path inside the container, vault_addr, vault_ca_bundle).
	tfvarsJSON, err := json.Marshal(map[string]any{
		"cluster_name":    clusterName,
		"kubeconfig_path": kubeconfigPath,
		"vault_addr":      env.VaultAddr,
		"vault_ca_bundle": caBundleB64,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal tfvars: %w", err)
	}
	tfvarsSecret := dag.SetSecret("vault-issuer-tfvars", string(tfvarsJSON))

	// Build a Directory with the rendered main.tf and execute terraform
	// init+apply via the dagger/terraform module.
	tfDir := dag.Directory().WithNewFile("main.tf", mainTf)
	vaultTokenSecret := dag.SetSecret("vault-issuer-token", env.VaultToken)

	out := dag.Terraform().Execute(
		tfDir,
		dagger.TerraformExecuteOpts{
			Operation:           "apply",
			SecretJSONVariables: tfvarsSecret,
			VaultToken:          vaultTokenSecret,
			VaultAddr:           env.VaultAddr,
			KubeConfig:          kubeconfigSecret,
			KubeConfigPath:      kubeconfigPath,
		},
	)
	if _, err := out.Sync(ctx); err != nil {
		return nil, fmt.Errorf("terraform apply: %w", err)
	}
	return out, nil
}

// installCertManagerPrereqs ensures the `cert-manager` namespace exists and
// the cert-manager CRDs are registered on the target cluster, so the
// Terraform-managed ClusterIssuer + Secret can apply without racing the
// validating webhook. Both ops are server-side-applied and therefore
// idempotent — re-running, or running concurrently with the
// `cert-manager-install` AppSet, is safe.
//
// We deliberately do NOT install the cert-manager controller / webhook /
// `ValidatingWebhookConfiguration` here: only the CRDs. With CRDs present
// but no webhook registered, the API server admits the ClusterIssuer
// without invoking a (potentially-not-yet-serving) webhook pod. Once the
// AppSet's full install lands, the webhook takes over for subsequent
// changes and the ClusterIssuer becomes Ready.
func (m *Argocd) installCertManagerPrereqs(
	ctx context.Context,
	kubeconfigSecret *dagger.Secret,
	certManagerVersion string,
) error {
	if certManagerVersion == "" {
		certManagerVersion = "v1.19.2"
	}

	// 1. Ensure the cert-manager namespace exists. Tiny inline manifest;
	//    server-side apply means no conflict with the AppSet's later
	//    `CreateNamespace=true` install.
	nsManifest := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"
	nsFile := dag.Directory().WithNewFile("ns.yaml", nsManifest).File("ns.yaml")
	if _, err := dag.Kubernetes().Kubectl(ctx, dagger.KubernetesKubectlOpts{
		Operation:  "apply",
		SourceFile: nsFile,
		KubeConfig: kubeconfigSecret,
		ServerSide: true,
	}); err != nil {
		return fmt.Errorf("apply cert-manager namespace: %w", err)
	}

	// 2. Apply just the CRDs from the upstream cert-manager release via
	//    the existing kubernetes-deployment.InstallCustomResourceDefinitions
	//    function — same kubectl-apply primitive, but reused so consumers
	//    have one canonical entry point for "apply CRDs from a URL". URL
	//    pattern is stable across versions; using the chart's pinned
	//    version avoids schema drift between the AppSet's helm install
	//    and our pre-install.
	crdsURL := "https://github.com/cert-manager/cert-manager/releases/download/" +
		certManagerVersion + "/cert-manager.crds.yaml"
	if _, err := dag.KubernetesDeployment().InstallCustomResourceDefinitions(ctx,
		dagger.KubernetesDeploymentInstallCustomResourceDefinitionsOpts{
			SourceUrls: crdsURL,
			Operation:  "apply",
			ServerSide: true,
			KubeConfig: kubeconfigSecret,
		}); err != nil {
		return fmt.Errorf("apply cert-manager CRDs (%s): %w", crdsURL, err)
	}

	return nil
}

// fetchVaultCABundleB64 returns the Vault PKI root CA in base64-encoded PEM
// form, suitable for `var.vault_ca_bundle` in the rendered Terraform.
func (m *Argocd) fetchVaultCABundleB64(
	ctx context.Context,
	vaultAddr, vaultToken string,
	skipVerify bool,
) (string, error) {
	curlArgs := []string{"curl", "-fsS"}
	if skipVerify {
		curlArgs = append(curlArgs, "-k")
	}
	curlArgs = append(curlArgs, "-H", "X-Vault-Token: "+vaultToken,
		strings.TrimRight(vaultAddr, "/")+"/v1/pki/ca/pem")

	cmd := strings.Join(curlArgs, " ") + " | base64 -w0"
	stdout, err := dag.Container().
		From("alpine:3.21").
		WithExec([]string{"apk", "add", "--no-cache", "curl"}).
		WithExec([]string{"sh", "-c", cmd}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("curl vault ca/pem: %w", err)
	}
	bundle := strings.TrimSpace(stdout)
	if bundle == "" {
		return "", fmt.Errorf("vault returned empty CA bundle")
	}
	return bundle, nil
}

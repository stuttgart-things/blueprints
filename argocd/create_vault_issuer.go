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
	// Wait for cert-manager (CRDs + webhook Deployment) to be Ready on the
	// target cluster before running Terraform. The TF creates a ClusterIssuer
	// (cert-manager CRD) and a Secret in the `cert-manager` namespace; both
	// fail if cert-manager isn't installed yet. Disable only if you're sure
	// cert-manager is already up and want to skip the ~10s probe.
	// +optional
	// +default=true
	waitForCertManager bool,
	// Maximum time to wait for cert-manager-webhook Deployment to be
	// Available. Forwarded to `kubectl wait --timeout=`.
	// +optional
	// +default="5m"
	certManagerWaitTimeout string,
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

	// Wait for cert-manager to be ready before running Terraform — the TF
	// creates a ClusterIssuer (cert-manager CRD) and a Secret in the
	// cert-manager namespace, both of which fail without cert-manager
	// installed and its admission webhook Available.
	if waitForCertManager {
		if err := m.waitForCertManager(ctx, kubeconfigYaml, certManagerWaitTimeout); err != nil {
			return nil, fmt.Errorf("wait for cert-manager: %w", err)
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

// waitForCertManager blocks until the cert-manager admission webhook is
// Available and the ClusterIssuer CRD is registered on the target cluster.
// Without this, Terraform's kubernetes provider fails fast on a missing
// CRD or, worse, succeeds the plan and times out at apply when the webhook
// isn't yet serving.
func (m *Argocd) waitForCertManager(
	ctx context.Context,
	kubeconfigYaml, timeout string,
) error {
	if timeout == "" {
		timeout = "5m"
	}
	kubeconfigSecret := dag.SetSecret("vault-issuer-cm-wait-kubeconfig", kubeconfigYaml)
	script := strings.Join([]string{
		// CRD must exist (silent if not — kubectl get crd exits 1).
		"kubectl get crd clusterissuers.cert-manager.io >/dev/null",
		// Webhook Deployment must be Available; covers the validating webhook
		// that gates ClusterIssuer admission.
		"kubectl -n cert-manager wait deployment/cert-manager-webhook " +
			"--for=condition=Available --timeout=" + timeout,
	}, " && ")
	_, err := dag.Container().
		From("bitnami/kubectl:1.31").
		WithMountedSecret("/.kube/config", kubeconfigSecret).
		WithEnvVariable("KUBECONFIG", "/.kube/config").
		WithExec([]string{"sh", "-c", script}).
		Sync(ctx)
	if err != nil {
		return fmt.Errorf("cert-manager not ready (CRD missing or webhook not Available within %s): %w", timeout, err)
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

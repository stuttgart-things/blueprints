package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/argocd/internal/dagger"
)

// RenderClusterbookClusterConfig renders the clusterbook-cluster-gen KCL module,
// which produces the manifests needed to register a cluster with Argo CD
// (kubeconfig Secret reference, provider config, cluster labels, optional DNS).
//
// Values can come from a YAML/JSON file (--values-file) and/or individual CLI
// flags. When both are provided, CLI flags override matching keys in the file
// (KCL --parameters takes precedence over --parametersFile).
//
// Returns the rendered multi-document YAML as a Dagger File.
func (m *Argocd) RenderClusterbookClusterConfig(
	ctx context.Context,
	// Cluster name (KCL -D name) — required unless valuesFile is provided
	// +optional
	name string,
	// /24 network key, e.g. 10.31.101 — required unless valuesFile is provided
	// +optional
	networkKey string,
	// YAML/JSON file with KCL parameters (passed as --parametersFile);
	// CLI flags below override matching keys
	// +optional
	valuesFile *dagger.File,
	// OCI KCL module source
	// +optional
	// +default="ghcr.io/stuttgart-things/clusterbook-cluster-gen:0.1.0"
	ociSource string,
	// Argo CD-side cluster name; defaults to name when neither is set via file
	// +optional
	clusterName string,
	// Create a DNS record for the cluster (default true when no values file)
	// +optional
	createDNS *bool,
	// Preserve the existing server field from the kubeconfig Secret (default true when no values file)
	// +optional
	preserveKubeconfigServer *bool,
	// Release the Argo CD cluster Secret on resource delete (default true when no values file)
	// +optional
	releaseOnDelete *bool,
	// Name of the Secret holding the cluster kubeconfig; defaults to name when no values file
	// +optional
	kubeconfigSecretName string,
	// Namespace of the kubeconfig Secret (default "argocd" when no values file)
	// +optional
	kubeconfigSecretNamespace string,
	// Argo CD namespace (default "argocd" when no values file)
	// +optional
	argocdNamespace string,
	// Crossplane ProviderConfig name (default "default" when no values file)
	// +optional
	providerConfigName string,
	// Cluster labels as a JSON object literal, e.g. {"env":"lab","role":"mgmt"}
	// +optional
	clusterLabels string,
	// KCL entrypoint file
	// +optional
	// +default="main.k"
	entrypoint string,
) (*dagger.File, error) {
	if valuesFile == nil {
		if name == "" {
			return nil, fmt.Errorf("name is required (or pass --values-file)")
		}
		if networkKey == "" {
			return nil, fmt.Errorf("networkKey is required (or pass --values-file)")
		}
		if clusterName == "" {
			clusterName = name
		}
		if kubeconfigSecretName == "" { // pragma: allowlist secret
			kubeconfigSecretName = name // pragma: allowlist secret
		}
		if kubeconfigSecretNamespace == "" {
			kubeconfigSecretNamespace = "argocd" // pragma: allowlist secret
		}
		if argocdNamespace == "" {
			argocdNamespace = "argocd"
		}
		if providerConfigName == "" {
			providerConfigName = "default"
		}
		trueVal := true
		if createDNS == nil {
			createDNS = &trueVal
		}
		if preserveKubeconfigServer == nil {
			preserveKubeconfigServer = &trueVal
		}
		if releaseOnDelete == nil {
			releaseOnDelete = &trueVal
		}
	}

	var params []string
	addStr := func(k, v string) {
		if v != "" {
			params = append(params, k+"="+v)
		}
	}
	addBool := func(k string, v *bool) {
		if v != nil {
			params = append(params, fmt.Sprintf("%s=%t", k, *v))
		}
	}

	addStr("name", name)
	addStr("networkKey", networkKey)
	addStr("clusterName", clusterName)
	addBool("createDNS", createDNS)
	addBool("preserveKubeconfigServer", preserveKubeconfigServer)
	addBool("releaseOnDelete", releaseOnDelete)
	addStr("kubeconfigSecretName", kubeconfigSecretName)
	addStr("kubeconfigSecretNamespace", kubeconfigSecretNamespace)
	addStr("argocdNamespace", argocdNamespace)
	addStr("providerConfigName", providerConfigName)
	addStr("clusterLabels", clusterLabels)

	opts := dagger.KclRunOpts{
		OciSource:  ociSource,
		Entrypoint: entrypoint,
	}
	if len(params) > 0 {
		opts.Parameters = strings.Join(params, ",")
	}
	if valuesFile != nil {
		opts.ParametersFile = valuesFile
	}

	return dag.Kcl().Run(opts), nil
}

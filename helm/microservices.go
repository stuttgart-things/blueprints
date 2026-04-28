package main

import (
	"context"
	"strings"

	"dagger/helm/internal/dagger"
)

// DeployMicroservices runs a sequence of Helmfile operations from a
// comma-separated list of helmfile references against the same cluster.
func (m *Helm) DeployMicroservices(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	// +optional
	// +default="helmfile.yaml"
	helmfileRefs string,
	// +optional
	// +default="apply"
	operation string,
	// +optional
	registrySecret *dagger.Secret,
	// +optional
	kubeConfig *dagger.Secret,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// +optional
	secretPathKubeconfig string,
	// +optional
	// +default="approle"
	vaultAuthMethod string,
	// Comma-separated key=value pairs for --state-values-set
	// (e.g., "issuerName=cluster-issuer-approle,domain=demo.example.com")
	// +optional
	stateValues string,
) error {

	refs := strings.Split(helmfileRefs, ",")

	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}

		if err := m.DeployHelmfile(
			ctx,
			src,
			ref,
			operation,
			registrySecret,
			kubeConfig,
			vaultAppRoleID,
			vaultSecretID,
			vaultURL,
			secretPathKubeconfig,
			vaultAuthMethod,
			stateValues,
		); err != nil {
			return err
		}
	}

	return nil
}

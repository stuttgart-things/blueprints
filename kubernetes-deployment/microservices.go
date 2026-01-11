package main

import (
	"context"
	"dagger/kubernetes-deployment/internal/dagger"
	"strings"
)

func (m *KubernetesDeployment) DeployMicroservices(
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

	// Parse comma-separated helmfile references
	refs := strings.Split(helmfileRefs, ",")

	// Deploy each helmfile
	for _, ref := range refs {
		// Trim whitespace from each reference
		ref = strings.TrimSpace(ref)

		// Skip empty references
		if ref == "" {
			continue
		}

		// Deploy this helmfile
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

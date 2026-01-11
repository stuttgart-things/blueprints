package main

import (
	"context"

	"dagger/kubernetes-deployment/internal/dagger"
)

func (m *KubernetesDeployment) DeployHelmfile(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	// +optional
	// +default="helmfile.yaml"
	HelmfileRef string,
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

	return dag.Helm().HelmfileOperation(
		ctx,
		dagger.HelmHelmfileOperationOpts{
			Src:                  src,
			HelmfileRef:          HelmfileRef,
			Operation:            operation,
			RegistrySecret:       registrySecret,
			KubeConfig:           kubeConfig,
			StateValues:          stateValues,
			VaultAppRoleID:       vaultAppRoleID,
			VaultSecretID:        vaultSecretID,
			VaultURL:             vaultURL,
			SecretPathKubeconfig: secretPathKubeconfig,
			VaultAuthMethod:      vaultAuthMethod,
		},
	)
}

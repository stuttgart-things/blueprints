package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
)

func (m *KubernetesMicroservice) DeployHelmfile(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	// +default="./"
	pathHelmfile string,
	// +optional
	// +default="helmfile.yaml"
	helmfileName string,
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
) error {

	return dag.Helm().HelmfileOperation(
		ctx,
		src,
		dagger.HelmHelmfileOperationOpts{
			PathHelmfile:         pathHelmfile,
			HelmfileName:         helmfileName,
			Operation:            operation,
			RegistrySecret:       registrySecret,
			KubeConfig:           kubeConfig,
			VaultAppRoleID:       vaultAppRoleID,
			VaultSecretID:        vaultSecretID,
			VaultURL:             vaultURL,
			SecretPathKubeconfig: secretPathKubeconfig,
			VaultAuthMethod:      vaultAuthMethod,
		},
	)
}

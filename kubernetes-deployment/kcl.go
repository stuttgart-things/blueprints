package main

import (
	"context"
	"dagger/kubernetes-deployment/internal/dagger"
)

// DeployKcl renders KCL manifests and applies them to a Kubernetes cluster in one step.
func (m *KubernetesDeployment) DeployKcl(
	ctx context.Context,
	// Local KCL source directory
	// +optional
	source *dagger.Directory,
	// OCI source path (e.g., oci://ghcr.io/stuttgart-things/kcl-module)
	// +optional
	ociSource string,
	// KCL parameters as comma-separated key=value pairs
	// +optional
	parameters string,
	// YAML/JSON file containing KCL parameters
	// +optional
	parametersFile *dagger.File,
	// Kubeconfig for cluster access
	kubeConfig *dagger.Secret,
	// Target namespace
	// +optional
	// +default="default"
	namespace string,
	// kubectl operation (apply or delete)
	// +optional
	// +default="apply"
	operation string,
) (string, error) {

	// Render KCL manifests
	rendered := dag.Kcl().Run(dagger.KclRunOpts{
		Source:         source,
		OciSource:     ociSource,
		Parameters:    parameters,
		ParametersFile: parametersFile,
	})

	// Apply rendered manifests to cluster
	return dag.Kubernetes().Kubectl(
		ctx,
		dagger.KubernetesKubectlOpts{
			Operation:  operation,
			SourceFile: rendered,
			KubeConfig: kubeConfig,
			Namespace:  namespace,
		},
	)
}

package main

import (
	"context"
	"dagger/kubernetes-deployment/internal/dagger"
	"strings"
)

func (m *KubernetesDeployment) ApplyManifests(
	ctx context.Context,
	// +optional
	// +default="*.yaml"
	manifestPattern string,
	// +optional
	sourceFile string,
	// +optional
	// +default=""
	sourceURL string,
	// +optional
	// +default="apply"
	operation string,
	// +optional
	kubeConfig *dagger.Secret,
	// +optional
	// +default="default"
	namespace string,
) (string, error) {
	var results []string

	// Parse sourceFiles (comma-separated)
	files := strings.Split(sourceFile, ",")
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		// Convert file path to dagger.File
		result, err := dag.Kubernetes().Kubectl(
			ctx,
			dagger.KubernetesKubectlOpts{
				Operation:  operation,
				SourceFile: dag.CurrentModule().Source().File(file),
				KubeConfig: kubeConfig,
				Namespace:  namespace,
			},
		)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}

	// Parse sourceURLs (comma-separated)
	urls := strings.Split(sourceURL, ",")
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		result, err := dag.Kubernetes().Kubectl(
			ctx,
			dagger.KubernetesKubectlOpts{
				Operation:  operation,
				URLSource:  url,
				KubeConfig: kubeConfig,
				Namespace:  namespace,
			},
		)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}

	return strings.Join(results, "\n---\n"), nil
}

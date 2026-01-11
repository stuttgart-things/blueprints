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
	sourceFiles string,
	// +optional
	// +default=""
	sourceURLs string,
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
	files := strings.Split(sourceFiles, ",")
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
	urls := strings.Split(sourceURLs, ",")
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

func (m *KubernetesDeployment) InstallCustomResourceDefinitions(
	ctx context.Context,
	// +optional
	kustomizeSources string,
	// +optional
	// +default=""
	sourceURLs string,
	// +optional
	// +default="apply"
	operation string,
	// Use server-side apply (only valid with apply operation)
	// +optional
	// +default=true
	serverSide bool,
	// +optional
	kubeConfig *dagger.Secret,
) (string, error) {
	var results []string

	// Parse kustomizeSources (comma-separated)
	kustomizes := strings.Split(kustomizeSources, ",")
	for _, kustomize := range kustomizes {
		kustomize = strings.TrimSpace(kustomize)
		if kustomize == "" {
			continue
		}
		result, err := dag.Kubernetes().Kubectl(
			ctx,
			dagger.KubernetesKubectlOpts{
				Operation:       operation,
				KustomizeSource: kustomize,
				KubeConfig:      kubeConfig,
				ServerSide:      serverSide,
			},
		)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}

	// Parse sourceURLs (comma-separated)
	urls := strings.Split(sourceURLs, ",")
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
				ServerSide: serverSide,
			},
		)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}

	return strings.Join(results, "\n---\n"), nil
}

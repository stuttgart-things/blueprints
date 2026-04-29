package main

import (
	"context"
	"fmt"

	"dagger/argocd/internal/dagger"
)

// ApplyConfig applies a rendered config file to the cluster. The target
// namespace is created (server-side, kubectl apply on a Namespace doc)
// before the manifests are applied.
func (m *Argocd) ApplyConfig(
	ctx context.Context,
	// Rendered config file (from render-clusterbook-cluster-config or local YAML)
	configFile *dagger.File,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
	// Target namespace
	// +optional
	// +default="argocd"
	namespace string,
) (string, error) {
	if configFile == nil {
		return "", fmt.Errorf("config-file is required")
	}

	rendered, err := configFile.Contents(ctx)
	if err != nil {
		return "", fmt.Errorf("apply-config: read file: %w", err)
	}

	nsDoc := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s`, namespace)

	combined := dag.Directory().
		WithNewFile("config.yaml", nsDoc+"\n---\n"+rendered).
		File("config.yaml")

	if _, err := dag.Kubernetes().Kubectl(
		ctx,
		dagger.KubernetesKubectlOpts{
			Operation:  "apply",
			SourceFile: combined,
			KubeConfig: kubeConfig,
		},
	); err != nil {
		return "", fmt.Errorf("apply-config: %w", err)
	}

	return fmt.Sprintf("Config applied to cluster (namespace=%s)", namespace), nil
}

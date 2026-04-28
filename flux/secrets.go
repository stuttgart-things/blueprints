package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/flux/internal/dagger"
)

// ApplySecrets applies secret manifests to the cluster.
func (m *Flux) ApplySecrets(
	ctx context.Context,
	// Secret YAML content
	secretContent string,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
) (string, error) {
	secretFile := dag.Directory().
		WithNewFile("secrets.yaml", secretContent).
		File("secrets.yaml")

	_, err := dag.Kubernetes().Kubectl(
		ctx,
		dagger.KubernetesKubectlOpts{
			Operation:  "apply",
			SourceFile: secretFile,
			Namespace:  namespace,
			KubeConfig: kubeConfig,
		},
	)
	if err != nil {
		return "", fmt.Errorf("apply-secrets: %w", err)
	}

	return "Secrets applied to cluster", nil
}

// VerifySecrets auto-extracts secret names from the YAML and verifies they
// exist in the cluster.
func (m *Flux) VerifySecrets(
	ctx context.Context,
	// Secret YAML content (multi-document)
	secretContent string,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
) (string, error) {
	docs := strings.Split(secretContent, "---")
	var secretNames []string
	for _, doc := range docs {
		if !strings.Contains(doc, "kind: Secret") {
			continue
		}
		inMetadata := false
		for _, line := range strings.Split(doc, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "metadata:" {
				inMetadata = true
				continue
			}
			if inMetadata && strings.HasPrefix(trimmed, "name:") {
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				name = strings.Trim(name, "\"'")
				if name != "" {
					secretNames = append(secretNames, name)
				}
				inMetadata = false
				break
			}
			if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
				inMetadata = false
			}
		}
	}

	if len(secretNames) == 0 {
		return "No secret names found in YAML", nil
	}

	var found, missing []string
	for _, name := range secretNames {
		_, err := dag.Container().
			From("bitnami/kubectl:latest").
			WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
				Mode: 0444,
			}).
			WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig").
			WithExec([]string{"kubectl", "get", "secret", name, "-n", namespace, "-o", "name"}).
			Stdout(ctx)
		if err != nil {
			missing = append(missing, name)
		} else {
			found = append(found, name)
		}
	}

	var result []string
	if len(found) > 0 {
		result = append(result, fmt.Sprintf("Found secrets: %s", strings.Join(found, ", ")))
	}
	if len(missing) > 0 {
		result = append(result, fmt.Sprintf("Missing secrets: %s", strings.Join(missing, ", ")))
		return strings.Join(result, "\n"), fmt.Errorf("verify-secrets: %d secret(s) missing: %s", len(missing), strings.Join(missing, ", "))
	}

	return strings.Join(result, "\n"), nil
}

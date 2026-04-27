package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/flux/internal/dagger"
)

// FluxDeployOperator deploys the Flux operator via Helmfile.
func (m *Flux) FluxDeployOperator(
	ctx context.Context,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
	// Helmfile reference
	// +optional
	// +default="helmfile.yaml"
	helmfileRef string,
	// Directory containing the helmfile
	// +optional
	src *dagger.Directory,
	// Comma-separated key=value pairs for --state-values-set
	// (e.g., "version=0.42.1")
	// +optional
	stateValues string,
) error {
	return dag.Helm().HelmfileOperation(
		ctx,
		dagger.HelmHelmfileOperationOpts{
			Src:             src,
			HelmfileRef:     helmfileRef,
			Operation:       "apply",
			KubeConfig:      kubeConfig,
			StateValues:     stateValues,
			VaultAuthMethod: "approle",
		},
	)
}

// FluxApplyConfig applies rendered config (non-secret) manifests to the cluster.
func (m *Flux) FluxApplyConfig(
	ctx context.Context,
	// Config YAML content
	configContent string,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
) (string, error) {
	nsDoc := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s`, namespace)

	fullContent := nsDoc + "\n---\n" + configContent

	configFile := dag.Directory().
		WithNewFile("config.yaml", fullContent).
		File("config.yaml")

	_, err := dag.Kubernetes().Kubectl(
		ctx,
		dagger.KubernetesKubectlOpts{
			Operation:  "apply",
			SourceFile: configFile,
			KubeConfig: kubeConfig,
		},
	)
	if err != nil {
		return "", fmt.Errorf("flux-apply-config: %w", err)
	}

	return "Config applied to cluster", nil
}

// FluxWaitForReconciliation runs flux check with retry, reconciles sources,
// and gets all Flux resources.
func (m *Flux) FluxWaitForReconciliation(
	ctx context.Context,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
	// Timeout for reconciliation check
	// +optional
	// +default="5m"
	reconciliationTimeout string,
	// Flux CLI container image
	// +optional
	// +default="ghcr.io/fluxcd/flux-cli:v2.8.3"
	fluxCliImage string,
) (string, error) {
	timeoutSecs := parseTimeout(reconciliationTimeout)

	retryScript := fmt.Sprintf(`#!/bin/sh
echo "Waiting for Flux controllers to be deployed by the operator..."
INTERVAL=15
ELAPSED=0
TIMEOUT_SECS=%d
while [ $ELAPSED -lt $TIMEOUT_SECS ]; do
  if flux check 2>&1; then
    echo "Flux check passed after ${ELAPSED}s"
    exit 0
  fi
  echo "Flux not ready yet, retrying in ${INTERVAL}s (${ELAPSED}s/${TIMEOUT_SECS}s)..."
  sleep $INTERVAL
  ELAPSED=$((ELAPSED + INTERVAL))
done
echo "Timeout waiting for Flux reconciliation after ${TIMEOUT_SECS}s"
flux check
`, timeoutSecs)

	cli := fluxCliContainer(fluxCliImage, kubeConfig)

	var results []string

	checkOutput, err := cli.
		WithExec([]string{"sh", "-c", retryScript}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("flux-wait: flux check failed: %w", err)
	}
	results = append(results, fmt.Sprintf("Flux check passed:\n%s", checkOutput))

	reconcileOutput, err := cli.
		WithExec([]string{"flux", "reconcile", "source", "git", "flux-system", "-n", namespace}).
		Stdout(ctx)
	if err != nil {
		results = append(results, fmt.Sprintf("Warning - flux reconcile failed: %v", err))
	} else {
		results = append(results, fmt.Sprintf("Source reconciled:\n%s", reconcileOutput))
	}

	getAllOutput, err := cli.
		WithExec([]string{"flux", "get", "all", "-n", namespace}).
		Stdout(ctx)
	if err != nil {
		results = append(results, fmt.Sprintf("Warning - flux get all failed: %v", err))
	} else {
		results = append(results, fmt.Sprintf("Flux resources:\n%s", getAllOutput))
	}

	return strings.Join(results, "\n"), nil
}

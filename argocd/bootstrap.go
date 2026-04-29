package main

import (
	"context"
	"fmt"

	"dagger/argocd/internal/dagger"
)

// BootstrapClusterbookCluster orchestrates the full cluster-registration
// workflow: render the clusterbook config, optionally apply it to a cluster
// (--deploy), and optionally commit it to a Git repo with optional PR and
// merge (--commit-to-git).
//
// Returns the rendered file so callers can also `export --path=...` it.
func (m *Argocd) BootstrapClusterbookCluster(
	ctx context.Context,

	// --- Render params (forwarded to render-clusterbook-cluster-config) ---

	// Cluster name — required unless valuesFile is provided
	// +optional
	name string,
	// /24 network key — required unless valuesFile is provided
	// +optional
	networkKey string,
	// YAML/JSON values file (KCL --parametersFile)
	// +optional
	valuesFile *dagger.File,
	// OCI KCL module source
	// +optional
	// +default="ghcr.io/stuttgart-things/clusterbook-cluster-gen:0.1.0"
	ociSource string,
	// Argo CD-side cluster name
	// +optional
	clusterName string,
	// +optional
	createDNS *bool,
	// +optional
	preserveKubeconfigServer *bool,
	// +optional
	releaseOnDelete *bool,
	// +optional
	kubeconfigSecretName string,
	// +optional
	kubeconfigSecretNamespace string,
	// +optional
	argocdNamespace string,
	// +optional
	providerConfigName string,
	// JSON object literal, e.g. {"env":"lab"}
	// +optional
	clusterLabels string,
	// +optional
	// +default="main.k"
	entrypoint string,

	// --- Deploy step (optional) ---

	// Apply the rendered config to a cluster
	// +optional
	// +default=false
	deploy bool,
	// Kubeconfig secret — required when deploy=true
	// +optional
	kubeConfig *dagger.Secret,
	// Target namespace for apply
	// +optional
	// +default="argocd"
	deployNamespace string,

	// --- Commit step (optional) ---

	// Commit the rendered config to a Git repository
	// +optional
	// +default=false
	commitToGit bool,
	// Repository in "owner/repo" — required when commitToGit=true
	// +optional
	repository string,
	// GitHub token — required when commitToGit=true
	// +optional
	gitToken *dagger.Secret,
	// Branch to commit to
	// +optional
	// +default="main"
	branchName string,
	// Destination folder within the repository
	// +optional
	// +default="argocd/clusters/"
	destinationPath string,
	// File name to write under destinationPath
	// +optional
	// +default="cluster.yaml"
	fileName string,
	// Commit message
	// +optional
	// +default="Add Argo CD cluster registration"
	commitMessage string,
	// Open a PR from branchName into baseBranch
	// +optional
	// +default=false
	createPR bool,
	// PR base branch
	// +optional
	// +default="main"
	baseBranch string,
	// +optional
	prTitle string,
	// +optional
	prBody string,
	// Auto-merge the PR after creation
	// +optional
	// +default=false
	mergePR bool,
	// squash | merge | rebase
	// +optional
	// +default="squash"
	mergeMethod string,
) (*dagger.File, error) {
	rendered, err := m.RenderClusterbookClusterConfig(
		ctx,
		name, networkKey, valuesFile, ociSource, clusterName,
		createDNS, preserveKubeconfigServer, releaseOnDelete,
		kubeconfigSecretName, kubeconfigSecretNamespace,
		argocdNamespace, providerConfigName, clusterLabels, entrypoint,
	)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	if deploy {
		if kubeConfig == nil {
			return nil, fmt.Errorf("deploy=true requires --kube-config")
		}
		if _, err := m.ApplyConfig(ctx, rendered, kubeConfig, deployNamespace); err != nil {
			return nil, fmt.Errorf("deploy: %w", err)
		}
	}

	if commitToGit {
		if repository == "" {
			return nil, fmt.Errorf("commit-to-git=true requires --repository")
		}
		if gitToken == nil {
			return nil, fmt.Errorf("commit-to-git=true requires --git-token")
		}
		if _, err := m.CommitConfig(
			ctx, rendered, repository, gitToken,
			branchName, destinationPath, fileName, commitMessage,
			createPR, baseBranch, prTitle, prBody,
			mergePR, mergeMethod,
		); err != nil {
			return nil, fmt.Errorf("commit-to-git: %w", err)
		}
	}

	return rendered, nil
}

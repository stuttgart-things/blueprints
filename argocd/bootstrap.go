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
	// Kubeconfig secret — required when deploy=true or detect-network-key=true
	// +optional
	kubeConfig *dagger.Secret,
	// Target namespace for apply
	// +optional
	// +default="argocd"
	deployNamespace string,

	// --- Auto-detect network-key step (optional) ---

	// Run kubectl get nodes -o json against --kube-config and use the
	// dominant /24 InternalIP prefix as --network-key. Only fires when
	// --network-key is empty.
	// +optional
	// +default=false
	detectNetworkKey bool,

	// --- Render kubeconfig Secret step (optional) ---

	// Render a v1/Secret wrapping a SOPS-encrypted kubeconfig source file;
	// applied alongside the cluster config on --deploy=true and committed
	// alongside it on --commit-to-git=true.
	// +optional
	// +default=false
	renderKubeconfigSecret bool,
	// SOPS-encrypted kubeconfig source — required when render-kubeconfig-secret=true // pragma: allowlist secret
	// +optional
	kubeconfigSourceFile *dagger.File,
	// AGE private key for decrypting kubeconfig-source-file
	// +optional
	sopsKey *dagger.Secret,
	// AGE public key for re-encrypting the rendered Secret — required when
	// render-kubeconfig-secret=true and commit-to-git=true // pragma: allowlist secret
	// +optional
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml) used during re-encryption
	// +optional
	sopsConfigFile *dagger.File,
	// Data key under data: in the rendered Secret
	// +optional
	// +default="kubeconfig"
	kubeconfigDataKey string,
	// File name to use when committing the kubeconfig Secret (joined with destination-path)
	// +optional
	// +default="kubeconfig.yaml"
	kubeconfigFileName string,

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

	// --- Cache control ---

	// Arbitrary string mixed into the function-level cache key. Pass a
	// timestamp (e.g. `date +%s%N`) from CI to force a fresh execution
	// when the function's other args are deterministic but the side
	// effects (git push, kubectl apply) must replay. Inner CACHE_BUSTER
	// env vars on individual containers don't help here — Dagger short-
	// circuits the whole function on input-hash match before any
	// container in the body is instantiated.
	// +optional
	cacheBuster string,
) (*dagger.File, error) {
	_ = cacheBuster
	if detectNetworkKey && networkKey == "" {
		if kubeConfig == nil {
			return nil, fmt.Errorf("detect-network-key=true requires --kube-config")
		}
		detected, err := m.DetectNetworkKey(ctx, kubeConfig)
		if err != nil {
			return nil, fmt.Errorf("detect-network-key: %w", err)
		}
		networkKey = detected
	}

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

	// Force the render pipeline to actually execute (KCL run) before any
	// downstream git or apply step. Dagger evaluates *dagger.File lazily —
	// without this Sync, KCL only fires when CommitConfig reads the file,
	// which happens *after* CreateGithubBranch has already touched the
	// remote branch. A KCL failure at that point would leave the branch
	// in a partially-mutated state with the scaffolder's commits already
	// reset (see stuttgart-things/blueprints#156). Materializing here
	// makes render failures stop the pipeline cleanly.
	rendered, err = rendered.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	var (
		encryptedKubeconfigSecret *dagger.File
		plaintextKubeconfigSecret *dagger.File
	)
	if renderKubeconfigSecret {
		if kubeconfigSourceFile == nil {
			return nil, fmt.Errorf("render-kubeconfig-secret=true requires --kubeconfig-source-file") // pragma: allowlist secret
		}
		if sopsKey == nil {
			return nil, fmt.Errorf("render-kubeconfig-secret=true requires --sops-key") // pragma: allowlist secret
		}

		secretName := kubeconfigSecretName // pragma: allowlist secret
		if secretName == "" {
			secretName = name // pragma: allowlist secret
		}
		secretNamespace := kubeconfigSecretNamespace // pragma: allowlist secret
		if secretNamespace == "" {
			secretNamespace = "argocd" // pragma: allowlist secret
		}
		dataKey := kubeconfigDataKey
		if dataKey == "" {
			dataKey = "kubeconfig"
		}
		secretFileName := kubeconfigFileName // pragma: allowlist secret
		if secretFileName == "" {
			secretFileName = "kubeconfig.yaml" // pragma: allowlist secret
		}

		if commitToGit {
			if agePublicKey == nil {
				return nil, fmt.Errorf("render-kubeconfig-secret=true with commit-to-git=true requires --age-public-key") // pragma: allowlist secret
			}
			encryptedKubeconfigSecret, err = m.RenderKubeconfigSecret(
				ctx, kubeconfigSourceFile, sopsKey, secretName, secretNamespace,
				dataKey, true, agePublicKey, sopsConfigFile,
			)
			if err != nil {
				return nil, fmt.Errorf("render-kubeconfig-secret (encrypted): %w", err)
			}
		}
		if deploy {
			plaintextKubeconfigSecret, err = m.RenderKubeconfigSecret(
				ctx, kubeconfigSourceFile, sopsKey, secretName, secretNamespace,
				dataKey, false, nil, nil,
			)
			if err != nil {
				return nil, fmt.Errorf("render-kubeconfig-secret (plaintext): %w", err)
			}
		}

		// Stash for downstream commit step
		_ = secretFileName
	}

	if deploy {
		if kubeConfig == nil {
			return nil, fmt.Errorf("deploy=true requires --kube-config")
		}
		if _, err := m.ApplyConfig(ctx, rendered, kubeConfig, deployNamespace); err != nil {
			return nil, fmt.Errorf("deploy: %w", err)
		}
		if plaintextKubeconfigSecret != nil { // pragma: allowlist secret
			ns := kubeconfigSecretNamespace
			if ns == "" {
				ns = "argocd"
			}
			if _, err := m.ApplyConfig(ctx, plaintextKubeconfigSecret, kubeConfig, ns); err != nil {
				return nil, fmt.Errorf("deploy kubeconfig secret: %w", err)
			}
		}
	}

	if commitToGit {
		if repository == "" {
			return nil, fmt.Errorf("commit-to-git=true requires --repository")
		}
		if gitToken == nil {
			return nil, fmt.Errorf("commit-to-git=true requires --git-token")
		}

		// Bundle cluster.yaml and (optionally) kubeconfig.yaml into a single
		// directory and push them in ONE commit. Two back-to-back
		// dag.Git().AddFileToGithubBranch calls collide because the second
		// call's CloneGithub returns a Dagger-cached Directory that pre-dates
		// the first push, so `git push` is rejected as non-fast-forward
		// (`Updates were rejected because the remote contains work that you
		// do not have locally`). One commit avoids the race entirely and is
		// also the right shape for a Backstage-PR — both files appear in the
		// same diff.
		secretFileName := kubeconfigFileName // pragma: allowlist secret
		if secretFileName == "" {
			secretFileName = "kubeconfig.yaml" // pragma: allowlist secret
		}
		filesDir := dag.Directory().WithFile(fileName, rendered)
		if encryptedKubeconfigSecret != nil { // pragma: allowlist secret
			filesDir = filesDir.WithFile(secretFileName, encryptedKubeconfigSecret)
		}
		if _, err := m.CommitFiles(
			ctx, filesDir, repository, gitToken,
			branchName, destinationPath, commitMessage,
			createPR, baseBranch, prTitle, prBody,
			mergePR, mergeMethod,
		); err != nil {
			return nil, fmt.Errorf("commit-to-git: %w", err)
		}
	}

	return rendered, nil
}

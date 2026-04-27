package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/flux/internal/dagger"
)

// FluxDestroy tears down Flux from a cluster.
//
// Phase order:
//
//	0: Delete FluxInstance CR
//	1: Delete Flux secrets
//	2: Uninstall Flux operator (Helmfile destroy)
//	3: Delete flux-system namespace
//
// Usage:
//
//	dagger call flux-destroy --kube-config file:///tmp/kubeconfig
func (m *Flux) FluxDestroy(
	ctx context.Context,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Helmfile reference for Flux operator
	// +optional
	// +default="helmfile.yaml"
	helmfileRef string,
	// Directory containing the helmfile
	// +optional
	src *dagger.Directory,
	// Flux operator version for Helmfile state values
	// +optional
	// +default="0.42.1"
	operatorVersion string,
) (string, error) {

	var results []string
	kubectlImage := "bitnami/kubectl:latest"

	// =========================================================================
	// Phase 0: Delete FluxInstance CR
	// =========================================================================

	_, err := dag.Container().
		From(kubectlImage).
		WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig").
		WithExec([]string{"kubectl", "delete", "fluxinstance", "--all", "-n", namespace, "--ignore-not-found=true"}).
		Stdout(ctx)
	if err != nil {
		results = append(results, fmt.Sprintf("Phase 0: Warning — delete FluxInstance: %v", err))
	} else {
		results = append(results, "Phase 0: FluxInstance CRs deleted")
	}

	// =========================================================================
	// Phase 1: Delete Flux secrets
	// =========================================================================

	_, err = dag.Container().
		From(kubectlImage).
		WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig").
		WithExec([]string{"kubectl", "delete", "secret", "--all", "-n", namespace, "--ignore-not-found=true"}).
		Stdout(ctx)
	if err != nil {
		results = append(results, fmt.Sprintf("Phase 1: Warning — delete secrets: %v", err))
	} else {
		results = append(results, "Phase 1: Flux secrets deleted")
	}

	// =========================================================================
	// Phase 2: Uninstall Flux operator (Helmfile destroy)
	// =========================================================================

	err = dag.Helm().HelmfileOperation(
		ctx,
		dagger.HelmHelmfileOperationOpts{
			Src:             src,
			HelmfileRef:     helmfileRef,
			Operation:       "destroy",
			KubeConfig:      kubeConfig,
			StateValues:     "version=" + operatorVersion,
			VaultAuthMethod: "approle",
		},
	)
	if err != nil {
		results = append(results, fmt.Sprintf("Phase 2: Warning — helmfile destroy: %v", err))
	} else {
		results = append(results, "Phase 2: Flux operator uninstalled via Helmfile destroy")
	}

	// =========================================================================
	// Phase 3: Delete flux-system namespace
	// =========================================================================

	_, err = dag.Container().
		From(kubectlImage).
		WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig").
		WithExec([]string{"kubectl", "delete", "namespace", namespace, "--ignore-not-found=true", "--timeout=120s"}).
		Stdout(ctx)
	if err != nil {
		results = append(results, fmt.Sprintf("Phase 3: Warning — delete namespace: %v", err))
	} else {
		results = append(results, fmt.Sprintf("Phase 3: Namespace %s deleted", namespace))
	}

	return strings.Join(results, "\n"), nil
}

// FluxBootstrap orchestrates a full Flux bootstrap lifecycle.
//
// Phase order:
//
//	0: ValidateAgeKeyPair — fail fast on key mismatch
//	1: FluxRenderConfig — render all manifests
//	2: FluxEncryptSecrets — encrypt before committing
//	3: FluxCommitConfig — push to Git
//	4: FluxDeployOperator — install operator (Helmfile)
//	5: FluxApplyConfig — apply FluxInstance CR
//	6: FluxApplySecrets — apply AFTER operator is running
//	7: FluxVerifySecrets — confirm secrets exist
//	8: FluxWaitForReconciliation — wait for Flux to reconcile
func (m *Flux) FluxBootstrap(
	ctx context.Context,
	// OCI KCL module source for rendering Flux instance config
	// +optional
	// +default="ghcr.io/stuttgart-things/kcl-flux-instance:0.3.3"
	ociSource string,
	// Additional comma-separated key=value pairs for KCL parameters
	// +optional
	configParameters string,
	// Flux instance version
	// +optional
	// +default="2.8.5"
	fluxVersion string,
	// KCL entrypoint file name
	// +optional
	// +default="main.k"
	entrypoint string,
	// Whether KCL should also render Secret manifests
	// +optional
	// +default=false
	renderSecrets bool,
	// Git username for pull secret
	// +optional
	gitUsername *dagger.Secret,
	// GitHub token for git pull secret
	// +optional
	gitPassword *dagger.Secret,
	// AGE private key for SOPS decryption (applied to cluster)
	// +optional
	sopsAgeKey *dagger.Secret,
	// AGE public key for encrypting secrets before git commit
	// +optional
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
	// Target namespace for Flux
	// +optional
	// +default="flux-system"
	namespace string,
	// Repository in "owner/repo" format
	// +optional
	repository string,
	// Branch name for git operations
	// +optional
	// +default="main"
	branchName string,
	// Destination path within the repository
	// +optional
	// +default="clusters/"
	destinationPath string,
	// Git reference for Flux source (e.g., refs/heads/main)
	// +optional
	// +default="refs/heads/main"
	gitRef string,
	// GitHub token for git operations
	// +optional
	gitToken *dagger.Secret,
	// Helmfile reference
	// +optional
	// +default="helmfile.yaml"
	helmfileRef string,
	// Directory containing the helmfile
	// +optional
	src *dagger.Directory,
	// Apply rendered secrets to cluster
	// +optional
	// +default=true
	applySecrets bool,
	// Encrypt secrets with SOPS before git commit
	// +optional
	// +default=false
	encryptSecrets bool,
	// Commit rendered config to git
	// +optional
	// +default=false
	commitToGit bool,
	// Deploy Flux operator via Helmfile
	// +optional
	// +default=true
	deployOperator bool,
	// Wait for Flux reconciliation
	// +optional
	// +default=true
	waitForReconciliation bool,
	// Timeout for reconciliation check
	// +optional
	// +default="5m"
	reconciliationTimeout string,
	// Apply rendered config to cluster
	// +optional
	// +default=false
	applyConfig bool,
	// Flux CLI container image
	// +optional
	// +default="ghcr.io/fluxcd/flux-cli:v2.8.5"
	fluxCliImage string,
	// Flux operator version for Helmfile state values
	// +optional
	// +default="0.47.0"
	operatorVersion string,
) (string, error) {

	var results []string

	// =========================================================================
	// Phase 0: Validate AGE Key Pair
	// =========================================================================

	if sopsAgeKey != nil && agePublicKey != nil {
		msg, err := m.ValidateAgeKeyPair(ctx, sopsAgeKey, agePublicKey)
		if err != nil {
			return "", fmt.Errorf("phase 0: %w", err)
		}
		results = append(results, fmt.Sprintf("Phase 0: %s", msg))
	} else {
		results = append(results, "Phase 0: Skipped (sopsAgeKey or agePublicKey not provided)")
	}

	// =========================================================================
	// Phase 1: Render Flux Instance Config (KCL)
	// =========================================================================

	kclParams := "name=flux,namespace=" + namespace + ",version=" + fluxVersion
	if repository != "" {
		kclParams += ",gitUrl=https://github.com/" + repository
	}
	if destinationPath != "" {
		kclParams += ",gitPath=" + destinationPath
	}
	if gitRef != "" {
		kclParams += ",gitRef=" + gitRef
	}
	if configParameters != "" {
		kclParams += "," + configParameters
	}

	renderedContent, err := m.FluxRenderConfig(
		ctx, ociSource, kclParams, entrypoint, renderSecrets,
		gitUsername, gitPassword, sopsAgeKey,
	)
	if err != nil {
		return "", fmt.Errorf("phase 1: %w", err)
	}

	docs := strings.Split(renderedContent, "---\n")
	var secretDocs []string
	var configDocs []string

	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" {
			continue
		}
		if strings.Contains(doc, "kind: Secret") {
			secretDocs = append(secretDocs, doc)
		} else {
			configDocs = append(configDocs, doc)
		}
	}

	var paramKeys []string
	for _, p := range strings.Split(kclParams, ",") {
		if parts := strings.SplitN(p, "=", 2); len(parts) == 2 {
			paramKeys = append(paramKeys, parts[0])
		}
	}
	results = append(results, fmt.Sprintf("Phase 1: KCL parameter keys: %v — rendered %d config doc(s) and %d secret doc(s)", paramKeys, len(configDocs), len(secretDocs)))

	// =========================================================================
	// Phase 2: Encrypt Secrets with SOPS
	// =========================================================================

	secretContent := strings.Join(secretDocs, "---\n")
	var secretsForCommit string // pragma: allowlist secret

	if encryptSecrets && len(secretDocs) > 0 {
		if agePublicKey == nil {
			return "", fmt.Errorf("phase 2: encryptSecrets=true but agePublicKey is nil") // pragma: allowlist secret
		}

		encrypted, err := m.FluxEncryptSecrets(ctx, secretContent, agePublicKey, sopsConfig) // pragma: allowlist secret
		if err != nil {
			return "", fmt.Errorf("phase 2: %w", err)
		}
		secretsForCommit = encrypted // pragma: allowlist secret
		results = append(results, "Phase 2: Secrets encrypted with SOPS")
	} else if len(secretDocs) > 0 {
		secretsForCommit = secretContent // pragma: allowlist secret
		results = append(results, "Phase 2: Skipped encryption (encryptSecrets=false)")
	} else {
		results = append(results, "Phase 2: Skipped (no secrets to encrypt)")
	}

	// =========================================================================
	// Phase 3: Commit to Git
	// =========================================================================

	if commitToGit {
		if repository == "" {
			return "", fmt.Errorf("phase 3: commitToGit=true but repository is empty")
		}
		if gitToken == nil {
			return "", fmt.Errorf("phase 3: commitToGit=true but gitToken is nil")
		}

		configContent := strings.Join(configDocs, "---\n")
		msg, err := m.FluxCommitConfig(ctx, configContent, repository, branchName, destinationPath, gitToken, secretsForCommit)
		if err != nil {
			return "", fmt.Errorf("phase 3: %w", err)
		}
		results = append(results, fmt.Sprintf("Phase 3: %s", msg))
	} else {
		results = append(results, "Phase 3: Skipped (commitToGit=false)")
	}

	// =========================================================================
	// Phase 4: Deploy Flux Operator via Helmfile
	// =========================================================================

	if deployOperator {
		err := m.FluxDeployOperator(ctx, kubeConfig, helmfileRef, src, "version="+operatorVersion)
		if err != nil {
			return "", fmt.Errorf("phase 4: deploy flux operator: %w", err)
		}
		results = append(results, "Phase 4: Flux operator deployed via Helmfile")
	} else {
		results = append(results, "Phase 4: Skipped (deployOperator=false)")
	}

	// =========================================================================
	// Phase 5: Apply Rendered Config to Cluster
	// =========================================================================

	if applyConfig && len(configDocs) > 0 {
		configContent := strings.Join(configDocs, "---\n")
		msg, err := m.FluxApplyConfig(ctx, configContent, namespace, kubeConfig)
		if err != nil {
			return "", fmt.Errorf("phase 5: %w", err)
		}
		results = append(results, fmt.Sprintf("Phase 5: %s", msg))
	} else {
		results = append(results, "Phase 5: Skipped (applyConfig=false or no config docs)")
	}

	// =========================================================================
	// Phase 6: Apply Secrets to Cluster (AFTER operator is running)
	// =========================================================================

	if applySecrets && len(secretDocs) > 0 {
		msg, err := m.FluxApplySecrets(ctx, secretContent, namespace, kubeConfig)
		if err != nil {
			return "", fmt.Errorf("phase 6: %w", err)
		}
		results = append(results, fmt.Sprintf("Phase 6: %s", msg))
	} else {
		results = append(results, "Phase 6: Skipped (applySecrets=false or no secrets)")
	}

	// =========================================================================
	// Phase 7: Verify Secrets Exist
	// =========================================================================

	if applySecrets && len(secretDocs) > 0 {
		msg, err := m.FluxVerifySecrets(ctx, secretContent, namespace, kubeConfig)
		if err != nil {
			results = append(results, fmt.Sprintf("Phase 7: Warning — %v", err))
		} else {
			results = append(results, fmt.Sprintf("Phase 7: %s", msg))
		}
	} else {
		results = append(results, "Phase 7: Skipped (no secrets to verify)")
	}

	// =========================================================================
	// Phase 8: Wait for Reconciliation (Flux CLI)
	// =========================================================================

	if waitForReconciliation {
		msg, err := m.FluxWaitForReconciliation(ctx, namespace, kubeConfig, reconciliationTimeout, fluxCliImage)
		if err != nil {
			return "", fmt.Errorf("phase 8: %w", err)
		}
		results = append(results, fmt.Sprintf("Phase 8: %s", msg))
	} else {
		results = append(results, "Phase 8: Skipped (waitForReconciliation=false)")
	}

	return strings.Join(results, "\n"), nil
}

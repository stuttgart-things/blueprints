package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/kubernetes-deployment/internal/dagger"
)

// fluxCliContainer returns a container with the Flux CLI and kubeconfig mounted.
func fluxCliContainer(fluxCliImage string, kubeConfig *dagger.Secret) *dagger.Container {
	return dag.Container().
		From(fluxCliImage).
		WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig")
}

// parseTimeout parses a Go duration string (e.g. "5m", "300s") and returns the
// equivalent number of seconds. Falls back to 300 on error.
func parseTimeout(timeout string) int {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return 300
	}
	return int(d.Seconds())
}

// ValidateAgeKeyPair derives the public key from the given AGE private key
// and verifies it matches the provided public key. Fails fast on mismatch.
//
// Usage:
//
//	dagger call validate-age-key-pair --sops-age-key env:SOPS_AGE_KEY --age-public-key env:AGE_PUB
func (m *KubernetesDeployment) ValidateAgeKeyPair(
	ctx context.Context,
	// AGE private key
	sopsAgeKey *dagger.Secret,
	// AGE public key to validate against
	agePublicKey *dagger.Secret,
) (string, error) {
	pubKeyPlain, err := agePublicKey.Plaintext(ctx)
	if err != nil {
		return "", fmt.Errorf("validate-age-key-pair: read agePublicKey: %w", err)
	}
	pubKeyPlain = strings.TrimSpace(pubKeyPlain)

	derived, err := dag.Container().
		From("alpine:3.21").
		WithExec([]string{"apk", "add", "--no-cache", "age"}).
		WithMountedSecret("/tmp/age-key", sopsAgeKey, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithExec([]string{"sh", "-c", "age-keygen -y /tmp/age-key"}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("validate-age-key-pair: derive public key: %w", err)
	}
	derived = strings.TrimSpace(derived)

	if derived != pubKeyPlain {
		return "", fmt.Errorf("validate-age-key-pair: MISMATCH — derived public key %q does not match provided %q", derived, pubKeyPlain)
	}

	return fmt.Sprintf("AGE key pair valid: %s", derived), nil
}

// FluxRenderConfig renders the Flux instance configuration using a KCL module.
// Returns the full rendered YAML (multi-document).
func (m *KubernetesDeployment) FluxRenderConfig(
	ctx context.Context,
	// OCI KCL module source
	// +optional
	// +default="ghcr.io/stuttgart-things/kcl-flux-instance:0.3.3"
	ociSource string,
	// Comma-separated key=value pairs for KCL parameters
	configParameters string,
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
) (string, error) {
	params := configParameters

	if renderSecrets {
		params += ",renderSecrets=true" // pragma: allowlist secret

		if gitUsername != nil {
			val, err := gitUsername.Plaintext(ctx) // pragma: allowlist secret
			if err != nil {
				return "", fmt.Errorf("flux-render-config: read gitUsername: %w", err)
			}
			params += ",gitUsername=" + val
		}

		if gitPassword != nil { // pragma: allowlist secret
			val, err := gitPassword.Plaintext(ctx) // pragma: allowlist secret
			if err != nil {
				return "", fmt.Errorf("flux-render-config: read gitPassword: %w", err)
			}
			params += ",gitPassword=" + val
		}

		if sopsAgeKey != nil {
			val, err := sopsAgeKey.Plaintext(ctx)
			if err != nil {
				return "", fmt.Errorf("flux-render-config: read sopsAgeKey: %w", err)
			}
			params += ",sopsAgeKey=" + val
		}
	}

	renderedFile := dag.Kcl().Run(
		dagger.KclRunOpts{
			OciSource:  ociSource,
			Parameters: params,
			Entrypoint: entrypoint,
		})

	return renderedFile.Contents(ctx)
}

// FluxEncryptSecrets encrypts secret YAML content with SOPS using the given AGE public key.
func (m *KubernetesDeployment) FluxEncryptSecrets(
	ctx context.Context,
	// Plain-text secret YAML content
	secretContent string,
	// AGE public key for encryption
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
) (string, error) {
	plainSecretFile := dag.Directory().
		WithNewFile("secrets.yaml", secretContent).
		File("secrets.yaml")

	encryptedFile := dag.Sops().Encrypt(
		agePublicKey,
		plainSecretFile,
		dagger.SopsEncryptOpts{
			FileExtension: "yaml",
			SopsConfig:    sopsConfig,
		},
	)

	return encryptedFile.Contents(ctx)
}

// FluxCommitConfig commits rendered config and optional secrets to a Git repository.
func (m *KubernetesDeployment) FluxCommitConfig(
	ctx context.Context,
	// Config YAML content to commit
	configContent string,
	// Repository in "owner/repo" format
	repository string,
	// Branch name for git operations
	// +optional
	// +default="main"
	branchName string,
	// Destination path within the repository
	// +optional
	// +default="clusters/"
	destinationPath string,
	// GitHub token for git operations
	gitToken *dagger.Secret,
	// Optional secrets YAML content to include in the commit
	// +optional
	secretsContent string,
) (string, error) {
	commitDir := dag.Directory().
		WithNewFile("config.yaml", configContent)

	if secretsContent != "" {
		commitDir = commitDir.WithNewFile("secrets.yaml", secretsContent)
	}

	_, err := dag.Git().AddFolderToGithubBranch(
		ctx,
		repository,
		branchName,
		"Add rendered Flux instance config",
		gitToken,
		commitDir,
		destinationPath,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no changes to commit") {
			return fmt.Sprintf("No changes to commit (config already up-to-date in %s)", repository), nil
		}
		return "", fmt.Errorf("flux-commit-config: %w", err)
	}

	return fmt.Sprintf("Committed to %s branch %s at %s", repository, branchName, destinationPath), nil
}

// FluxDeployOperator deploys the Flux operator via Helmfile.
func (m *KubernetesDeployment) FluxDeployOperator(
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
) error {
	return m.DeployHelmfile(
		ctx,
		src,
		helmfileRef,
		"apply",
		nil,
		kubeConfig,
		nil,
		nil,
		nil,
		"",
		"approle",
		"",
	)
}

// FluxApplyConfig applies rendered config (non-secret) manifests to the cluster.
func (m *KubernetesDeployment) FluxApplyConfig(
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

// FluxApplySecrets applies secret manifests to the cluster.
func (m *KubernetesDeployment) FluxApplySecrets(
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
		return "", fmt.Errorf("flux-apply-secrets: %w", err)
	}

	return "Secrets applied to cluster", nil
}

// FluxVerifySecrets auto-extracts secret names from the YAML and verifies they
// exist in the cluster.
func (m *KubernetesDeployment) FluxVerifySecrets(
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
	// Parse secret names from YAML docs
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
			// If we hit a non-indented line after metadata, stop looking
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
		return strings.Join(result, "\n"), fmt.Errorf("flux-verify-secrets: %d secret(s) missing: %s", len(missing), strings.Join(missing, ", "))
	}

	return strings.Join(result, "\n"), nil
}

// FluxWaitForReconciliation runs flux check with retry, reconciles sources,
// and gets all Flux resources.
func (m *KubernetesDeployment) FluxWaitForReconciliation(
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
	// +default="ghcr.io/fluxcd/flux-cli:v2.7.5"
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
func (m *KubernetesDeployment) FluxBootstrap(
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
	// +default="2.4.0"
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
	// +default="ghcr.io/fluxcd/flux-cli:v2.7.5"
	fluxCliImage string,
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

	// Build KCL parameters from dedicated flags
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

	// Split multi-document YAML into secret and config documents
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

	// Log parameter keys for debugging
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
		err := m.FluxDeployOperator(ctx, kubeConfig, helmfileRef, src)
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

package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/kubernetes-deployment/internal/dagger"
)

func (m *KubernetesDeployment) FluxBootstrap(
	ctx context.Context,
	// OCI KCL module source for rendering Flux instance config
	// +optional
	// +default="ghcr.io/stuttgart-things/kcl-flux-instance:0.3.3"
	ociSource string,
	// Comma-separated key=value pairs for KCL parameters (e.g., "name=flux,namespace=flux-system,version=2.4.0")
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
	// Flux CLI container image
	// +optional
	// +default="ghcr.io/fluxcd/flux-cli:v2.4.0"
	fluxCliImage string,
) (string, error) {

	var results []string

	// =========================================================================
	// Phase 1: Render Flux Instance Config (KCL)
	// =========================================================================

	params := configParameters

	if renderSecrets {
		if gitUsername != nil {
			val, err := gitUsername.Plaintext(ctx)
			if err != nil {
				return "", fmt.Errorf("phase 1: read gitUsername secret: %w", err)
			}
			params += ",gitUsername=" + val
		}

		if gitPassword != nil {
			val, err := gitPassword.Plaintext(ctx)
			if err != nil {
				return "", fmt.Errorf("phase 1: read gitPassword secret: %w", err)
			}
			params += ",gitPassword=" + val
		}

		if sopsAgeKey != nil {
			val, err := sopsAgeKey.Plaintext(ctx)
			if err != nil {
				return "", fmt.Errorf("phase 1: read sopsAgeKey secret: %w", err)
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

	renderedContent, err := renderedFile.Contents(ctx)
	if err != nil {
		return "", fmt.Errorf("phase 1: get rendered content: %w", err)
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

	results = append(results, fmt.Sprintf("Phase 1: Rendered %d config doc(s) and %d secret doc(s)", len(configDocs), len(secretDocs)))

	// =========================================================================
	// Phase 2: Apply Secrets to Cluster
	// =========================================================================

	if applySecrets && len(secretDocs) > 0 {
		secretContent := strings.Join(secretDocs, "---\n")

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
			return "", fmt.Errorf("phase 2: apply secrets to cluster: %w", err)
		}

		results = append(results, "Phase 2: Secrets applied to cluster")
	} else {
		results = append(results, "Phase 2: Skipped (applySecrets=false or no secrets)")
	}

	// =========================================================================
	// Phase 3: Encrypt Secrets with SOPS
	// =========================================================================

	var secretsDir *dagger.Directory

	if encryptSecrets && len(secretDocs) > 0 {
		if agePublicKey == nil {
			return "", fmt.Errorf("phase 3: encryptSecrets=true but agePublicKey is nil")
		}

		secretContent := strings.Join(secretDocs, "---\n")

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

		encryptedContent, err := encryptedFile.Contents(ctx)
		if err != nil {
			return "", fmt.Errorf("phase 3: read encrypted content: %w", err)
		}

		secretsDir = dag.Directory().
			WithNewFile("secrets.yaml", encryptedContent)

		results = append(results, "Phase 3: Secrets encrypted with SOPS")
	} else if len(secretDocs) > 0 {
		secretContent := strings.Join(secretDocs, "---\n")
		secretsDir = dag.Directory().
			WithNewFile("secrets.yaml", secretContent)

		results = append(results, "Phase 3: Skipped encryption (encryptSecrets=false)")
	} else {
		results = append(results, "Phase 3: Skipped (no secrets to encrypt)")
	}

	// =========================================================================
	// Phase 4: Commit to Git
	// =========================================================================

	if commitToGit {
		if repository == "" {
			return "", fmt.Errorf("phase 4: commitToGit=true but repository is empty")
		}
		if gitToken == nil {
			return "", fmt.Errorf("phase 4: commitToGit=true but gitToken is nil")
		}

		configContent := strings.Join(configDocs, "---\n")
		commitDir := dag.Directory().
			WithNewFile("config.yaml", configContent)

		if secretsDir != nil {
			commitDir = commitDir.WithDirectory(".", secretsDir)
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
			return "", fmt.Errorf("phase 4: commit to git: %w", err)
		}

		results = append(results, fmt.Sprintf("Phase 4: Committed to %s branch %s at %s", repository, branchName, destinationPath))
	} else {
		results = append(results, "Phase 4: Skipped (commitToGit=false)")
	}

	// =========================================================================
	// Phase 5: Deploy Flux Operator via Helmfile
	// =========================================================================

	if deployOperator {
		err := m.DeployHelmfile(
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
		if err != nil {
			return "", fmt.Errorf("phase 5: deploy flux operator via helmfile: %w", err)
		}

		results = append(results, "Phase 5: Flux operator deployed via Helmfile")
	} else {
		results = append(results, "Phase 5: Skipped (deployOperator=false)")
	}

	// =========================================================================
	// Phase 6: Wait for Reconciliation (Flux CLI)
	// =========================================================================

	if waitForReconciliation {
		checkOutput, err := dag.Container().
			From(fluxCliImage).
			WithMountedSecret("/root/.kube/config", kubeConfig).
			WithEnvVariable("KUBECONFIG", "/root/.kube/config").
			WithExec([]string{"flux", "check", "--timeout", reconciliationTimeout}).
			Stdout(ctx)
		if err != nil {
			return "", fmt.Errorf("phase 6: flux check failed: %w", err)
		}

		results = append(results, fmt.Sprintf("Phase 6: Flux check passed:\n%s", checkOutput))

		getAllOutput, err := dag.Container().
			From(fluxCliImage).
			WithMountedSecret("/root/.kube/config", kubeConfig).
			WithEnvVariable("KUBECONFIG", "/root/.kube/config").
			WithExec([]string{"flux", "get", "all", "-n", namespace}).
			Stdout(ctx)
		if err != nil {
			results = append(results, fmt.Sprintf("Phase 6: Warning - flux get all failed: %v", err))
		} else {
			results = append(results, fmt.Sprintf("Phase 6: Flux resources:\n%s", getAllOutput))
		}
	} else {
		results = append(results, "Phase 6: Skipped (waitForReconciliation=false)")
	}

	return strings.Join(results, "\n"), nil
}

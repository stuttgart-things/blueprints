package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/flux/internal/dagger"
)

// RenderConfig renders the Flux instance configuration using a KCL module.
// Returns the full rendered YAML (multi-document).
func (m *Flux) RenderConfig(
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
				return "", fmt.Errorf("render-config: read gitUsername: %w", err)
			}
			params += ",gitUsername=" + val
		}

		if gitPassword != nil { // pragma: allowlist secret
			val, err := gitPassword.Plaintext(ctx) // pragma: allowlist secret
			if err != nil {
				return "", fmt.Errorf("render-config: read gitPassword: %w", err)
			}
			params += ",gitPassword=" + val
		}

		if sopsAgeKey != nil {
			val, err := sopsAgeKey.Plaintext(ctx)
			if err != nil {
				return "", fmt.Errorf("render-config: read sopsAgeKey: %w", err)
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

// CommitConfig commits rendered config and optional secrets to a Git repository.
func (m *Flux) CommitConfig(
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
		return "", fmt.Errorf("commit-config: %w", err)
	}

	return fmt.Sprintf("Committed to %s branch %s at %s", repository, branchName, destinationPath), nil
}

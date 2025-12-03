package main

import (
	"context"
	"dagger/configuration/internal/dagger"
	"fmt"
)

func (v *Configuration) RenderFluxKustomization(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	// OCI source path (e.g., oci://ghcr.io/stuttgart-things/kcl-flux-instance)
	// +optional
	ociSource string,
	// KCL parameters as comma-separated key=value pairs
	// +optional
	configParameters string,
	// Entry point file name
	// +optional
	// +default="main.k"
	entrypoint string,
	// Repository in format "owner/repo"
	// +optional
	repository string,
	// +optional
	// +default="main"
	baseBranch string,
	// +optional
	// Name of the new branch to create
	branchName string,
	// Destination path within the repository (e.g., "flux/" or "clusters/prod/")
	// +optional
	// +default="flux/"
	destinationPath string,
	// +optional
	// +default="false"
	createBranch bool,
	// +optional
	// +default="false"
	commitChanges bool,
	// +optional
	// +default="false"
	applyToCluster bool,
	// Kubeconfig secret for authentication
	// +optional
	kubeConfig *dagger.Secret,
	// Namespace for the operation
	// +optional
	// +default="flux-system"
	namespace string,
	// +optional
	// GitHub token for authentication
	token *dagger.Secret,
) (*dagger.Directory, error) {

	// Render KCL templates
	renderedFile := dag.Kcl().Run(
		dagger.KclRunOpts{
			Source:     src,
			OciSource:  ociSource,
			Parameters: configParameters,
			Entrypoint: entrypoint,
		})

	// EXPORT RENDERED FILE CONTENT
	renderedContent, err := renderedFile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rendered content: %w", err)
	}

	// CREATE DIRECTORY WITH RENDERED OUTPUT
	outputDir := dag.Directory().
		WithNewFile("rendered-output.yaml", renderedContent)

	// CREATE BRANCH FOR RENDERED TEMPLATES
	if createBranch {
		_, err := dag.Git().CreateGithubBranch(
			ctx,
			repository,
			branchName,
			token,
			dagger.GitCreateGithubBranchOpts{
				BaseBranch: baseBranch,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create branch: %w", err)
		}
	}

	// COMMIT CHANGES TO BRANCH
	if commitChanges {
		// Add rendered files to the branch
		_, err = dag.Git().AddFolderToGithubBranch(
			ctx,
			repository,
			branchName,
			"Add rendered KCL templates",
			token,
			outputDir,
			destinationPath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to add files to branch: %w", err)
		}
	}

	if applyToCluster {

		dag.Kubernetes().Kubectl(ctx,
			dagger.KubernetesKubectlOpts{
				Operation:       "apply",
				SourceFile:      outputDir.File("rendered-output.yaml"),
				KubeConfig:      kubeConfig,
				Namespace:       namespace,
				AdditionalFlags: "",
			},
		)

	}

	// RETURN OUTPUT DIRECTORY
	return outputDir, nil
}

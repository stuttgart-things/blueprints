package main

import (
	"context"
	"dagger/configuration/internal/dagger"
)

func (v *Configuration) VsphereVm(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	configParameters,
	// +optional
	variablesFile,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/vsphere-vm/refs/heads/main/templates/vm.yaml.tmpl,https://raw.githubusercontent.com/stuttgart-things/vsphere-vm/refs/heads/main/templates/README.md.tmpl"
	templatePaths string,
	// Repository in format "owner/repo"
	// +optional
	repository string,
	// +optional
	// Name of the new branch to create
	branchName string,
	// Base ref/branch to create from (e.g., "main", "develop")
	// +optional
	// +default="main"
	baseBranch string,
	// +optional
	// GitHub token for authentication
	token *dagger.Secret,
	// +optional
	// +default="false"
	createBranch bool,
	// +optional
	// +default="false"
	commitConfig bool,
	// +optional
	// +default="false"
	createPullRequest bool,
) (*dagger.Directory, error) {

	// RENDER TEMPLATES WITH PROVIDED PARAMETERS AND VARIABLES FILE
	renderedTemplates := dag.Templating().Render(
		src,
		templatePaths,
		dagger.TemplatingRenderOpts{
			Variables:     configParameters,
			VariablesFile: variablesFile,
		},
	)

	// CREATE BRANCH FOR RENDERED TEMPLATES
	if createBranch {
		dag.Git().CreateGithubBranch(
			ctx,
			repository,
			branchName,
			token,
			dagger.GitCreateGithubBranchOpts{
				BaseBranch: baseBranch,
			},
		)
	}

	// ADD FOLDER TO REPOSITORY/BRANCH
	if commitConfig {
		dag.Git().AddFolderToGithubBranch(
			ctx,
			repository,
			branchName,
			"new commit: add vsphere vm configuration",
			token,
			renderedTemplates,
			"vsphere-vm-config",
			dagger.GitAddFolderToGithubBranchOpts{
				AuthorName:  "",
				AuthorEmail: "",
			},
		)
	}

	// CREATE PR FOR BRANCH WITH RENDERED TEMPLATES
	if createPullRequest {
		dag.Git().CreateGithubPullRequest(
			ctx,
			repository,
			branchName,
			"Automated PR: Add rendered vSphere VM configuration",
			"This PR adds the rendered vSphere VM configuration files.",
			token,
		)
	}

	return renderedTemplates, nil
}

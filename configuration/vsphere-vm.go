package main

import (
	"context"
	"dagger/configuration/internal/dagger"
	"fmt"
	"strings"
)

func (v *Configuration) VsphereVm(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	configParameters,
	// +optional
	variablesFile,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/vsphere-vm/refs/heads/main/templates/vm.tf.tmpl,https://raw.githubusercontent.com/stuttgart-things/vsphere-vm/refs/heads/main/templates/README.md.tmpl"
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
	// +default="true"
	renderAnsibleRequirements bool,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	ansibleRequirementsTemplate string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	ansibleRequirementsData string,
	// +optional
	// +default="true"
	renderExecutionfile bool,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/blueprints/refs/heads/main/tests/vm/execution-vars.yaml"
	executionfileData string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/blueprints/refs/heads/main/tests/vm/execution.yaml.tmpl"
	executionfileTemplate string,
	// +optional
	// +default="false"
	commitConfig bool,
	// +optional
	// +default="false"
	createPullRequest bool,
	// +optional
	// +default=""
	commitMessage string,
	// +optional
	// +default=""
	destinationFolder string,
	// +optional
	// +default="./"
	destinationBasePath string,
	// +optional
	// +default=""
	authorName string,
	// +optional
	// +default=""
	authorEmail string,
	// +optional
	// +default=""
	pullRequestTitle string,
	// +optional
	// +default=""
	pullRequestBody string,
) (*dagger.Directory, error) {

	// ANALYZE VARIABLES
	// Map of mandatory configuration keys that must be present in configParameters
	// The boolean value (true) indicates that the key is required for validation
	mandatoryKeys := map[string]bool{
		"name":            true,
		"count":           true,
		"ram":             true,
		"template":        true,
		"disk":            true,
		"cpu":             true,
		"firmware":        true,
		"vm_folder":       true,
		"datacenter":      true,
		"datastore":       true,
		"resourcePool":    true,
		"network":         true,
		"useVault":        true,
		"vaultSecretPath": true,
	}

	configMap, err := analyzeConfigString(configParameters, mandatoryKeys)
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Extract VM name and datacenter for branch/commit message generation
	vmName, _ := configMap["name"].(string)
	datacenter, _ := configMap["datacenter"].(string)
	datacenter = strings.TrimPrefix(datacenter, "/")

	// Generate branch name from VM name and datacenter if not provided
	if branchName == "" {
		// Construct branch name: vmname-datacenter (e.g., "demo-infra1-LabUL")
		branchName = fmt.Sprintf("%s-%s", vmName, datacenter)
	}

	// Generate commit message if not provided
	if commitMessage == "" {
		// Construct commit message: "Add vsphere vm configuration for {vmname} in {datacenter}"
		commitMessage = fmt.Sprintf("Add vsphere vm configuration for %s in %s", vmName, datacenter)
	}

	// Generate destination folder from VM name and datacenter if not provided
	if destinationFolder == "" {
		// Construct destination folder: vmname-datacenter (e.g., "demo-infra1-LabUL")
		destinationFolder = fmt.Sprintf("%s-%s", vmName, datacenter)
	}

	// Generate pull request title if not provided
	if pullRequestTitle == "" {
		// Construct PR title: "Add vSphere VM configuration for {vmname} in {datacenter}"
		pullRequestTitle = fmt.Sprintf("Add vSphere VM configuration for %s in %s", vmName, datacenter)
	}

	// Generate pull request body if not provided
	if pullRequestBody == "" {
		// Construct PR body with configuration details
		pullRequestBody = fmt.Sprintf("This PR adds the rendered vSphere VM configuration for %s in datacenter %s.", vmName, datacenter)
	}

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

	// RENDER ANSIBLE REQUIREMENTS IF REQUESTED
	if renderAnsibleRequirements {
		ansibleReqs, err := v.CreateAnsibleRequirementFiles(
			ctx,
			src,
			ansibleRequirementsTemplate,
			ansibleRequirementsData,
			true,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create ansible requirements: %w", err)
		}

		// ADD Requirements to rendered Templates
		renderedTemplates = renderedTemplates.WithDirectory("ansible", ansibleReqs)
	}

	// RENDER EXECUTION FILE IF REQUESTED
	if renderExecutionfile {
		executionFile, err := v.RenderMetadata(
			ctx,
			src,
			"",
			executionfileTemplate,
			executionfileData,
			false,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to render execution file: %w", err)
		}

		// ADD Executionfile to rendered Templates
		renderedTemplates = renderedTemplates.WithDirectory(".", executionFile)
	}

	// 2. Fix the path construction
	destinationPath := strings.TrimSuffix(destinationBasePath, "/") + "/" + destinationFolder
	// 3. Use the fixed path
	if commitConfig {
		_, err := dag.Git().AddFolderToGithubBranch(
			ctx,
			repository,
			branchName,
			commitMessage,
			token,
			renderedTemplates,
			destinationPath,
			dagger.GitAddFolderToGithubBranchOpts{
				AuthorName:  authorName,
				AuthorEmail: authorEmail,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to add folder to branch: %w", err)
		}
	}

	// CREATE PR FOR BRANCH WITH RENDERED TEMPLATES
	if createPullRequest {
		dag.Git().CreateGithubPullRequest(
			ctx,
			repository,
			branchName,
			pullRequestTitle,
			pullRequestBody,
			token,
		)
	}

	return renderedTemplates, nil
}

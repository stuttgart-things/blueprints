package main

import (
	"context"
	"dagger/vmtemplate/internal/dagger"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenderBuildConfig renders templates by merging multiple YAML variable files
// (environment + OS + build overrides) into a single context. Later files
// override earlier ones. Returns a directory with rendered files.
func (m *Vmtemplate) RenderBuildConfig(
	ctx context.Context,
	// Directory containing the template files (.tmpl)
	templatesDir *dagger.Directory,
	// Comma-separated list of template files to render
	templates string,
	// Directory containing build-specific variable files and static files
	buildDir *dagger.Directory,
	// Comma-separated list of YAML variable files to merge, in order of priority (last wins)
	variablesFiles string,
	// Additional directory containing shared variable files (e.g., environment configs)
	// +optional
	envDir *dagger.Directory,
	// Comma-separated key=value overrides with highest priority (e.g., "isoChecksum=abc123,cpus=16")
	// +optional
	overrides string,
) (*dagger.Directory, error) {

	// MERGE BUILD DIR WITH ENV DIR SO ALL VARIABLE FILES ARE ACCESSIBLE
	varsDir := buildDir
	if envDir != nil {
		varsDir = varsDir.WithDirectory(".", envDir)
	}

	// MERGE VARIABLE FILES
	merged := make(map[string]interface{})

	for _, vf := range strings.Split(variablesFiles, ",") {
		vf = strings.TrimSpace(vf)
		if vf == "" {
			continue
		}

		content, err := varsDir.File(vf).Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables file %s: %w", vf, err)
		}

		var vars map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &vars); err != nil {
			return nil, fmt.Errorf("failed to parse variables file %s: %w", vf, err)
		}

		for k, v := range vars {
			merged[k] = v
		}
	}

	// WRITE MERGED VARIABLES TO TEMPORARY FILE
	mergedBytes, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged variables: %w", err)
	}

	mergedFileName := ".merged-vars.yaml"
	srcDir := templatesDir.
		WithDirectory(".", buildDir).
		WithNewFile(mergedFileName, string(mergedBytes))

	fmt.Println("RENDERING BUILD TEMPLATES:", templates)

	renderOpts := dagger.TemplatingRenderOpts{
		VariablesFile: mergedFileName,
		StrictMode:    true,
	}

	if overrides != "" {
		renderOpts.Variables = overrides
	}

	renderedDir := dag.Templating().Render(
		srcDir,
		templates,
		renderOpts,
	)

	return renderedDir, nil
}

// RenderAndCommit renders templates and optionally commits the result to a
// GitHub branch with an optional pull request.
func (m *Vmtemplate) RenderAndCommit(
	ctx context.Context,
	// Directory containing packer template files (.tmpl)
	packerTemplatesDir *dagger.Directory,
	// Comma-separated list of packer template files to render
	packerTemplates string,
	// Directory containing test VM template files (.tmpl)
	// +optional
	testVmTemplatesDir *dagger.Directory,
	// Comma-separated list of test VM template files to render
	// +optional
	testVmTemplates string,
	// Directory containing build-specific variables and static files
	buildDir *dagger.Directory,
	// Additional directory containing shared variable files (e.g., environment configs)
	// +optional
	envDir *dagger.Directory,
	// Comma-separated list of YAML variable files to merge, in priority order (last wins)
	variablesFiles string,
	// Comma-separated key=value overrides with highest priority (e.g., "isoChecksum=abc123,cpus=16")
	// +optional
	overrides string,
	// GitHub repository (e.g., "stuttgart-things/stuttgart-things")
	// +optional
	repository string,
	// GitHub authentication token
	// +optional
	token *dagger.Secret,
	// Branch name for the commit
	// +optional
	branchName string,
	// Base branch to create from
	// +optional
	// +default="main"
	baseBranch string,
	// Create a new branch before committing
	// +optional
	// +default=false
	createBranch bool,
	// Commit rendered files to the branch
	// +optional
	// +default=false
	commitConfig bool,
	// Create a pull request after committing
	// +optional
	// +default=false
	createPullRequest bool,
	// Commit message
	// +optional
	commitMessage string,
	// Destination path in the repository for packer files
	// +optional
	packerDestinationPath string,
	// Destination path in the repository for test VM files
	// +optional
	testVmDestinationPath string,
	// Pull request title
	// +optional
	pullRequestTitle string,
	// Pull request body
	// +optional
	pullRequestBody string,
) (*dagger.Directory, error) {

	// RENDER PACKER TEMPLATES
	fmt.Println("RENDERING PACKER TEMPLATES:", packerTemplates)
	renderedPackerDir, err := m.RenderBuildConfig(
		ctx, packerTemplatesDir, packerTemplates, buildDir, variablesFiles, envDir, overrides,
	)
	if err != nil {
		return nil, fmt.Errorf("rendering packer templates failed: %w", err)
	}

	// MERGE RENDERED PACKER FILES WITH STATIC BUILD FILES
	outputDir := buildDir.WithDirectory(".", renderedPackerDir)

	// RENDER TEST VM TEMPLATES IF PROVIDED
	var renderedTestVmDir *dagger.Directory
	if testVmTemplates != "" && testVmTemplatesDir != nil {
		fmt.Println("RENDERING TEST VM TEMPLATES:", testVmTemplates)
		renderedTestVmDir, err = m.RenderBuildConfig(
			ctx, testVmTemplatesDir, testVmTemplates, buildDir, variablesFiles, envDir, overrides,
		)
		if err != nil {
			return nil, fmt.Errorf("rendering test VM templates failed: %w", err)
		}
		outputDir = outputDir.WithDirectory("test-vm", renderedTestVmDir)
	}

	if !createBranch && !commitConfig && !createPullRequest {
		return outputDir, nil
	}

	if repository == "" || token == nil {
		return outputDir, fmt.Errorf("repository and token are required for git operations")
	}

	if branchName == "" {
		branchName = "rendered-packer-config"
	}
	if commitMessage == "" {
		commitMessage = "feat: add rendered packer build config"
	}
	if pullRequestTitle == "" {
		pullRequestTitle = "feat: rendered packer build configuration"
	}
	if pullRequestBody == "" {
		pullRequestBody = "This PR adds rendered packer build configuration files.\n\nGenerated by vmtemplate RenderAndCommit."
	}

	if createBranch {
		fmt.Printf("CREATING BRANCH %s FROM %s\n", branchName, baseBranch)
		if _, err := dag.Git().CreateGithubBranch(
			ctx, repository, branchName, token,
			dagger.GitCreateGithubBranchOpts{BaseBranch: baseBranch},
		); err != nil {
			return outputDir, fmt.Errorf("creating branch failed: %w", err)
		}
	}

	if commitConfig && packerDestinationPath != "" {
		// BUILD SINGLE COMMIT DIRECTORY: static files (base) + rendered files (overlay)
		commitDir := buildDir.WithDirectory(".", renderedPackerDir)
		if renderedTestVmDir != nil {
			commitDir = commitDir.WithDirectory("test-vm", renderedTestVmDir)
		}

		fmt.Printf("COMMITTING ALL FILES TO %s/%s\n", branchName, packerDestinationPath)
		if _, err := dag.Git().AddFolderToGithubBranch(
			ctx, repository, branchName, commitMessage, token,
			commitDir, packerDestinationPath,
		); err != nil {
			return outputDir, fmt.Errorf("committing files failed: %w", err)
		}
	}

	if createPullRequest {
		fmt.Printf("CREATING PULL REQUEST: %s\n", pullRequestTitle)
		prUrl, err := dag.Git().CreateGithubPullRequest(
			ctx, repository, branchName, pullRequestTitle, pullRequestBody, token,
			dagger.GitCreateGithubPullRequestOpts{BaseBranch: baseBranch},
		)
		if err != nil {
			return outputDir, fmt.Errorf("creating pull request failed: %w", err)
		}
		fmt.Printf("PULL REQUEST CREATED: %s\n", prUrl)
	}

	return outputDir, nil
}

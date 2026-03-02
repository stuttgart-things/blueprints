package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
	"strings"
)

func (v *Vm) BakeFromGit(
	ctx context.Context,
	// Repository to clone from GitHub
	gitRepository string,
	// Ref/Branch to checkout - If not specified, defaults to "main"
	// +optional
	// +default="main"
	gitRef string,
	// Github token for authentication (private repositories)
	// +optional
	gitToken *dagger.Secret,
) (*dagger.Directory, error) {

	// CLONE GIT REPOSITORY
	workingDir = dag.Git().CloneGithub(
		gitRepository,
		gitToken,
		dagger.GitCloneGithubOpts{
			Ref: gitRef,
		},
	)
	workDir = "/tmp/repo"

	// INIT WORKING CONTAINER
	ctr, err := v.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}
	ctr = ctr.WithDirectory(workDir, workingDir).WithWorkdir(workDir)

	result, err := ctr.WithExec([]string{"ls", "-lta", workDir}).Stdout(ctx)
	fmt.Println("Working directory contents:", result)

	return workingDir, nil
}

// CommitToGit commits a directory of files to a GitHub repository branch.
func (m *Vm) CommitToGit(
	ctx context.Context,
	// Directory containing files to commit
	sourceDir *dagger.Directory,
	// Repository in "owner/repo" format
	repository string,
	// Branch name for git operations
	// +optional
	// +default="main"
	branchName string,
	// Commit message
	// +optional
	// +default="Add files via Dagger"
	commitMessage string,
	// Destination path within the repository
	// +optional
	// +default="/"
	destinationPath string,
	// GitHub token for authentication
	gitToken *dagger.Secret,
) (string, error) {

	_, err := dag.Git().AddFolderToGithubBranch(
		ctx,
		repository,
		branchName,
		commitMessage,
		gitToken,
		sourceDir,
		destinationPath,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no changes to commit") {
			return fmt.Sprintf("No changes to commit (files already up-to-date in %s)", repository), nil
		}
		return "", fmt.Errorf("commit-to-git: %w", err)
	}

	return fmt.Sprintf("Committed to %s branch %s at %s", repository, branchName, destinationPath), nil
}

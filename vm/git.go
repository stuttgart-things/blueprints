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
// Optionally creates a new branch and opens a pull request.
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
	// If non-empty, create this branch (from branchName as base) and commit there instead
	// +optional
	createBranch string,
	// If true (and createBranch set), open a PR from the new branch back to branchName
	// +optional
	createPr bool,
	// PR title (defaults to commitMessage if empty)
	// +optional
	prTitle string,
) (string, error) {

	// Determine the target branch for the commit
	targetBranch := branchName

	// Create a new branch if requested
	if createBranch != "" {
		_, err := dag.Git().CreateGithubBranch(ctx, repository, createBranch, gitToken, dagger.GitCreateGithubBranchOpts{
			BaseBranch: branchName,
		})
		if err != nil {
			return "", fmt.Errorf("create-branch: %w", err)
		}
		targetBranch = createBranch
	}

	// Commit files to the target branch
	_, err := dag.Git().AddFolderToGithubBranch(
		ctx,
		repository,
		targetBranch,
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

	// Create a pull request if requested
	if createPr && createBranch != "" {
		title := prTitle
		if title == "" {
			title = commitMessage
		}

		prURL, err := dag.Git().CreateGithubPullRequest(ctx, repository, createBranch, title, commitMessage, gitToken, dagger.GitCreateGithubPullRequestOpts{
			BaseBranch: branchName,
		})
		if err != nil {
			return "", fmt.Errorf("create-pr: %w", err)
		}
		return fmt.Sprintf("Committed to %s branch %s at %s — PR: %s", repository, createBranch, destinationPath, prURL), nil
	}

	return fmt.Sprintf("Committed to %s branch %s at %s", repository, targetBranch, destinationPath), nil
}

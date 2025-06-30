package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
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
	workingDir = dag.Git().CloneGitHub(
		gitRepository,
		gitToken,
		dagger.GitCloneGitHubOpts{
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

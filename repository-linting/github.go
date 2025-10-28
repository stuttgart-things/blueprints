package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// CreateGithubIssue creates a GitHub issue for the linting findings
func (m *RepositoryLinting) CreateGithubIssue(
	ctx context.Context,
	// Repository in format "owner/repo"
	repository string,
	// Ref/Branch to checkout - If not specified, defaults to "main"
	// +optional
	// +default="main"
	ref string,
	title,
	body,
	// +optional
	label string,
	// +optional
	assignees []string,
	// GitHub token for authentication
	token *dagger.Secret) (string, error) {

	return dag.Git().CreateGithubIssue(
		ctx,
		repository,
		title,
		body,
		token,
		dagger.GitCreateGithubIssueOpts{
			Ref:       ref,
			Label:     label,
			Assignees: assignees,
		},
	)
}

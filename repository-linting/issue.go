package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// CREATE A GITHUB ISSUE WITH AI-ENHANCED FORMATTING
func (m *RepositoryLinting) CreateIssue(
	ctx context.Context,
	// User's issue description
	content string,
	// GitHub configuration
	repository string,
	// Ref/Branch to checkout - If not specified, defaults to "main"
	// +optional
	// +default="main"
	ref string,
	token *dagger.Secret,
	// +optional
	label string,
	// +optional
	assignees []string,
	model string,
) (string, error) {

	// PREPARE THE AI ENVIRONMENT WITH OUTPUTS FOR GITHUB ISSUE
	environment := dag.Env().
		WithStringInput("content", content, "the user's issue description").
		WithStringOutput("title", "the issue title").
		WithStringOutput("body", "the detailed and well-formatted issue body")

	// AI AGENT PROMPT
	work := dag.LLM().
		WithModel(model).
		WithEnv(environment).
		WithPrompt(`
			You are a GitHub expert specializing in creating well-structured and professional issues.

			Take the user's issue description and transform it into a polished GitHub issue.

			Generate two outputs:
			1. A clear and concise issue title (max 80 characters) that captures the essence
			2. A detailed, well-formatted issue body with:
			   - Clear problem statement or feature request
			   - Context and background (if provided)
			   - Expected behavior vs actual behavior (for bugs)
			   - Proposed solution or requirements (if applicable)
			   - Additional notes or considerations
			   - Actionable next steps

			Use proper markdown formatting including:
			   - Headers (##) for sections
			   - Code blocks with syntax highlighting where appropriate
			   - Lists for clarity
			   - Bold/italic for emphasis
			   - Checkboxes for action items

			Make it professional, clear, and easy to understand.

			User's description:
			$content
		`)

	// GET THE AI-GENERATED TITLE AND BODY
	title, err := work.Env().Output("title").AsString(ctx)
	if err != nil {
		return "", err
	}

	body, err := work.Env().Output("body").AsString(ctx)
	if err != nil {
		return "", err
	}

	issueURL, err := m.CreateGithubIssue(
		ctx,
		repository,
		ref,
		title,
		body,
		label,
		assignees,
		token,
	)

	if err != nil {
		return "", err
	}

	return issueURL, nil
}

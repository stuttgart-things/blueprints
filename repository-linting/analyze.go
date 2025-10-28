package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// ANALYZE A LINTING REPORT FILE WITH AI AND CREATE A GITHUB ISSUE
func (m *RepositoryLinting) AnalyzeReportAndCreateIssue(
	ctx context.Context,
	reportFile *dagger.File,
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

	// READ THE REPORT CONTENTS
	reportContent, err := reportFile.Contents(ctx)
	if err != nil {
		return "", err
	}

	// PREPARE THE AI ENVIRONMENT WITH OUTPUTS FOR GITHUB ISSUE
	environment := dag.Env().
		WithStringInput("report", reportContent, "the linting report to analyze").
		WithStringOutput("title", "the issue title").
		WithStringOutput("body", "the detailed issue body with findings and recommendations")

	// AI AGENT PROMPT
	work := dag.LLM().
		WithModel(model).
		WithEnv(environment).
		WithPrompt(`
			You are an expert code reviewer.
			Analyze the following linting report and create a GitHub issue.

			Generate two outputs:
			1. A concise issue title (max 80 characters) that summarizes the main concern
			2. A detailed issue body with:
			   - Summary of findings
			   - Critical issues (if any)
			   - Improvement suggestions
			   - Action items

			Use markdown formatting for the body.

			Report:
			$report
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

// ANALYZE A LINTING REPORT FILE WITH AI AND RETURN A TEXT FILE WITH THE ANALYSIS
func (m *RepositoryLinting) AnalyzeReport(
	ctx context.Context,
	reportFile *dagger.File,
	// +optional
	// +default="ai-analysis.txt"
	outputFile string,
	model string,
) (*dagger.File, error) {

	// READ THE REPORT CONTENTS
	reportContent, err := reportFile.Contents(ctx)
	if err != nil {
		return nil, err
	}

	// PREPARE THE AI ENVIRONMENT
	environment := dag.Env().
		WithStringInput("report", reportContent, "the linting report to analyze").
		WithStringOutput("analysis", "the AI-generated analysis of the report")

	// AI AGENT PROMPT
	work := dag.LLM().
		WithEnv(environment).
		WithModel(model).
		WithPrompt(`
			You are an expert code reviewer.
			Analyze the following linting report and summarize the most important findings, improvement suggestions, and any critical issues.
			Be concise and actionable.
			Report:
			$report
		`)

	// GET THE ANALYSIS RESULT
	analysis, err := work.Env().Output("analysis").AsString(ctx)
	if err != nil {
		return nil, err
	}

	// RETURN AS A NEW FILE
	return dag.Directory().
		WithNewFile(outputFile, analysis).
		File(outputFile), nil
}

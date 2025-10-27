// RepositoryLinting Module
//
// This module provides advanced linting, validation, and AI-powered analysis for code repositories.
// It supports multi-technology linting (YAML, Markdown, etc.), merges findings, and enables automated review workflows.
// The module can analyze linting reports using AI agents to deliver actionable feedback and improvement suggestions.
//
// Key features:
// - Validate and lint multiple file types in a repository
// - Merge and summarize findings from different linters
// - Use AI to analyze linting reports and generate human-readable reviews
// - Integrate with Dagger pipelines for automated CI/CD quality gates
//
// Designed for extensibility and integration in modern DevOps and platform engineering environments.

package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

type RepositoryLinting struct{}

func (m *RepositoryLinting) ValidateMultipleTechnologies(
	ctx context.Context,
	// +optional
	// +default=".yamllint"
	yamlConfigPath string,
	// +optional
	// +default="yamllint-findings.txt"
	yamlOutputFile string,
	// +optional
	// +default=".mdlrc"
	markdownConfigPath string,
	// +optional
	// +default="markdown-findings.txt"
	markdownOutputFile string,
	// +optional
	// +default="all-findings.txt"
	mergedOutputFile string,
	src *dagger.Directory) *dagger.File {

	// Create default YAML linting config if not present
	yamlConfig := `---
extends: default

rules:
  line-length:
    max: 120
    level: warning
  document-start: disable
  truthy:
    allowed-values: ['true', 'false', 'yes', 'no']
  comments:
    min-spaces-from-content: 1
  indentation:
    spaces: 2
    indent-sequences: true
`

	// Create default Markdown linting config if not present
	markdownConfig := `{
  "default": true,
  "MD013": false,
  "MD033": false,
  "MD041": false,
  "line-length": false,
  "no-inline-html": false,
  "first-line-h1": false
}
`

	// Check if config files exist, if not create them with defaults
	srcWithConfigs := src

	// Check and add YAML config if missing
	yamlConfigFile := src.File(yamlConfigPath)
	if _, err := yamlConfigFile.Contents(ctx); err != nil {
		// Config doesn't exist, create it
		srcWithConfigs = srcWithConfigs.WithNewFile(yamlConfigPath, yamlConfig)
	}

	// Check and add Markdown config if missing
	markdownConfigFile := src.File(markdownConfigPath)
	if _, err := markdownConfigFile.Contents(ctx); err != nil {
		// Config doesn't exist, create it
		srcWithConfigs = srcWithConfigs.WithNewFile(markdownConfigPath, markdownConfig)
	}

	// Run linting with the (potentially augmented) source directory
	yamlReport := m.LintYAML(ctx, yamlConfigPath, yamlOutputFile, srcWithConfigs)
	markdownReport := m.LintMarkdown(ctx, markdownConfigPath, markdownOutputFile, srcWithConfigs)

	// Read the contents of both reports
	yamlContent, _ := yamlReport.Contents(ctx)
	markdownContent, _ := markdownReport.Contents(ctx)

	// Merge the reports with headers
	mergedContent := "=== YAML Linting Results ===\n" + yamlContent +
		"\n\n=== Markdown Linting Results ===\n" + markdownContent

	// Return as a new file
	return dag.Directory().
		WithNewFile(mergedOutputFile, mergedContent).
		File(mergedOutputFile)
}

// ANALYZE A LINTING REPORT FILE WITH AI AND RETURN A TEXT FILE WITH THE ANALYSIS
func (m *RepositoryLinting) AnalyzeReport(
	ctx context.Context,
	reportFile *dagger.File,
	// +optional
	// +default="ai-analysis.txt"
	outputFile string,
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

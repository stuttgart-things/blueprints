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
	// +default=true
	enableYaml bool,
	// +optional
	// +default=".yamllint"
	yamlConfigPath string,
	// +optional
	// +default="yamllint-findings.txt"
	yamlOutputFile string,
	// +optional
	// +default=true
	enableMarkdown bool,
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

	// Check and add YAML config if missing and YAML linting is enabled
	if enableYaml {
		yamlConfigFile := src.File(yamlConfigPath)
		if _, err := yamlConfigFile.Contents(ctx); err != nil {
			// Config doesn't exist, create it
			srcWithConfigs = srcWithConfigs.WithNewFile(yamlConfigPath, yamlConfig)
		}
	}

	// Check and add Markdown config if missing and Markdown linting is enabled
	if enableMarkdown {
		markdownConfigFile := src.File(markdownConfigPath)
		if _, err := markdownConfigFile.Contents(ctx); err != nil {
			// Config doesn't exist, create it
			srcWithConfigs = srcWithConfigs.WithNewFile(markdownConfigPath, markdownConfig)
		}
	}

	var yamlContent, markdownContent string

	// Run YAML linting if enabled
	if enableYaml {
		yamlReport := m.LintYAML(ctx, yamlConfigPath, yamlOutputFile, srcWithConfigs)
		yamlContent, _ = yamlReport.Contents(ctx)
	}

	// Run Markdown linting if enabled
	if enableMarkdown {
		markdownReport := m.LintMarkdown(ctx, markdownConfigPath, markdownOutputFile, srcWithConfigs)
		markdownContent, _ = markdownReport.Contents(ctx)
	}

	// Build merged content based on which linters are enabled
	mergedContent := ""

	if enableYaml {
		mergedContent += "=== YAML Linting Results ===\n" + yamlContent
	}

	if enableMarkdown {
		if enableYaml {
			mergedContent += "\n\n"
		}
		mergedContent += "=== Markdown Linting Results ===\n" + markdownContent
	}

	// If both are disabled, provide a message
	if !enableYaml && !enableMarkdown {
		mergedContent = "No linting technologies enabled. Set enableYaml and/or enableMarkdown to true."
	}

	// Return as a new file
	return dag.Directory().
		WithNewFile(mergedOutputFile, mergedContent).
		File(mergedOutputFile)
}

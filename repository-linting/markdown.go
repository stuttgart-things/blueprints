package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// LintYAML lints YAML files in the provided directory
func (m *RepositoryLinting) LintMarkdown(
	ctx context.Context,
	// +optional
	// +default=".mdlrc"
	configPath string,
	// +optional
	// +default="markdown-findings.txt"
	outputFile string,
	src *dagger.Directory) *dagger.File {
	return dag.Linting().LintMarkdown(
		src,
		dagger.LintingLintMarkdownOpts{
			ConfigPath: configPath,
			OutputFile: outputFile,
		},
	)
}

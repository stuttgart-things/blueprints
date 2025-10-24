// A generated module for RepositoryLinting functions
//
// This module has been generated via dagger init and serves as a reference to
// basic module structure as you get started with Dagger.
//
// Two functions have been pre-created. You can modify, delete, or add to them,
// as needed. They demonstrate usage of arguments and return types using simple
// echo and grep commands. The functions can be called from the dagger CLI or
// from one of the SDKs.
//
// The first line in this comment block is a short description line and the
// rest is a long description with more detail on the module's purpose or usage,
// if appropriate. All modules should have a short description.

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

	yamlReport := m.LintYAML(ctx, yamlConfigPath, yamlOutputFile, src)
	markdownReport := m.LintMarkdown(ctx, markdownConfigPath, markdownOutputFile, src)

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


// LintYAML lints YAML files in the provided directory
func (m *RepositoryLinting) LintYAML(
	ctx context.Context,
	// +optional
	// +default=".yamllint"
	configPath string,
	// +optional
	// +default="yamllint-findings.txt"
	outputFile string,
	src *dagger.Directory) (*dagger.File) {
	return dag.Linting().LintYaml(
		src,
		dagger.LintingLintYamlOpts{
			ConfigPath:  configPath,
			OutputFile: outputFile,
		},
	)
}

// LintYAML lints YAML files in the provided directory
func (m *RepositoryLinting) LintMarkdown(
	ctx context.Context,
	// +optional
	// +default=".mdlrc"
	configPath string,
	// +optional
	// +default="markdown-findings.txt"
	outputFile string,
	src *dagger.Directory) (*dagger.File) {
	return dag.Linting().LintMarkdown(
		src,
		dagger.LintingLintMarkdownOpts{
			ConfigPath:  configPath,
			OutputFile: outputFile,
		},
	)
}

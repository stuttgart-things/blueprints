package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// LintYAML lints YAML files in the provided directory
func (m *RepositoryLinting) LintYAML(
	ctx context.Context,
	// +optional
	// +default=".yamllint"
	configPath string,
	// +optional
	// +default="yamllint-findings.txt"
	outputFile string,
	src *dagger.Directory) *dagger.File {
	return dag.Linting().LintYaml(
		src,
		dagger.LintingLintYamlOpts{
			ConfigPath: configPath,
			OutputFile: outputFile,
		},
	)
}

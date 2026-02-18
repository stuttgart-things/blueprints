package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// RunPreCommit runs pre-commit hooks on the provided directory
func (m *RepositoryLinting) RunPreCommit(
	ctx context.Context,
	// +optional
	// +default=".pre-commit-config.yaml"
	configPath string,
	// +optional
	// +default="pre-commit-findings.txt"
	outputFile string,
	// +optional
	skipHooks []string,
	src *dagger.Directory) *dagger.File {
	return dag.Linting().RunPreCommit(
		src,
		dagger.LintingRunPreCommitOpts{
			ConfigPath: configPath,
			OutputFile: outputFile,
			SkipHooks:  skipHooks,
		},
	)
}

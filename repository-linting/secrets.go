package main

import (
	"context"
	"dagger/repository-linting/internal/dagger"
)

// ScanSecrets runs detect-secrets scan on the provided directory and returns a JSON findings report
func (m *RepositoryLinting) ScanSecrets(
	ctx context.Context,
	// +optional
	// +default="secret-findings.json"
	outputFile string,
	// +optional
	excludeFiles string,
	src *dagger.Directory) *dagger.File {
	return dag.Linting().ScanSecrets(
		src,
		dagger.LintingScanSecretsOpts{
			OutputFile:   outputFile,
			ExcludeFiles: excludeFiles,
		},
	)
}

// AutoFixSecrets uses AI to analyze detect-secrets findings and add pragma comments to flagged lines
func (m *RepositoryLinting) AutoFixSecrets(
	ctx context.Context,
	// +optional
	excludeFiles string,
	src *dagger.Directory) *dagger.Directory {
	return dag.Linting().AutoFixSecrets(
		src,
		dagger.LintingAutoFixSecretsOpts{
			ExcludeFiles: excludeFiles,
		},
	)
}

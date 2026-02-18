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
	"fmt"
	"strings"
	"sync"

	"dagger/repository-linting/internal/dagger"

	"golang.org/x/sync/errgroup"
)

type RepositoryLinting struct{}

type linterResult struct {
	name    string
	content string
}

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
	src *dagger.Directory,
	// +optional
	// +default=false
	enablePreCommit bool,
	// +optional
	// +default=".pre-commit-config.yaml"
	preCommitConfigPath string,
	// +optional
	// +default="pre-commit-findings.txt"
	preCommitOutputFile string,
	// +optional
	skipHooks []string,
	// +optional
	// +default=false
	enableSecrets bool,
	// +optional
	// +default="secret-findings.json"
	secretsOutputFile string,
	// +optional
	secretsExcludeFiles string,
	// +optional
	// +default="none"
	failOn string,
) (*dagger.File, error) {

	// Default configs
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

	// Ensure config files exist
	srcWithConfigs := src

	if enableYaml {
		yamlConfigFile := src.File(yamlConfigPath)
		if _, err := yamlConfigFile.Contents(ctx); err != nil {
			srcWithConfigs = srcWithConfigs.WithNewFile(yamlConfigPath, yamlConfig)
		}
	}

	if enableMarkdown {
		markdownConfigFile := src.File(markdownConfigPath)
		if _, err := markdownConfigFile.Contents(ctx); err != nil {
			srcWithConfigs = srcWithConfigs.WithNewFile(markdownConfigPath, markdownConfig)
		}
	}

	// Run linters in parallel using errgroup
	var mu sync.Mutex
	results := make(map[string]linterResult)

	g, ctx := errgroup.WithContext(ctx)

	if enableYaml {
		g.Go(func() error {
			report := m.LintYAML(ctx, yamlConfigPath, yamlOutputFile, srcWithConfigs)
			content, _ := report.Contents(ctx)
			mu.Lock()
			results["yaml"] = linterResult{name: "YAML Linting", content: content}
			mu.Unlock()
			return nil
		})
	}

	if enableMarkdown {
		g.Go(func() error {
			report := m.LintMarkdown(ctx, markdownConfigPath, markdownOutputFile, srcWithConfigs)
			content, _ := report.Contents(ctx)
			mu.Lock()
			results["markdown"] = linterResult{name: "Markdown Linting", content: content}
			mu.Unlock()
			return nil
		})
	}

	if enablePreCommit {
		g.Go(func() error {
			report := m.RunPreCommit(ctx, preCommitConfigPath, preCommitOutputFile, skipHooks, srcWithConfigs)
			content, _ := report.Contents(ctx)
			mu.Lock()
			results["precommit"] = linterResult{name: "Pre-Commit", content: content}
			mu.Unlock()
			return nil
		})
	}

	if enableSecrets {
		g.Go(func() error {
			report := m.ScanSecrets(ctx, secretsOutputFile, secretsExcludeFiles, srcWithConfigs)
			content, _ := report.Contents(ctx)
			mu.Lock()
			results["secrets"] = linterResult{name: "Secrets Scan", content: content} // pragma: allowlist secret
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Build merged output in fixed order
	order := []string{"yaml", "markdown", "precommit", "secrets"}
	var sections []string

	for _, key := range order {
		if r, ok := results[key]; ok {
			sections = append(sections, fmt.Sprintf("=== %s Results ===\n%s", r.name, r.content))
		}
	}

	mergedContent := ""
	if len(sections) > 0 {
		mergedContent = strings.Join(sections, "\n\n")
	} else {
		mergedContent = "No linting technologies enabled. Set enableYaml and/or enableMarkdown to true."
	}

	reportFile := dag.Directory().
		WithNewFile(mergedOutputFile, mergedContent).
		File(mergedOutputFile)

	// Evaluate fail condition
	if failOn == "any" {
		for _, key := range order {
			if r, ok := results[key]; ok {
				if strings.TrimSpace(r.content) != "" {
					return reportFile, fmt.Errorf("linter %q produced findings (failOn=any)", r.name)
				}
			}
		}
	}

	return reportFile, nil
}

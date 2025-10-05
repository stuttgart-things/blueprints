package main

import (
	"context"
	"dagger/go-microservice/internal/dagger"
	"dagger/go-microservice/stats"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

func (m *GoMicroservice) RunStaticStage(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	// +default="500s"
	lintTimeout string,
	// +optional
	// +default="1.24.4"
	goVersion string,
	// +optional
	// +default="linux"
	os string,
	// +optional
	// +default="amd64"
	arch string,
	// +optional
	// +default="main.go"
	goMainFile string,
	// +optional
	// +default="main"
	binName string,
	// +optional
	ldflags string,
	// +optional
	// +default="false"
	lintCanFail bool,
	// +optional
	// +default="./..."
	testArg string,
	// +optional
	// +default=true
	lintEnabled bool,
	// +optional
	// +default=true
	test bool,
) (*dagger.File, error) {
	startTime := time.Now()
	stats := stats.WorkflowStats{}
	g, gctx := errgroup.WithContext(ctx)

	// Lint step
	if lintEnabled {
		g.Go(func() error {
			lintStart := time.Now()
			// Get the container reference first
			lintContainer := dag.Go().Lint(
				src,
				dagger.GoLintOpts{Timeout: lintTimeout},
			)

			// Capture both stdout and stderr
			combinedOutput, err := lintContainer.Stdout(ctx)
			if err != nil {
				// Try to get more detailed output
				if exitErr, ok := err.(*dagger.ExecError); ok {
					combinedOutput = exitErr.Stderr + exitErr.Stdout
				}

				stats.Lint.Duration = time.Since(lintStart).String()

				if lintCanFail {
					// Capture the detailed output
					if combinedOutput != "" {
						stats.Lint.Findings = strings.Split(getExecOutput(err), "\n")
					} else {
						stats.Lint.Findings = []string{fmt.Sprintf("Linting failed (non-fatal): %v", err)}
					}
					return nil
				}
				return fmt.Errorf("error running lint: %s\n%w", combinedOutput, err)
			}

			// If no error, use the stdout
			stats.Lint.Duration = time.Since(lintStart).String()
			stats.Lint.Findings = strings.Split(combinedOutput, "\n")
			return nil
		})
	}

	// Test step
	if test {
		g.Go(func() error {
			testStart := time.Now()
			testOutput, err := dag.Go().Test(
				gctx,
				src,
				dagger.GoTestOpts{GoVersion: goVersion},
			)

			stats.Test.Duration = time.Since(testStart).String()

			if err != nil {
				return fmt.Errorf("error running tests: %w", err)
			}

			// Set empty coverage since security package is removed
			stats.Test.Coverage = ""
			_ = testOutput // Use testOutput to avoid unused variable warning
			return nil
		})
	}

	// Wait for all enabled steps to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Calculate total duration
	stats.TotalDuration = time.Since(startTime).String()

	// Generate JSON report
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error generating stats JSON: %w", err)
	}

	// Create report file
	statsFile := dag.Directory().
		WithNewFile("static-analysis-report.json", string(statsJSON)).
		File("static-analysis-report.json")

	return statsFile, nil
}

func getExecOutput(err error) string {
	if execErr, ok := err.(*dagger.ExecError); ok {
		return fmt.Sprintf("STDOUT:\n%s\n\nSTDERR:\n%s", execErr.Stdout, execErr.Stderr)
	}
	return err.Error()
}

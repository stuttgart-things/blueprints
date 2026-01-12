package main

import (
	"context"
	"dagger/go-microservice/internal/dagger"
	"dagger/go-microservice/stats"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	// +default="1.25.5"
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

	// Track errors separately without stopping execution
	var lintErr, testErr error
	var mu sync.Mutex

	// Lint step
	if lintEnabled {
		g.Go(func() error {
			lintStart := time.Now()
			lintContainer := dag.Go().Lint(
				src,
				dagger.GoLintOpts{Timeout: lintTimeout},
			)

			combinedOutput, err := lintContainer.Stdout(gctx)

			mu.Lock()
			stats.Lint.Duration = time.Since(lintStart).String()
			mu.Unlock()

			if err != nil {
				// Try to get detailed output
				if exitErr, ok := err.(*dagger.ExecError); ok {
					combinedOutput = exitErr.Stderr + "\n" + exitErr.Stdout
				}

				mu.Lock()
				stats.Lint.Failed = true
				stats.Lint.Error = err.Error()
				if combinedOutput != "" {
					stats.Lint.Findings = strings.Split(combinedOutput, "\n")
				} else {
					stats.Lint.Findings = []string{fmt.Sprintf("Linting failed: %v", err)}
				}
				lintErr = err
				mu.Unlock()

				// Don't return error - just capture it
				return nil
			}

			mu.Lock()
			stats.Lint.Failed = false
			stats.Lint.Findings = strings.Split(combinedOutput, "\n")
			mu.Unlock()

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

			mu.Lock()
			stats.Test.Duration = time.Since(testStart).String()
			mu.Unlock()

			if err != nil {
				// Capture test output for reporting
				outputStr := getExecOutput(err)
				if testOutput != "" {
					outputStr = testOutput + "\n" + outputStr
				}

				mu.Lock()
				stats.Test.Failed = true
				stats.Test.Error = err.Error()
				stats.Test.Output = outputStr
				testErr = err
				mu.Unlock()

				// Don't return error - just capture it
				return nil
			}

			mu.Lock()
			stats.Test.Failed = false
			stats.Test.Output = testOutput
			stats.Test.Coverage = ""
			mu.Unlock()

			return nil
		})
	}

	// Wait for all enabled steps to complete
	// Note: g.Wait() will only return an error if we explicitly return one from the goroutines
	// Since we're now returning nil in all cases, this should always succeed
	_ = g.Wait()

	// Calculate total duration
	stats.TotalDuration = time.Since(startTime).String()

	// Add overall status
	stats.HasFailures = (lintErr != nil && !lintCanFail) || testErr != nil

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

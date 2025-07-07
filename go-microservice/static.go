package main

import (
	"context"
	"dagger/go-microservice/internal/dagger"
	"dagger/go-microservice/security"
	"dagger/go-microservice/stats"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
	// +default="2.22.1"
	secureGoVersion string,
	// +optional
	// +default="false"
	lintCanFail bool, // If true, linting can fail without stopping the workflow
	// +optional
	// +default="./..."
	testArg string, // Arguments for `go test`
) (*dagger.File, error) {
	// CREATE A STRUCT TO HOLD THE STATISTICS
	stats := stats.WorkflowStats{}

	// START TIMING THE WORKFLOW
	startTime := time.Now()

	// CREATE A CHANNEL TO COLLECT ERRORS FROM GOROUTINES
	errChan := make(chan error, 5) // Buffer size of 5 for lint, build, test, security scan, and Trivy scan

	// RUN LINT STEP IN A GOROUTINE
	go func() {
		lintStart := time.Now()
		lintOutput, err := dag.Go().Lint(
			src,
			dagger.GoLintOpts{
				Timeout: lintTimeout,
			}).
			Stdout(ctx)

		if err != nil {
			if !lintCanFail {
				errChan <- fmt.Errorf("error running lint: %w", err)
				return
			}
			// IF LINTCANFAIL IS TRUE, LOG THE ERROR BUT CONTINUE
			stats.Lint.Findings = []string{fmt.Sprintf("Linting failed: %v", err)}
		} else {
			stats.Lint.Findings = strings.Split(lintOutput, "\n") // Split lint output into findings
		}
		stats.Lint.Duration = time.Since(lintStart).String()
		errChan <- nil
	}()

	// RUN SECURITY SCAN STEP IN A GOROUTINE
	go func() {
		securityScanStart := time.Now()
		reportFile := dag.
			Go().
			SecurityScan(
				src,
				dagger.GoSecurityScanOpts{
					SecureGoVersion: secureGoVersion,
				})

		// READ THE REPORT FILE CONTENTS
		reportContent, err := reportFile.Contents(ctx)
		if err != nil {
			errChan <- fmt.Errorf("error reading security report: %w", err)
			return
		}
		stats.SecurityScan.Findings = strings.Split(reportContent, "\n") // Split report content into findings

		stats.SecurityScan.Duration = time.Since(securityScanStart).String()
		errChan <- nil
	}()

	// RUN TEST STEP IN A GOROUTINE
	go func() {
		testStart := time.Now()
		testOutput, err := dag.Go().
			Test(
				ctx,
				src,
				dagger.GoTestOpts{
					GoVersion: goVersion,
				})

		if err != nil {
			errChan <- fmt.Errorf("error running tests: %w", err)
			return
		}
		stats.Test.Duration = time.Since(testStart).String()

		// EXTRACT COVERAGE FROM TEST OUTPUT
		coverage := security.ExtractCoverage(testOutput)
		stats.Test.Coverage = coverage
		errChan <- nil
	}()

	// WAIT FOR ALL GOROUTINES TO COMPLETE
	for i := 0; i < 3; i++ {
		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	// TRACK TOTAL WORKFLOW DURATION
	stats.TotalDuration = time.Since(startTime).String()

	// GENERATE JSON FILE WITH STATISTICS
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error generating stats JSON: %w", err)
	}

	// WRITE JSON TO A FILE IN THE CONTAINER
	statsFile := dag.Directory().
		WithNewFile("workflow-stats.json", string(statsJSON)).
		File("workflow-stats.json")

	// RETURN THE STATS FILE
	return statsFile, nil
}

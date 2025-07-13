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
	// +default="2.22.1"
	secureGoVersion string,
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
	securityScan bool,
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
			lintOutput, err := dag.Go().Lint(
				src,
				dagger.GoLintOpts{Timeout: lintTimeout},
			).Stdout(gctx)

			stats.Lint.Duration = time.Since(lintStart).String()

			if err != nil {
				if lintCanFail {
					stats.Lint.Findings = []string{fmt.Sprintf("Linting failed (non-fatal): %v", err)}
					return nil
				}
				return fmt.Errorf("error running lint: %w", err)
			}

			stats.Lint.Findings = strings.Split(lintOutput, "\n")
			return nil
		})
	}

	// Security scan step
	if securityScan {
		g.Go(func() error {
			securityStart := time.Now()
			reportFile := dag.Go().SecurityScan(
				src,
				dagger.GoSecurityScanOpts{
					SecureGoVersion: secureGoVersion,
				})

			reportContent, err := reportFile.Contents(gctx)
			stats.SecurityScan.Duration = time.Since(securityStart).String()

			if err != nil {
				return fmt.Errorf("error reading security report: %w", err)
			}

			stats.SecurityScan.Findings = strings.Split(reportContent, "\n")
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

			stats.Test.Coverage = security.ExtractCoverage(testOutput)
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

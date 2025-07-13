package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type StaticStageReport struct {
	Lint struct {
		Output   string   `json:"output,omitempty"`
		Error    string   `json:"error,omitempty"`
		Findings []string `json:"findings,omitempty"`
		Duration string   `json:"duration,omitempty"`
	} `json:"lint"`
	Scan struct {
		Output   string   `json:"output,omitempty"`
		Error    string   `json:"error,omitempty"`
		Findings []string `json:"findings,omitempty"`
		Duration string   `json:"duration,omitempty"`
	} `json:"scan"`
	TotalDuration string `json:"totalDuration"`
}

func (m *KubernetesMicroservice) RunStaticStage(
	ctx context.Context,
	// the src directory
	src *dagger.Directory,
	// +optional
	// +default=""
	pathToDockerfile string,
	// +optional
	// +default="Dockerfile"
	nameDockerfile string,
	// +optional
	// +default="HIGH,CRITICAL"
	severity string,
	// +optional
	// +default="0.64.1"
	trivyVersion string,
	// The failure threshold
	// +optional
	threshold string,
) (*dagger.File, error) {
	startTime := time.Now()
	report := StaticStageReport{}
	g, gctx := errgroup.WithContext(ctx)

	// LINT STEP
	g.Go(func() error {
		start := time.Now()
		output, err := m.LintDockerfile(
			gctx,
			src.Directory(pathToDockerfile),
			nameDockerfile,
			threshold,
		)
		report.Lint.Duration = time.Since(start).String()

		if err != nil {
			report.Lint.Error = err.Error()
			return nil // Non-fatal error
		}

		report.Lint.Output = output
		report.Lint.Findings = strings.Split(output, "\n")
		return nil
	})

	// SCAN STEP
	g.Go(func() error {
		start := time.Now()
		output := m.ScanFilesystem(
			gctx,
			src,
			severity,
			trivyVersion,
		)
		report.Scan.Duration = time.Since(start).String()

		outputContent, err := output.Contents(gctx)
		if err != nil {
			return err
		}

		report.Scan.Output = outputContent

		report.Scan.Output = outputContent
		report.Scan.Findings = strings.Split(outputContent, "\n")
		return nil
	})

	// WAIT FOR ALL STEPS TO COMPLETE
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// FINALIZE REPORT
	report.TotalDuration = time.Since(startTime).String()

	// GENERATE JSON REPORT
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report: %w", err)
	}

	// CREATE REPORT FILE
	reportFile := dag.Directory().
		WithNewFile("static-stage-report.json", string(reportJSON)).
		File("static-stage-report.json")

	return reportFile, nil
}

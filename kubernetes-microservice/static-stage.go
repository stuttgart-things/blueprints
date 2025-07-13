package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
	"fmt"
)

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
) {
	scanReport := m.
		ScanFilesystem(
			ctx,
			src,
			severity,
			trivyVersion,
		)

	LintReport, err := m.
		LintDockerfile(
			ctx,
			src.Directory(pathToDockerfile),
			nameDockerfile,
			threshold,
		)

	if err != nil {
		fmt.Printf("Error during linting: %v\n", err)
		return
	}

	fmt.Println("Static analysis report:", LintReport)

	fmt.Println("Static analysis report:", scanReport)
}

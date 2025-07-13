package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
)

func (m *KubernetesMicroservice) LintDockerfile(
	ctx context.Context,
	// the src directory
	src *dagger.Directory,
	// The dockerfile path
	// +optional
	dockerfile string,
	// The failure threshold
	// +optional
	threshold string,
) (string, error) {
	return dag.
		Docker().
		Lint(
			ctx,
			src,
			dagger.DockerLintOpts{
				Dockerfile: dockerfile,
				Threshold:  threshold,
			},
		)
}

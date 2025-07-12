package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
)

func (m *KubernetesMicroservice) StageImage(
	ctx context.Context,
	source string,
	target string,
	// +optional
	sourceRegistry string,
	// +optional
	sourceUsername string,
	// +optional
	sourcePassword *dagger.Secret,
	// +optional
	targetRegistry string,
	// +optional
	targetUsername string,
	// +optional
	targetPassword *dagger.Secret,
	// +optional
	// +flag
	// +default=false
	insecure bool,
	// +optional
	// +flag
	// +default="linux/amd64"
	platform string,
	// +optional
	dockerConfigSecret *dagger.Secret, //
) (string, error) {
	return dag.
		Crane().
		Copy(
			ctx,
			source,
			target,
			dagger.CraneCopyOpts{
				SourceRegistry:     sourceRegistry,
				SourceUsername:     sourceUsername,
				SourcePassword:     sourcePassword,
				TargetRegistry:     targetRegistry,
				TargetUsername:     targetUsername,
				TargetPassword:     targetPassword,
				Insecure:           insecure,
				Platform:           platform,
				DockerConfigSecret: dockerConfigSecret,
			},
		)
}

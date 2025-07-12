package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
)

func (m *KubernetesMicroservice) BakeImage(
	ctx context.Context,
	// The source directory
	src *dagger.Directory,
	// The repository name
	repositoryName string,
	// tag
	tag string,
	// The registry username
	// +optional
	registryUsername *dagger.Secret,
	// The registry password
	// +optional
	registryPassword *dagger.Secret,
	// The registry URL
	registryURL string,
	// The Dockerfile path
	// +optional
	// +default="Dockerfile"
	dockerfile string,
	// Set extra directories
	// +optional
	withDirectories []*dagger.Directory,
) (string, error) {
	return dag.
		Docker().
		BuildAndPush(
			ctx,
			src,
			repositoryName,
			tag,
			registryURL,
			dagger.DockerBuildAndPushOpts{
				Dockerfile:       dockerfile,
				WithDirectories:  withDirectories,
				RegistryUsername: registryUsername,
				RegistryPassword: registryPassword,
			},
		)
}

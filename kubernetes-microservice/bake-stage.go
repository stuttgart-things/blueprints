package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
	"fmt"
)

func (m *KubernetesMicroservice) RunBakeStage(
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

	imageID, error := m.
		BakeImage(
			ctx,
			src,
			repositoryName,
			tag,
			registryUsername,
			registryPassword,
			registryURL,
			dockerfile,
			withDirectories,
		)

	if error != nil {
		return "", fmt.Errorf("failed to build and push image: %w", error)
	}

	fmt.Println("Image ID:", imageID)

	return imageID, nil
}

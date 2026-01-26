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

// BakeAndScanImage builds, pushes, and scans an image, returning the scan result file
func (m *KubernetesMicroservice) BakeAndScanImage(
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
	// Severity levels to scan for
	// +optional
	// +default="HIGH,CRITICAL"
	scanSeverity string,
	// Trivy version to use for scanning
	// +optional
	// +default="0.64.1"
	trivyVersion string,
) (*dagger.File, error) {
	_, err := m.BakeImage(
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

	if err != nil {
		return nil, err
	}

	// Construct image reference directly since BuildAndPush returns a formatted message
	imageRef := registryURL + "/" + repositoryName + ":" + tag

	return m.ScanImage(
		ctx,
		imageRef,
		registryUsername,
		registryPassword,
		scanSeverity,
		trivyVersion,
	), nil
}

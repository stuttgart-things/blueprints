package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
)

func (m *KubernetesMicroservice) ScanImage(
	ctx context.Context,
	imageRef string, // Fully qualified image reference (e.g., "ttl.sh/my-repo:1.0.0")
	// +optional
	registryUser *dagger.Secret,
	// +optional
	registryPassword *dagger.Secret,
	// +optional
	// +default="HIGH,CRITICAL"
	severity string,
	// +optional
	// +default="0.64.1"
	trivyVersion string,
) *dagger.File {
	return dag.
		Trivy().
		ScanImage(
			imageRef,
			dagger.TrivyScanImageOpts{
				RegistryUser:     registryUser,
				RegistryPassword: registryPassword,
				Severity:         severity,
				TrivyVersion:     trivyVersion,
			},
		)
}

func (m *KubernetesMicroservice) ScanFilesystem(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	// +default="HIGH,CRITICAL"
	severity string,
	// +optional
	// +default="0.64.1"
	trivyVersion string,
) *dagger.File {
	return dag.
		Trivy().
		ScanFilesystem(
			src,
			dagger.TrivyScanFilesystemOpts{
				Severity:     severity,
				TrivyVersion: trivyVersion,
			},
		)
}

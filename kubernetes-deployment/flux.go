package main

import (
	"time"

	"dagger/kubernetes-deployment/internal/dagger"
)

// fluxCliContainer returns a container with the Flux CLI and kubeconfig mounted.
func fluxCliContainer(fluxCliImage string, kubeConfig *dagger.Secret) *dagger.Container {
	return dag.Container().
		From(fluxCliImage).
		WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig")
}

// parseTimeout parses a Go duration string (e.g. "5m", "300s") and returns the
// equivalent number of seconds. Falls back to 300 on error.
func parseTimeout(timeout string) int {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return 300
	}
	return int(d.Seconds())
}

package main

import (
	"strconv"
	"strings"
	"time"

	"dagger/flux/internal/dagger"
)

// clusterContainer returns a container with the kubeconfig mounted, for
// anything that reads or changes the cluster (flux CLI, kubectl).
//
// It carries a per-call CACHEBUST: the cluster state is not an input Dagger
// can see, so an exec with unchanged arguments is otherwise served from cache
// and never reaches the cluster. A bootstrap against a rebuilt or emptied
// cluster then reports every phase green and installs nothing (#182).
//
// The stamps only help if the function body runs at all: Dagger also caches
// a module function call by its arguments. Every exported function that
// reads or changes the cluster (or commits to git) is therefore marked
// +cache="never".
func clusterContainer(image string, kubeConfig *dagger.Secret) *dagger.Container {
	return dag.Container().
		From(image).
		WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig").
		WithEnvVariable("CACHEBUST", runStamp())
}

// runStamp returns a value unique to this call. See clusterContainer.
func runStamp() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// stampDir adds a run-stamp dot-file to a directory handed to another module
// (helm's HelmfileOperation), so its cluster exec is not served from cache.
// helmfile never reads the file. A nil src becomes a directory with only the
// stamp, which helm mounts where it would otherwise create an empty one.
func stampDir(src *dagger.Directory) *dagger.Directory {
	if src == nil {
		src = dag.Directory()
	}
	return src.WithNewFile(".dagger-run-stamp", runStamp())
}

// stampManifest appends a comment line to a manifest handed to another module
// (kubernetes' Kubectl apply), so each run applies a distinct file.
func stampManifest(content string) string {
	return strings.TrimRight(content, "\n") + "\n# flux bootstrap run: " + runStamp() + "\n"
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

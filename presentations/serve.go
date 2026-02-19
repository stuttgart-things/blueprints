package main

import (
	"context"
	"dagger/presentations/internal/dagger"
	"strconv"
)

func (m *Presentations) Serve(
	ctx context.Context,
	content *dagger.Directory,
	// The project name
	// +optional
	// +default="hugo"
	name string,
	// The base url to use
	// +optional
	// +default="0.0.0.0"
	baseURL string,
	// The Port to use
	// +optional
	// +default="1313"
	port string,
	// The Theme to use
	// +optional
	// +default="github.com/joshed-io/reveal-hugo"
	theme string,
) *dagger.Service {
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		portNumber = 1313
	}

	config := content.File("hugo.toml")

	siteDir := dag.Container().
		From("cgr.dev/chainguard/wolfi-base:latest").
		WithExec([]string{"apk", "add", "--no-cache", "hugo", "go", "git"}).
		WithEntrypoint([]string{"hugo"}).
		WithExec([]string{"hugo", "new", "site", "hugo"}).
		WithWorkdir("hugo").
		WithFile("hugo.toml", config).
		WithExec([]string{"hugo", "mod", "init", "hugo"}).
		WithExec([]string{"hugo", "mod", "get", theme}).
		WithExec([]string{"hugo", "mod", "tidy"}).
		WithExec([]string{"hugo", "mod", "vendor"}).
		Directory(".").
		WithFile("hugo.toml", config).
		WithDirectory("content", content)

	patchedContainer := dag.Container().
		From("cgr.dev/chainguard/wolfi-base:latest").
		WithExec([]string{"apk", "add", "--no-cache", "hugo", "go", "git", "sed"}).
		WithEntrypoint([]string{"hugo", "server", "--bind", "0.0.0.0", "--baseURL", baseURL, "--port", port}).
		WithMountedDirectory("/src", siteDir).
		WithWorkdir("/src").
		WithExec([]string{"sh", "-c", `head="/src/_vendor/github.com/joshed-io/reveal-hugo/layouts/partials/layout/head.html"; if [ -f "$head" ]; then sed -i 's/\.Site\.Author\.name/site.Title/g' "$head"; grep -q '\.Site\.Author\.name' "$head" && { echo 'failed to patch reveal-hugo head partial' >&2; exit 1; } || true; fi`}).
		WithExposedPort(portNumber)

	return patchedContainer.AsService()
}

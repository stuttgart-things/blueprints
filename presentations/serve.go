package main

import (
	"context"
	"dagger/presentations/internal/dagger"
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
	// Read hugo.toml from content directory
	config := content.File("hugo.toml")

	service := dag.Hugo().Serve(
		config,
		content,
		dagger.HugoServeOpts{
			Name:    name,
			BaseURL: baseURL,
			Port:    port,
			Theme:   theme,
		},
	)
	return service
}

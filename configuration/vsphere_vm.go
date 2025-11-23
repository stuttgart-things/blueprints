package main

import (
	"context"
	"dagger/configuration/internal/dagger"
)

func (v *Configuration) VsphereVm(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	configParameters,
	// +optional
	variablesFile,
	// +optional
	templatePaths string,
) (*dagger.Directory, error) {

	renderedTemplates := dag.Templating().Render(
		src,
		templatePaths,
		dagger.TemplatingRenderOpts{
			Variables:     configParameters,
			VariablesFile: variablesFile,
		},
	)

	return renderedTemplates, nil

}

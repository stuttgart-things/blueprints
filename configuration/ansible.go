package main

import (
	"context"
	"dagger/configuration/internal/dagger"
)

func (v *Configuration) CreateAnsibleRequirementFiles(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	templatePaths string,
	// Path to YAML or JSON file containing template data (supports HTTPS URLs)
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	dataFile string,
	// +optional
	// +default=false
	strictMode bool,
) (*dagger.Directory, error) {

	// RENDER TEMPLATES WITH DATA FROM FILE
	renderedRequirementsFile := dag.Templating().RenderFromFile(
		templatePaths,
		dataFile,
		dagger.TemplatingRenderFromFileOpts{
			Src:        src,
			StrictMode: strictMode,
		},
	)

	return renderedRequirementsFile, nil
}

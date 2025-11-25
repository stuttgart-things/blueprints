package main

import (
	"context"
	"dagger/configuration/internal/dagger"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenderReadme renders a README template with variables from multiple YAML files
// Multiple variables files are merged before rendering (comma-separated)
// Example usage:
//
//	dagger call render-readme \
//	  --src ./tests/configuration \
//	  --template-path README.md.tmpl \
//	  --data-files vm-ansible.yaml,additional-vars.yaml
func (v *Configuration) RenderVmReadme(
	ctx context.Context,
	// +optional
	// Source directory containing template and variables files
	src *dagger.Directory,
	// +optional
	// Configuration parameters as key=value pairs (comma-separated)
	configParameters string,
	// +optional
	// Path to template file
	// +default="README.md.tmpl"
	templatePath string,
	// Path(s) to YAML or JSON file(s) containing template data
	// Multiple files can be comma-separated and will be merged in order
	// +optional
	// +default="data.yaml"
	dataFiles string,
	// +optional
	// +default=false
	strictMode bool,
) (*dagger.Directory, error) {

	// Split the comma-separated file paths
	filePaths := strings.Split(dataFiles, ",")

	// Merge multiple YAML files
	mergedData := make(map[string]interface{})

	// If only one file and no configParameters, use RenderFromFile directly
	if len(filePaths) == 1 && configParameters == "" {
		renderedReadme := dag.Templating().RenderFromFile(
			templatePath,
			strings.TrimSpace(filePaths[0]),
			dagger.TemplatingRenderFromFileOpts{
				Src:        src,
				StrictMode: strictMode,
			},
		)
		return renderedReadme, nil
	}

	for _, filePath := range filePaths {
		filePath = strings.TrimSpace(filePath)

		// Read the file content
		fileContent, err := src.File(filePath).Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		// Parse YAML content
		var data map[string]interface{}
		if err := yaml.Unmarshal([]byte(fileContent), &data); err != nil {
			return nil, fmt.Errorf("failed to parse YAML from %s: %w", filePath, err)
		}

		// Merge data (later files override earlier ones)
		for key, value := range data {
			mergedData[key] = value
		}
	}

	// Parse and add configParameters if provided (highest priority - overrides data files)
	if configParameters != "" {
		params := strings.Split(configParameters, ",")
		for _, param := range params {
			param = strings.TrimSpace(param)
			if param == "" {
				continue
			}
			parts := strings.SplitN(param, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				mergedData[key] = value
			}
		}
	}

	// Convert merged data back to YAML
	mergedYAML, err := yaml.Marshal(mergedData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged data: %w", err)
	}

	// Create a new directory with the template and merged data
	// Start with the source directory (which contains the template)
	// and add the merged data file
	workingDir := src.WithNewFile("merged-data.yaml", string(mergedYAML))

	// Render template with merged data
	renderedReadme := dag.Templating().RenderFromFile(
		templatePath,
		"merged-data.yaml",
		dagger.TemplatingRenderFromFileOpts{
			Src:        workingDir,
			StrictMode: strictMode,
		},
	)

	return renderedReadme, nil
}

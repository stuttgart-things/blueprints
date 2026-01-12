package main

import (
	"context"
	"dagger/presentations/internal/dagger"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func (m *Presentations) AddContent(
	ctx context.Context,
	// the src directory
	src *dagger.Directory,
	// +optional
	// contextFile in yaml format
	presentationFile *dagger.File,
) (*dagger.Directory, error) {
	// Read and parse the YAML file
	yamlContent, err := presentationFile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read presentation file: %w", err)
	}

	// Define the structure to unmarshal YAML
	var presentation struct {
		Slides map[string]struct {
			Desc     string `yaml:"desc"`
			Duration int    `yaml:"duration"`
			Order    int    `yaml:"order"`
			File     string `yaml:"file"`
		} `yaml:"slides"`
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &presentation); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Create an empty directory to collect files
	outputDir := dag.Directory()

	// Process each slide
	for key, slide := range presentation.Slides {
		// Skip if file is not defined
		if slide.File == "" {
			continue
		}

		// Generate target filename: order-key.md
		targetName := fmt.Sprintf("%02d-%s.md", slide.Order, key)

		var fileContent *dagger.File

		// Check if the file is a URL or local path
		if strings.HasPrefix(slide.File, "http://") || strings.HasPrefix(slide.File, "https://") {
			// Download file from URL
			fileContent = dag.HTTP(slide.File)
		} else {
			// Get file from source directory
			fileContent = src.File(slide.File)
		}

		// Add file to output directory with the target name
		outputDir = outputDir.WithFile(targetName, fileContent)
	}

	return outputDir, nil
}

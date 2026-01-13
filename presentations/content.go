package main

import (
	"context"
	"dagger/presentations/internal/dagger"
	"fmt"
	"regexp"
	"strings"
	"unicode"

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

		// Read the file content
		content, err := fileContent.Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read slide file %s: %w", slide.File, err)
		}

		// Update or add weight in the front matter
		updatedContent := updateFrontMatterWeight(content, slide.Order)

		// Add file to output directory with the updated content
		outputDir = outputDir.WithNewFile(targetName, updatedContent)
	}

	return outputDir, nil
}

// updateFrontMatterWeight updates or adds the weight in the front matter
func updateFrontMatterWeight(content string, weight int) string {
	// Regular expression to find the front matter weight line
	// Matches "weight = <any number>" in the front matter
	weightPattern := regexp.MustCompile(`(?m)^(weight\s*=\s*)\d+(\s*)$`)

	// Regular expression to find the front matter boundaries
	// Matches the entire front matter section between +++ markers
	frontMatterPattern := regexp.MustCompile(`(?s)^\+\+\+\n(.*?)\n\+\+\+\n`)

	// Check if weight already exists
	if weightPattern.MatchString(content) {
		// Replace existing weight with the new one
		return weightPattern.ReplaceAllString(content, fmt.Sprintf("weight = %d", weight))
	}

	// Check if front matter exists
	matches := frontMatterPattern.FindStringSubmatch(content)
	if matches != nil {
		// Front matter exists, add weight line to it
		frontMatter := matches[1]
		updatedFrontMatter := fmt.Sprintf("%s\nweight = %d", frontMatter, weight)
		return strings.Replace(content, matches[1], updatedFrontMatter, 1)
	}

	// No front matter found, create new front matter with weight
	// Find where the content actually starts (skip any leading whitespace)
	contentStart := 0
	for i, r := range content {
		if !unicode.IsSpace(r) {
			contentStart = i
			break
		}
	}

	// Create new content with front matter
	return fmt.Sprintf("+++\nweight = %d\n+++\n%s", weight, content[contentStart:])
}

package main

import (
	"context"
	"dagger/presentations/internal/dagger"
	"fmt"
	"regexp"
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
			Desc            string `yaml:"desc"`
			Duration        int    `yaml:"duration"`
			Order           int    `yaml:"order"`
			File            string `yaml:"file"`
			Content         string `yaml:"content"`
			BackgroundColor string `yaml:"background-color"`
			Type            string `yaml:"type"`
			Transition      string `yaml:"transition"`
			TransitionSpeed string `yaml:"transition-speed"`
		} `yaml:"slides"`
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &presentation); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Create an empty directory to collect files
	outputDir := dag.Directory()

	// Process each slide
	for key, slide := range presentation.Slides {
		// Skip if neither file nor content is defined
		if slide.File == "" && slide.Content == "" {
			continue
		}

		// Generate target filename: order-key.md
		targetName := fmt.Sprintf("%02d-%s.md", slide.Order, key)

		var markdownContent string

		if slide.Content != "" {
			// Use inline content directly
			markdownContent = strings.TrimSpace(slide.Content)
		} else {
			// Read the content from the file (either local or URL)
			var content string

			if strings.HasPrefix(slide.File, "http://") || strings.HasPrefix(slide.File, "https://") {
				// Download file from URL
				httpFile := dag.HTTP(slide.File)
				content, err = httpFile.Contents(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to download slide from URL %s: %w", slide.File, err)
				}
			} else {
				// Get file from source directory
				file := src.File(slide.File)
				content, err = file.Contents(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to read slide file %s: %w", slide.File, err)
				}
			}

			// Extract just the markdown content (remove existing shortcodes and front matter)
			markdownContent = extractMarkdownContent(content)
		}

		// Set defaults if values are empty
		backgroundColor := slide.BackgroundColor
		if backgroundColor == "" {
			backgroundColor = "#FFFFFF" // default white
		}

		slideType := slide.Type
		if slideType == "" {
			slideType = "slide"
		}

		transition := slide.Transition
		if transition == "" {
			transition = "fade"
		}

		transitionSpeed := slide.TransitionSpeed
		if transitionSpeed == "" {
			transitionSpeed = "default"
		}

		// Generate the complete slide content
		// Weight is always (order + 1) * 10, so 0->10, 1->20, etc.
		weight := (slide.Order + 1) * 10
		generatedContent := fmt.Sprintf(`+++
weight = %d
+++

{{< slide id=%s background-color="%s" type="%s" transition="%s" transition-speed="%s" >}}

{{%% section %%}}

%s

{{%% /section %%}}`,
			weight,
			key,
			backgroundColor,
			slideType,
			transition,
			transitionSpeed,
			strings.TrimSpace(markdownContent),
		)

		// Add file to output directory
		outputDir = outputDir.WithNewFile(targetName, generatedContent)
	}

	return outputDir, nil
}

// extractMarkdownContent extracts just the markdown content, removing
// existing front matter and slide/section shortcodes
func extractMarkdownContent(content string) string {
	// Remove front matter (between +++ markers)
	frontMatterRegex := regexp.MustCompile(`(?s)^\+\+\+\n.*?\n\+\+\+\n`)
	content = frontMatterRegex.ReplaceAllString(content, "")

	// Remove slide shortcode (could be single or multi-line)
	// Match {{< slide ... >}} with any content in between
	slideShortcodeRegex := regexp.MustCompile(`(?s)\{\{<\s*slide[^>]*>\}\}`)
	content = slideShortcodeRegex.ReplaceAllString(content, "")

	// Remove section shortcodes
	content = strings.ReplaceAll(content, "{{% section %}}", "")
	content = strings.ReplaceAll(content, "{{% /section %}}", "")

	// Clean up extra empty lines
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" || len(cleaned) == 0 || strings.TrimSpace(cleaned[len(cleaned)-1]) != "" {
			cleaned = append(cleaned, line)
		}
	}

	// Trim leading/trailing whitespace
	result := strings.TrimSpace(strings.Join(cleaned, "\n"))
	return result
}

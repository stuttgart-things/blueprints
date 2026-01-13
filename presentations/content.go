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
			Desc            string `yaml:"desc"`
			Duration        int    `yaml:"duration"`
			Order           int    `yaml:"order"`
			File            string `yaml:"file"`
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
		// Skip if file is not defined
		if slide.File == "" {
			continue
		}

		// Generate target filename: order-key.md
		targetName := fmt.Sprintf("%02d-%s.md", slide.Order, key)

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
		markdownContent := extractMarkdownContent(content)

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
		generatedContent := fmt.Sprintf(`+++
weight = %d
+++

{{< slide id=%s background-color="%s" type="%s" transition="%s" transition-speed="%s" >}}

{{%% section %%}}

%s

{{%% /section %%}}`,
			slide.Order,
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

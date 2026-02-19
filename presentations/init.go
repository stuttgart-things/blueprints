package main

import (
	"context"
	"dagger/presentations/internal/dagger"
	"dagger/presentations/templates"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func (m *Presentations) Init(
	ctx context.Context,
	name string,
	// +optional
	defaultsFile *dagger.File,
	// +optional
	variablesFile *dagger.File,
	// +optional
	variables string,
) (*dagger.Directory, error) {

	packageName := name
	if packageName == "" {
		packageName = "presentation"
	}
	workingDir := "/" + packageName + "/"

	// Data to be used with the template
	data := map[string]interface{}{
		// hugo.toml
		"BaseURL":      "/",
		"LanguageCode": "en-us",
		"Title":        packageName,
		"Author": map[string]interface{}{
			"name":  "guest",
			"email": "guest@example.com",
		},
		"Themes": []string{
			"github.com/joshed-io/reveal-hugo",
		},
		"Module": map[string]interface{}{
			"Proxy":    "direct",
			"Vendored": true,
		},
		"Markup": map[string]interface{}{
			"Goldmark": map[string]interface{}{
				"Renderer": map[string]interface{}{
					"Unsafe": true,
				},
			},
		},
		"OutputFormats": map[string]interface{}{
			"Reveal": map[string]interface{}{
				"BaseName":  "index",
				"MediaType": "text/html",
				"IsHTML":    true,
			},
		},

		// reveal front-matter (content markdown)
		"Reveal": map[string]interface{}{
			"Outputs": []string{
				"Reveal",
			},
			"Hugo": map[string]interface{}{
				"History":         true,
				"SlideNumber":     true,
				"CustomTheme":     "reveal-hugo/themes/robot-lung.css",
				"Margin":          0.2,
				"Mermaid":         true,
				"HighlightTheme":  "color-brewer",
				"Transition":      "slide",
				"TransitionSpeed": "fast",
				"Templates": map[string]interface{}{
					"Hotpink": map[string]interface{}{
						"Class":      "hotpink",
						"Background": "#FF4081",
					},
				},
			},
		},

		// slide + section content
		"Slide": map[string]interface{}{
			"ID":              "agenda",
			"BackgroundColor": "#A2D8FF",
			"Type":            "slide",
			"Transition":      "zoom",
			"TransitionSpeed": "fast",
			"BackgroundImage": "https://artifacts.demo-infra.sthings-vsphere.labul.sva.de/images/stories2.png",
			"BackgroundSize":  "500px",
		},
		"Section": map[string]interface{}{
			"Spacer": `<br/>
<br/>
<br/>
<br/>
<br/>
<br/>`,
			"Content": "🚀 Container stories 🚀<br/>🔁 OCI artifacts everywhere 🔁",
		},
	}

	// Parse defaults from YAML file first (lowest priority)
	if defaultsFile != nil {
		content, err := defaultsFile.Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("read defaults file: %w", err)
		}

		var yamlData map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &yamlData); err != nil {
			return nil, fmt.Errorf("parse defaults YAML: %w", err)
		}

		// Merge defaults into data map
		for key, value := range yamlData {
			if value != nil {
				data[key] = value
			}
		}

		normalizeAuthor(data)
	}

	// Parse variables from YAML file (middle priority)
	if variablesFile != nil {
		content, err := variablesFile.Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("read variables file: %w", err)
		}

		var yamlData map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &yamlData); err != nil {
			return nil, fmt.Errorf("parse variables YAML: %w", err)
		}

		// Merge YAML data into data map
		for key, value := range yamlData {
			if value != nil {
				data[key] = value
			}
		}

		normalizeAuthor(data)
	}

	// Parse and merge additional variables from comma-separated string (highest priority)
	if variables != "" {
		// Parse variables with support for JSON values
		// Strategy: find key= patterns and extract value until next key= or end
		parseVariables(variables, data)
		normalizeAuthor(data)
	}

	normalizeAuthor(data)

	xplane := dag.Container().
		From("alpine:latest").
		WithWorkdir(workingDir)

	for _, tmpl := range templates.PresentationFiles {
		varsJSON, _ := json.Marshal(data)
		rendered, err := dag.Templating().RenderInline(
			ctx,
			tmpl.Template,
			dagger.TemplatingRenderInlineOpts{
				Variables:  string(varsJSON),
				StrictMode: true,
			},
		)

		if err != nil {
			return nil, fmt.Errorf("render template %s: %w", tmpl.Destination, err)
		}

		// Use the full destination path to preserve folder structure
		// WithNewFile automatically creates parent directories
		xplane = xplane.WithNewFile(tmpl.Destination, rendered)
	}

	return xplane.Directory(workingDir), nil
}

func normalizeAuthor(data map[string]interface{}) {
	author := map[string]interface{}{
		"name":  "guest",
		"email": "guest@example.com",
	}

	if value, ok := data["Author"]; ok {
		for key, mapped := range toStringMap(value) {
			switch strings.ToLower(key) {
			case "name":
				author["name"] = mapped
			case "email":
				author["email"] = mapped
			}
		}
	}

	if value, ok := data["author"]; ok {
		for key, mapped := range toStringMap(value) {
			switch strings.ToLower(key) {
			case "name":
				author["name"] = mapped
			case "email":
				author["email"] = mapped
			}
		}
	}

	data["Author"] = author
}

func toStringMap(value interface{}) map[string]interface{} {
	result := map[string]interface{}{}

	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			result[key] = item
		}
	case map[interface{}]interface{}:
		for key, item := range typed {
			result[fmt.Sprint(key)] = item
		}
	}

	return result
}

// parseVariables parses key=value pairs with support for JSON values
// Examples:
// - simple: "key1=value1,key2=value2"
// - with JSON: "key1=value1,functions=[{...}],key2=value2"
func parseVariables(variables string, data map[string]interface{}) {
	var i int
	for i < len(variables) {
		// Find the key
		eq := strings.Index(variables[i:], "=")
		if eq == -1 {
			break
		}

		key := strings.TrimSpace(variables[i : i+eq])
		i += eq + 1

		// Find the value - handle JSON arrays/objects specially
		var value string
		var isJSON bool

		if i < len(variables) && variables[i] == '[' {
			// JSON array - find matching closing bracket (include brackets in value)
			isJSON = true
			bracket := 1
			start := i
			i++ // skip opening bracket
			for i < len(variables) && bracket > 0 {
				if variables[i] == '[' {
					bracket++
				} else if variables[i] == ']' {
					bracket--
				} else if variables[i] == '"' {
					// Skip quoted strings to avoid counting brackets inside strings
					i++
					for i < len(variables) && variables[i] != '"' {
						if variables[i] == '\\' {
							i++
						}
						i++
					}
				}
				i++
			}
			// Include the brackets in the value
			value = variables[start:i]
		} else if i < len(variables) && variables[i] == '{' {
			// JSON object - find matching closing brace (include braces in value)
			isJSON = true
			brace := 1
			start := i
			i++ // skip opening brace
			for i < len(variables) && brace > 0 {
				if variables[i] == '{' {
					brace++
				} else if variables[i] == '}' {
					brace--
				} else if variables[i] == '"' {
					// Skip quoted strings
					i++
					for i < len(variables) && variables[i] != '"' {
						if variables[i] == '\\' {
							i++
						}
						i++
					}
				}
				i++
			}
			// Include the braces in the value
			value = variables[start:i]
		} else {
			// Regular value - read until next comma (which marks next key)
			start := i
			for i < len(variables) && variables[i] != ',' {
				i++
			}
			value = strings.TrimSpace(variables[start:i])
		}

		// Skip comma if present
		if i < len(variables) && variables[i] == ',' {
			i++
		}

		// Parse the value
		if isJSON {
			var jsonData interface{}
			if err := json.Unmarshal([]byte(value), &jsonData); err != nil {
				// If JSON parsing fails, treat as string
				data[key] = value
			} else {
				data[key] = jsonData
			}
		} else {
			data[key] = value
		}
	}
}

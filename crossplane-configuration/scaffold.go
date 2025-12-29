package main

import (
	"context"
	"dagger/crossplane-configuration/internal/dagger"
	"dagger/crossplane-configuration/templates"
	"encoding/json"
	"fmt"
	"strings"
)

func (m *CrossplaneConfiguration) Create(
	ctx context.Context,
	name string,
	// +optional
	variables string,
	// +optional
	dependencies string,
) (*dagger.Directory, error) {

	packageName := "test"
	workingDir := "/" + packageName + "/"

	// Data to be used with the template
	data := map[string]interface{}{
		"kind":              "default-kind",
		"maintainer":        "me@example.com",
		"source":            "https://github.com/stuttgart-things",
		"license":           "Apache-2.0",
		"claimKind":         "MyClaim",
		"crossplaneVersion": "2.13.0",
		"claimNamespace":    "default",
		"claimName":         "demo",
	}

	// Parse and merge additional variables from comma-separated string
	if variables != "" {
		// Parse variables with support for JSON values
		// Strategy: find key= patterns and extract value until next key= or end
		parseVariables(variables, data)
	}

	// Parse dependencies from comma-separated string
	var deps []map[string]string
	if dependencies != "" {
		pairs := strings.Split(dependencies, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				provider := strings.TrimSpace(parts[0])
				version := strings.TrimSpace(parts[1])
				deps = append(deps, map[string]string{
					"provider": provider,
					"version":  version,
				})
			}
		}
	} else {
		// Default dependencies if none provided
		deps = []map[string]string{
			{
				"provider": "xpkg.upbound.io/crossplane-contrib/provider-helm",
				"version":  ">=v0.19.0",
			},
			{
				"provider": "xpkg.upbound.io/crossplane-contrib/provider-kubernetes",
				"version":  ">=v0.14.1",
			},
		}
	}
	data["dependencies"] = deps

	xplane := dag.Container().
		From("alpine:latest").
		WithWorkdir(workingDir)

	for _, tmpl := range templates.PackageFiles {
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

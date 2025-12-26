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
		"source":            "https://example.com",
		"license":           "Apache-2.0",
		"claimKind":         "MyClaim",
		"crossplaneVersion": "1.13.0",
		"claimNamespace":    "default",
		"claimName":         "demo",
	}

	// Parse and merge additional variables from comma-separated string
	if variables != "" {
		pairs := strings.Split(variables, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				data[key] = value
			}
		}
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

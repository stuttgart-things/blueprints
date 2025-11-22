package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"

	"gopkg.in/yaml.v3"
)

type VmTshirtSize struct {
	CPU  int `yaml:"cpu"`
	RAM  int `yaml:"ram"`
	Disk int `yaml:"disk"`
}

type VmTshirtSizesConfig struct {
	VmTshirtSizes map[string]VmTshirtSize `yaml:"vm_tshirt_sizes"`
}

// TshirtSize returns a formatted string for VM configuration based on t-shirt size.
// This is a Dagger function that can be called via `dagger call tshirt-size`.
//
// Example:
//   dagger call tshirt-size --config-file=vm_tshirt_sizes.yaml --size=small
func (v *Vm) TshirtSize(
	ctx context.Context,
	// YAML file containing VM t-shirt sizes
	configFile *dagger.File,
	// T-shirt size: small, medium, large, or xlarge
	size string,
) (string, error) {
	// Read the file content from Dagger File
	content, err := configFile.Contents(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config VmTshirtSizesConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	if len(config.VmTshirtSizes) == 0 {
		return "", fmt.Errorf("no VM t-shirt sizes found in YAML file")
	}

	// Get the requested size
	vmConfig, exists := config.VmTshirtSizes[size]
	if !exists {
		var available []string
		for k := range config.VmTshirtSizes {
			available = append(available, k)
		}
		return "", fmt.Errorf("unknown VM t-shirt size: %s (available: %v)", size, available)
	}

	return fmt.Sprintf("cpu=%d,ram=%d,disk=%d", vmConfig.CPU, vmConfig.RAM, vmConfig.Disk), nil
}

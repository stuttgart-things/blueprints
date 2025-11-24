package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// analyzeConfigString parses a key=value configuration string and validates mandatory keys
// Example input: "name=demo-infra1,count=4,ram=8192,template=sthings-u24"
// mandatoryKeys: map of required keys
// Returns: parsed configuration as map[string]interface{}, error if mandatory keys are missing
func analyzeConfigString(configString string, mandatoryKeys map[string]bool) (map[string]interface{}, error) {
	// Parse the configuration string into a map
	configMap := make(map[string]interface{})
	pairs := strings.Split(configString, ",")

	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			configMap[key] = value
		}
	}

	// Validate that all mandatory keys are present
	var missingKeys []string
	for key := range mandatoryKeys {
		if _, exists := configMap[key]; !exists {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, fmt.Errorf("missing mandatory keys: %s", strings.Join(missingKeys, ", "))
	}

	return configMap, nil
}

// AnalyzeConfigString parses a key=value configuration string and validates mandatory keys
// Dagger-compatible wrapper that returns JSON string representation of the map
func (v *Configuration) AnalyzeConfigString(
	ctx context.Context,
	configString string,
	// Comma-separated list of mandatory keys (e.g., "name,template,disk")
	mandatoryKeys string,
) (string, error) {
	// Parse mandatory keys into a map
	mandatory := make(map[string]bool)
	if mandatoryKeys != "" {
		for _, key := range strings.Split(mandatoryKeys, ",") {
			mandatory[strings.TrimSpace(key)] = true
		}
	}

	// Analyze the config string
	configMap, err := analyzeConfigString(configString, mandatory)
	if err != nil {
		return "", err
	}

	// Return as JSON string for Dagger compatibility
	jsonBytes, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

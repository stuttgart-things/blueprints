package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// mergeAnsibleParameters merges parameters from YAML file and string
// String parameters have higher priority and override file parameters
func (m *Vm) mergeAnsibleParameters(ctx context.Context, file *dagger.File, strParams string) (string, error) {
	// Start with empty parameters
	fileParams := make(map[string]interface{})

	// Load parameters from YAML file if provided
	if file != nil {
		content, err := file.Contents(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to read parameters file: %w", err)
		}

		if err := yaml.Unmarshal([]byte(content), &fileParams); err != nil {
			return "", fmt.Errorf("failed to parse YAML parameters: %w", err)
		}
	}

	// If no string parameters, return YAML params as key=value pairs
	if strParams == "" {
		if len(fileParams) == 0 {
			return "", nil
		}
		return convertMapToAnsibleParams(fileParams), nil
	}

	// Parse string parameters (format: "key1=value1,key2=value2")
	strParamMap := parseStringParams(strParams)

	// Merge maps with string params taking priority
	mergedParams := mergeParamMaps(fileParams, strParamMap)

	// Convert to Ansible parameters format (key=value pairs)
	return convertMapToAnsibleParams(mergedParams), nil
}

// parseStringParams parses string parameters in format "key1=value1,key2=value2"
func parseStringParams(params string) map[string]interface{} {
	result := make(map[string]interface{})

	if params == "" {
		return result
	}

	// Split by comma to get key-value pairs
	pairs := strings.Split(params, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split by first equals sign to handle values with equals signs
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			// Try to detect if value should be converted to int or bool
			convertedValue := convertStringValue(value)
			result[key] = convertedValue
		}
	}

	return result
}

// convertStringValue attempts to convert string values to appropriate types
func convertStringValue(s string) interface{} {
	// Try boolean
	sLower := strings.ToLower(s)
	if sLower == "true" {
		return true
	}
	if sLower == "false" {
		return false
	}

	// Try integer
	if intVal, err := strconv.Atoi(s); err == nil {
		return intVal
	}

	// Try float
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return floatVal
	}

	// Return as string
	return s
}

// mergeParamMaps merges two parameter maps with overrideMap taking priority
func mergeParamMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base map
	for k, v := range base {
		result[k] = v
	}

	// Apply overrides
	for k, v := range override {
		result[k] = v
	}

	return result
}

// convertMapToAnsibleParams converts a map to Ansible parameters string format
// Returns "key1=value1 key2=value2" format (space-separated)
func convertMapToAnsibleParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}

	var pairs []string
	for k, v := range params {
		// Format the value appropriately
		switch val := v.(type) {
		case string:
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, val))
		case bool:
			pairs = append(pairs, fmt.Sprintf("%s=%v", k, val))
		case int, int32, int64, float32, float64:
			pairs = append(pairs, fmt.Sprintf("%s=%v", k, val))
		default:
			// For complex types, convert to JSON string
			jsonBytes, err := json.Marshal(val)
			if err != nil {
				// Fallback to string representation
				pairs = append(pairs, fmt.Sprintf("%s=%v", k, val))
			} else {
				// Escape single quotes in JSON for shell
				jsonStr := strings.ReplaceAll(string(jsonBytes), "'", "'\"'\"'")
				pairs = append(pairs, fmt.Sprintf("%s='%s'", k, jsonStr))
			}
		}
	}

	// Return space-separated pairs
	return strings.Join(pairs, " ")
}

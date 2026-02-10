package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"strings"
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

func (m *Vm) ExecuteAnsible(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	playbooks string,
	// +optional
	requirements *dagger.File,
	// +optional
	inventory *dagger.File,
	// Comma-separated list of hosts (e.g., "192.168.1.10,192.168.1.11")
	// Used to generate inventory if inventory file is not provided
	// +optional
	hosts string,
	// +optional
	parameters string,
	// Path to a YAML file containing parameters (lower priority)
	// +optional
	parametersFile *dagger.File,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// +optional
	sshUser *dagger.Secret,
	// +optional
	sshPassword *dagger.Secret,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	requirementsTemplate string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	requirementsData string,
	// Inventory type: "simple" (default [all] group) or "cluster" (master/worker groups)
	// +optional
	// +default="simple"
	inventoryType string,
) (bool, error) {

	if src == nil {
		src = dag.Directory()
	}

	// IF NO INVENTORY FILE PROVIDED BUT HOSTS ARE GIVEN, CREATE INVENTORY
	if inventory == nil && hosts != "" {
		var inventoryContent string
		var err error

		if inventoryType == "cluster" {
			// Create cluster inventory with master/worker groups
			inventoryContent, err = CreateClusterAnsibleInventoryFromHosts(hosts)
			if err != nil {
				return false, err
			}
		} else {
			// Create simple inventory with [all] group
			inventoryContent = "[all]\n"
			for _, host := range splitHosts(hosts) {
				inventoryContent += host + "\n"
			}
		}

		// Create inventory file from content
		inventory = dag.Directory().
			WithNewFile("inventory.ini", inventoryContent).
			File("inventory.ini")
	}

	// IF NO REQUIREMENTS FILE PROVIDED, GENERATE IT USING CONFIGURATION MODULE
	if requirements == nil {
		generatedRequirements := dag.Configuration().CreateAnsibleRequirementFiles(
			dagger.ConfigurationCreateAnsibleRequirementFilesOpts{
				Src:           src,
				TemplatePaths: requirementsTemplate,
				DataFile:      requirementsData,
				StrictMode:    false,
			},
		)
		// Extract requirements.yaml from generated directory
		requirements = generatedRequirements.File("requirements.yaml")
	}

	// MERGE PARAMETERS FROM FILE AND STRING (STRING HAS HIGHER PRIORITY)
	finalParameters, err := m.mergeAnsibleParameters(ctx, parametersFile, parameters)
	if err != nil {
		return false, fmt.Errorf("failed to merge parameters: %w", err)
	}

	// EXECUTE ANSIBLE USING DAGGER'S ANSIBLE MODULE
	return dag.Ansible().Execute(
		ctx,
		playbooks,
		dagger.AnsibleExecuteOpts{
			Src:            src,
			Inventory:      inventory,
			Parameters:     finalParameters,
			VaultAppRoleID: vaultAppRoleID,
			VaultSecretID:  vaultSecretID,
			VaultURL:       vaultURL,
			Requirements:   requirements,
			SSHUser:        sshUser,
			SSHPassword:    sshPassword,
		})
}

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

// splitHosts splits comma-separated hosts and trims whitespace
func splitHosts(hosts string) []string {
	var result []string
	for _, host := range strings.Split(hosts, ",") {
		host = strings.TrimSpace(host)
		if host != "" {
			result = append(result, host)
		}
	}
	return result
}

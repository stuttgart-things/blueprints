package main

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"
)

// TerraformOutput represents the structure of the input JSON
type TerraformOutput struct {
	IP struct {
		Value [][]string `json:"value"`
	} `json:"ip"`
}

// CreateAnsibleInventory converts Terraform output to Ansible YAML
func CreateAnsibleInventory(jsonStr string) (string, error) {
	// Parse JSON input
	var tfOutput TerraformOutput
	err := json.Unmarshal([]byte(jsonStr), &tfOutput)
	if err != nil {
		return "", fmt.Errorf("JSON parse error: %w", err)
	}

	// Extract IP addresses from nested structure
	ips := []string{}
	for _, outer := range tfOutput.IP.Value {
		if len(outer) > 0 {
			ips = append(ips, outer[0])
		}
	}

	// Build Ansible inventory structure
	inventory := map[string]interface{}{
		"all": map[string]interface{}{
			"hosts": make(map[string]interface{}),
		},
	}
	hosts := inventory["all"].(map[string]interface{})["hosts"].(map[string]interface{})
	for _, ip := range ips {
		hosts[ip] = struct{}{} // Empty struct for valid YAML with no variables
	}

	// Generate YAML output
	yamlData, err := yaml.Marshal(inventory)
	if err != nil {
		return "", fmt.Errorf("YAML generation error: %w", err)
	}

	return string(yamlData), nil
}

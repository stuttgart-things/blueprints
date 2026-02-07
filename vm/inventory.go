package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

type Host struct {
	FQDN string
}

type TemplateData struct {
	Hosts []Host
}

// TerraformOutput represents the structure of the input JSON.
type TerraformOutput struct {
	IP struct {
		Value StringList `json:"value"`
	} `json:"ip"`
}

// StringList is a helper that unmarshals either ["10.0.0.1", "10.0.0.2"] or [["10.0.0.1"], ["10.0.0.2"]]
// and also tolerates a single string value "10.0.0.1".
type StringList struct {
	Values []string
}

func (sl *StringList) UnmarshalJSON(b []byte) error {
	// Try flat array of strings
	var flat []string
	if err := json.Unmarshal(b, &flat); err == nil {
		sl.Values = flat
		return nil
	}

	// Try nested array of arrays of strings
	var nested [][]string
	if err := json.Unmarshal(b, &nested); err == nil {
		for _, inner := range nested {
			sl.Values = append(sl.Values, inner...)
		}
		return nil
	}

	// Try single string
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		sl.Values = []string{single}
		return nil
	}

	return fmt.Errorf("unsupported ip.value JSON shape: %s", string(b))
}

const clusterTemplate = `{{- if eq (len .Hosts) 1 }}
# SINGLENODE-CLUSTER
[initial_master_node]
{{- $host := index .Hosts 0 }}
{{ $host.FQDN }} ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[additional_master_nodes]

[workers]

{{- else if eq (len .Hosts) 3 }}
# 3-NODE CLUSTER
[initial_master_node]
{{- $first := index .Hosts 0 }}
{{ $first.FQDN }} ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[additional_master_nodes]
{{- range $i, $host := .Hosts }}
  {{- if and (gt $i 0) (le $i 2) }}
{{ $host.FQDN }} ansible_ssh_common_args='-o StrictHostKeyChecking=no'
  {{- end }}
{{- end }}

[workers]

{{- else }}
# LARGE CLUSTER (4+ NODES)
[initial_master_node]
{{- $first := index .Hosts 0 }}
{{ $first.FQDN }} ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[additional_master_nodes]
{{- range $i, $host := .Hosts }}
  {{- if and (gt $i 0) (le $i 2) }}
{{ $host.FQDN }} ansible_ssh_common_args='-o StrictHostKeyChecking=no'
  {{- end }}
{{- end }}

[workers]
{{- range $i, $host := .Hosts }}
  {{- if gt $i 2 }}
{{ $host.FQDN }} ansible_ssh_common_args='-o StrictHostKeyChecking=no'
  {{- end }}
{{- end }}
{{- end }}
`

func ParseIPsFromTfOutput(terraformVMOutput string) ([]string, error) {
	var tfOutput TerraformOutput
	err := json.Unmarshal([]byte(terraformVMOutput), &tfOutput)
	if err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	return tfOutput.IP.Value.Values, nil
}

// CreateDefaultAnsibleInventory converts Terraform output to Ansible YAML.
func CreateDefaultAnsibleInventory(terraformVMOutput string) (string, error) {

	ips, err := ParseIPsFromTfOutput(terraformVMOutput)
	if err != nil {
		return "", fmt.Errorf("FAILED TO PARSE IPS FROM TERRAFORM OUTPUT: %w", err)
	}

	// BUILD ANSIBLE INVENTORY STRUCTURE
	inventory := map[string]interface{}{
		"all": map[string]interface{}{
			"hosts": make(map[string]interface{}),
		},
	}

	hosts := inventory["all"].(map[string]interface{})["hosts"].(map[string]interface{})
	for _, ip := range ips {
		hosts[ip] = struct{}{} // Empty struct for valid YAML with no variables
	}

	// GENERATE YAML OUTPUT
	yamlData, err := yaml.Marshal(inventory)
	if err != nil {
		return "", fmt.Errorf("YAML generation error: %w", err)
	}

	return string(yamlData), nil
}

// CreateClusterAnsibleInventory converts Terraform output to an Ansible inventory in INI format.
func CreateClusterAnsibleInventory(terraformVMOutput string) (string, error) {

	ips, err := ParseIPsFromTfOutput(terraformVMOutput)
	if err != nil {
		return "", fmt.Errorf("FAILED TO PARSE IPS FROM TERRAFORM OUTPUT: %w", err)
	}

	var hosts []Host
	for _, ip := range ips {
		hosts = append(hosts, Host{FQDN: ip})
	}

	data := TemplateData{Hosts: hosts}

	tmpl, err := template.New("inventory").Parse(clusterTemplate)
	if err != nil {
		panic(err)
	}

	if err := tmpl.Execute(os.Stdout, data); err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// CreateClusterAnsibleInventoryFromHosts converts comma-separated hosts to cluster inventory
func CreateClusterAnsibleInventoryFromHosts(hostsString string) (string, error) {
	var hosts []Host

	// Split comma-separated hosts
	for _, host := range splitHostsForInventory(hostsString) {
		hosts = append(hosts, Host{FQDN: host})
	}

	data := TemplateData{Hosts: hosts}

	tmpl, err := template.New("inventory").Parse(clusterTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// splitHostsForInventory splits comma-separated hosts (helper for inventory.go)
func splitHostsForInventory(hosts string) []string {
	var result []string
	for _, host := range strings.Split(hosts, ",") {
		host = strings.TrimSpace(host)
		if host != "" {
			result = append(result, host)
		}
	}
	return result
}

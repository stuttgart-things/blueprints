package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"strings"
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
) (bool, error) {

	// IF NO INVENTORY FILE PROVIDED BUT HOSTS ARE GIVEN, CREATE SIMPLE INVENTORY
	if inventory == nil && hosts != "" {
		inventoryContent := "[all]\n"
		// Split comma-separated hosts and add to inventory
		for _, host := range splitHosts(hosts) {
			inventoryContent += host + "\n"
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

	// EXECUTE ANSIBLE USING DAGGER'S ANSIBLE MODULE
	return dag.Ansible().Execute(
		ctx,
		playbooks,
		dagger.AnsibleExecuteOpts{
			Src:            src,
			Inventory:      inventory,
			Parameters:     parameters,
			VaultAppRoleID: vaultAppRoleID,
			VaultSecretID:  vaultSecretID,
			VaultURL:       vaultURL,
			Requirements:   requirements,
			SSHUser:        sshUser,
			SSHPassword:    sshPassword,
		})
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

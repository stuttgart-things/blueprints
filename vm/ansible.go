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

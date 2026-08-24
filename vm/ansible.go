package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
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
	// Extra environment for the Ansible container, as a secret in dotenv format
	// (NAME=value per line). Needed by playbooks that resolve values with
	// lookup('env', ...), which is evaluated on the controller, not the target --
	// e.g. sthings.container.kind_machinery reads SOPS_AGE_KEY that way.
	// +optional
	envSecrets *dagger.Secret,
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
	// Any value that changes between runs -- a timestamp, a CI run id. Threaded
	// into CreateAnsibleRequirementFiles, where it forces a fresh fetch of the
	// remote requirements instead of a cached render. Leave empty to keep the
	// previous behaviour.
	// +optional
	// +default=""
	cacheBuster string,
) (bool, error) {

	prep, err := m.prepareAnsibleExecution(ctx, src, requirements, inventory, hosts, parameters, parametersFile, requirementsTemplate, requirementsData, inventoryType, cacheBuster)
	if err != nil {
		return false, err
	}

	// EXECUTE ANSIBLE USING DAGGER'S ANSIBLE MODULE
	return dag.Ansible().Execute(
		ctx,
		playbooks,
		dagger.AnsibleExecuteOpts{
			Src:            prep.src,
			Inventory:      prep.inventory,
			Parameters:     prep.parameters,
			VaultAppRoleID: vaultAppRoleID,
			VaultSecretID:  vaultSecretID,
			VaultURL:       vaultURL,
			Requirements:   prep.requirements,
			SSHUser:        sshUser,
			SSHPassword:    sshPassword,
			EnvSecrets:     envSecrets,
		})
}

// ansiblePrepResult holds the prepared inputs for Ansible execution.
type ansiblePrepResult struct {
	src          *dagger.Directory
	inventory    *dagger.File
	requirements *dagger.File
	parameters   string
}

// prepareAnsibleExecution handles inventory generation, requirements generation,
// and parameter merging — shared logic between ExecuteAnsible and ExecuteAnsibleWithExport.
func (m *Vm) prepareAnsibleExecution(
	ctx context.Context,
	src *dagger.Directory,
	requirements *dagger.File,
	inventory *dagger.File,
	hosts string,
	parameters string,
	parametersFile *dagger.File,
	requirementsTemplate string,
	requirementsData string,
	inventoryType string,
	cacheBuster string,
) (*ansiblePrepResult, error) {

	if src == nil {
		src = dag.Directory()
	}

	// IF NO INVENTORY FILE PROVIDED BUT HOSTS ARE GIVEN, CREATE INVENTORY
	if inventory == nil && hosts != "" {
		var inventoryContent string
		var err error

		if inventoryType == "cluster" {
			inventoryContent, err = CreateClusterAnsibleInventoryFromHosts(hosts)
			if err != nil {
				return nil, err
			}
		} else {
			inventoryContent = "[all]\n"
			for _, host := range splitHosts(hosts) {
				inventoryContent += host + "\n"
			}
		}

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
				CacheBuster:   cacheBuster,
			},
		)
		requirements = generatedRequirements.File("requirements.yaml")
	}

	// MERGE PARAMETERS FROM FILE AND STRING (STRING HAS HIGHER PRIORITY)
	finalParameters, err := m.mergeAnsibleParameters(ctx, parametersFile, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to merge parameters: %w", err)
	}

	return &ansiblePrepResult{
		src:          src,
		inventory:    inventory,
		requirements: requirements,
		parameters:   finalParameters,
	}, nil
}

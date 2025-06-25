// Vm module provides a comprehensive workflow to manage virtual machine lifecycle
// and configuration using Terraform and Ansible, integrated with secure secret
// management via Vault and SOPS.
//
// This generated module was created with dagger init as a starting point for VM-related
// operations. It demonstrates key DevOps tasks such as decrypting secrets, applying
// Terraform infrastructure changes, generating dynamic Ansible inventories, and
// executing Ansible playbooks to configure VMs. The module is designed to be flexible
// and extensible to support your infrastructure automation needs.
//
// The primary function Bake orchestrates this workflow, accepting Terraform directories,
// encrypted files, Vault credentials, and Ansible parameters as inputs. It optionally
// decrypts SOPS-encrypted configuration files before applying Terraform operations,
// then parses Terraform outputs to generate inventory files for Ansible. It supports
// multiple inventory types and allows you to specify Ansible playbooks and credentials.
//
// This module can be invoked from the Dagger CLI or programmatically via the SDK,
// making it suitable for integrating into CI/CD pipelines, GitOps workflows, or
// custom operator/controller logic.
//
// Future enhancements planned include:
// - Rendering manifests or configs to branches/PRs for GitOps-style deployments
// - Seamless integration with SOPS for secret management and decryption
// - Advanced Terraform execution and output parsing features
// - Enhanced Ansible inventory generation and execution customization
// - VM testing and validation steps post-provisioning
// - Automated merge requests/PR handling post-deployment
//
// This documentation serves both as a high-level overview and a detailed guide
// to the module’s capabilities and intended use cases.

// IDEA
// RENDER TO BRANCH/PR -> IMPLEMENT LATER
// SOPS DECRYPT
// TERRAFORM EXECUTE
// TERRAFORM OUTPUT
// ANSIBLE INVENTORY CREATION
// ANSIBLE EXECUTION
// TEST VM
// MERGE PR

package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
)

type Vm struct {
	BaseImage string
}

func (v *Vm) Bake(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	// +default="apply"
	operation string,
	// +optional
	encryptedFile *dagger.File,
	// +optional
	sopsKey *dagger.Secret,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultUrl *dagger.Secret,
	// +optional
	ansibleInventoryTemplate *dagger.File,
	// +optional
	ansiblePlaybooks string,
	// +optional
	ansibleRequirementsFile *dagger.File,
	// +optional
	ansibleUser *dagger.Secret,
	// +optional
	ansiblePassword *dagger.Secret,
	// +optional
	ansibleParameters string,
	// +optional
	// +default="default"
	ansibleInventoryType string,
) (*dagger.Directory, error) {
	workDir := "/src"

	// INIT WORKING CONTAINER
	ctr, err := v.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}
	ctr = ctr.WithDirectory(workDir, terraformDir).WithWorkdir(workDir)

	// OPTIONAL SOPS DECRYPTION
	if encryptedFile != nil {
		decryptedContent, err := dag.Sops().DecryptSops(ctx, sopsKey, encryptedFile)
		if err != nil {
			return nil, fmt.Errorf("decrypting sops file failed: %w", err)
		}
		ctr = ctr.WithNewFile(fmt.Sprintf("%s/terraform.tfvars.json", workDir), decryptedContent)
	}

	// RUN TERRAFORM
	terraformDirResult := dag.Terraform().Execute(ctr.Directory(workDir), dagger.TerraformExecuteOpts{
		Operation: operation,
	})

	// GET TERRAFORM OUTPUT
	tfOutput, err := dag.Terraform().Output(ctx, terraformDirResult)
	if err != nil {
		return nil, fmt.Errorf("getting terraform output failed: %w", err)
	}

	// GENERATE ANSIBLE INVENTORY
	var inventory string
	switch ansibleInventoryType {
	case "default":
		inventory, err = CreateDefaultAnsibleInventory(tfOutput)
	case "cluster":
		inventory, err = CreateClusterAnsibleInventory(tfOutput)
	default:
		err = fmt.Errorf("unsupported inventory type: %s", ansibleInventoryType)
	}
	if err != nil {
		return nil, fmt.Errorf("creating inventory failed: %w", err)
	}

	// WRITE INVENTORY TO CONTAINER
	ctr = ctr.WithNewFile(fmt.Sprintf("%s/inventory.yaml", workDir), inventory)

	// RUN ANSIBLE
	dag.Ansible().Execute(ctx, ansiblePlaybooks, dagger.AnsibleExecuteOpts{
		Src:            terraformDirResult,
		Inventory:      terraformDirResult.File("inventory.yaml"),
		Parameters:     ansibleParameters,
		VaultAppRoleID: vaultAppRoleID,
		VaultSecretID:  vaultSecretID,
		VaultURL:       vaultUrl,
		Requirements:   ansibleRequirementsFile,
		SSHUser:        ansibleUser,
		SSHPassword:    ansiblePassword,
	})

	// RETURN UPDATED WORKDIR WITH INVENTORY
	return terraformDirResult, nil
}

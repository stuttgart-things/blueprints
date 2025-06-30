package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
)

func (v *Vm) BakeLocal(
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
	vaultURL *dagger.Secret,
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
		VaultURL:       vaultURL,
		Requirements:   ansibleRequirementsFile,
		SSHUser:        ansibleUser,
		SSHPassword:    ansiblePassword,
	})

	// RETURN UPDATED WORKDIR WITH INVENTORY
	return terraformDirResult, nil
}

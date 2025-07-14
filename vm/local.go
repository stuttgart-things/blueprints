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
	// e.g., "cpu=4,ram=4096,storage=100"
	variables string,
	// +optional
	encryptedFile *dagger.File,
	// +optional
	sopsKey *dagger.Secret,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// vaultToken
	// +optional
	vaultToken *dagger.Secret,
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
		decryptedContent, err := v.
			DecryptSops(
				ctx,
				sopsKey,
				encryptedFile,
			)
		if err != nil {
			return nil, fmt.Errorf("decrypting sops file failed: %w", err)
		}
		ctr = ctr.
			WithNewFile(
				fmt.Sprintf("%s/terraform.tfvars.json", workDir),
				decryptedContent)
	}

	// RUN TERRAFORM
	terraformDirResult, error := v.
		ExecuteTerraform(
			ctx,
			ctr.Directory(workDir),
			operation,
			variables,
			vaultAppRoleID,
			vaultSecretID,
			vaultToken,
		)

	if error != nil {
		return nil, fmt.Errorf("running terraform failed: %w", error)
	}

	// IF OPERATION IS NOT APPLY, RETURN EARLY
	if operation != "apply" {
		return terraformDirResult, nil
	}

	// GET TERRAFORM OUTPUT
	tfOutput, err := dag.
		Terraform().
		Output(
			ctx,
			terraformDirResult,
		)

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

	// WRITE INVENTORY TO terraformDirResult
	terraformDirResult = terraformDirResult.WithNewFile("inventory.yaml", inventory)

	// RUN ANSIBLE
	ansibleSuccess, err := v.
		ExecuteAnsible(
			ctx,
			terraformDirResult,
			ansiblePlaybooks,
			ansibleRequirementsFile,
			terraformDirResult.File("inventory.yaml"),
			ansibleParameters,
			vaultAppRoleID,
			vaultSecretID,
			vaultURL,
			ansibleUser,
			ansiblePassword,
		)
	if err != nil {
		return nil, fmt.Errorf("running ansible failed: %w", err)
	}

	if !ansibleSuccess {
		return nil, fmt.Errorf("ansible execution failed")
	}

	// RETURN UPDATED WORKDIR WITH INVENTORY
	return terraformDirResult, nil
}

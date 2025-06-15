// A generated module for Vm functions
//
// This module has been generated via dagger init and serves as a reference to
// basic module structure as you get started with Dagger.
//
// Two functions have been pre-created. You can modify, delete, or add to them,
// as needed. They demonstrate usage of arguments and return types using simple
// echo and grep commands. The functions can be called from the dagger CLI or
// from one of the SDKs.
//
// The first line in this comment block is a short description line and the
// rest is a long description with more detail on the module's purpose or usage,
// if appropriate. All modules should have a short description.

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
) (*dagger.Directory, error) {

	workDir := "/src"

	// INIT WORKING CONTAINER
	ctr, err := v.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}
	ctr = ctr.WithDirectory(workDir, terraformDir).WithWorkdir(workDir)

	if encryptedFile != nil {

		// DECRYPT TO STRING
		decryptedContent, err := dag.Sops().DecryptSops(
			ctx,
			sopsKey,
			encryptedFile,
		)
		if err != nil {
			return nil, fmt.Errorf("decrypting sops file failed: %w", err)
		}

		// Write the decrypted content into the container
		ctr = ctr.WithNewFile(fmt.Sprintf("%s/terraform.tfvars.json", workDir), decryptedContent)
	}

	// Extract updated directory from container
	updatedDir := ctr.Directory(workDir)

	dir := dag.Terraform().Execute(updatedDir, dagger.TerraformExecuteOpts{
		Operation:     "apply",
		EncryptedFile: nil,
	})

	return dir, nil

}

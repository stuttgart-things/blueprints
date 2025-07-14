package main

import (
	"context"
	"dagger/vm/internal/dagger"
)

func (m *Vm) ExecuteTerraform(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	// +default="apply"
	operation string,
	// +optional
	// e.g., "cpu=4,ram=4096,storage=100"
	variables string,
	// vaultRoleID
	// +optional
	vaultRoleID *dagger.Secret,
	// vaultSecretID
	// +optional
	vaultSecretID *dagger.Secret,
	// vaultToken
	// +optional
	vaultToken *dagger.Secret,
) (*dagger.Directory, error) {
	// RUN TERRAFORM
	terraformDirResult := dag.
		Terraform().
		Execute(
			terraformDir,
			dagger.TerraformExecuteOpts{
				Operation:     operation,
				Variables:     variables,
				VaultRoleID:   vaultRoleID,
				VaultSecretID: vaultSecretID,
				VaultToken:    vaultToken,
			})

	return terraformDirResult, nil
}

func (m *Vm) OutputTerraformRun(
	ctx context.Context,
	terraformDir *dagger.Directory,
) (string, error) {
	return dag.
		Terraform().
		Output(
			ctx,
			terraformDir,
		)
}

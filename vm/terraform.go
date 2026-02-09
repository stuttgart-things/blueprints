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
	// AWS S3/MinIO credentials
	// +optional
	awsAccessKeyID *dagger.Secret,
	// +optional
	awsSecretAccessKey *dagger.Secret,
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
				AwsAccessKeyID:    awsAccessKeyID,
				AwsSecretAccessKey: awsSecretAccessKey,
				VaultRoleID:   vaultRoleID,
				VaultSecretID: vaultSecretID,
				VaultToken:    vaultToken,
			})

	return terraformDirResult, nil
}

func (m *Vm) OutputTerraformRun(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	awsAccessKeyID *dagger.Secret,
	// +optional
	awsSecretAccessKey *dagger.Secret,
) (string, error) {
	return dag.
		Terraform().
		Output(
			ctx,
			terraformDir,
			dagger.TerraformOutputOpts{
				AwsAccessKeyID:     awsAccessKeyID,
				AwsSecretAccessKey: awsSecretAccessKey,
			},
		)
}

// OutputTerraformRunWithCreds runs `terraform output --json` with AWS credentials
// for remote S3/MinIO backends. This is now just an alias for OutputTerraformRun.
func (m *Vm) OutputTerraformRunWithCreds(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	awsAccessKeyID *dagger.Secret,
	// +optional
	awsSecretAccessKey *dagger.Secret,
) (string, error) {
	return m.OutputTerraformRun(ctx, terraformDir, awsAccessKeyID, awsSecretAccessKey)
}

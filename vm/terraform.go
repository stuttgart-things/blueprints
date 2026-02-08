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
) (string, error) {
	return dag.
		Terraform().
		Output(
			ctx,
			terraformDir,
		)
}

// OutputTerraformRunWithCreds runs `terraform output --json` in a container
// with AWS credentials injected, to support remote S3/MinIO backends.
func (m *Vm) OutputTerraformRunWithCreds(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	awsAccessKeyID *dagger.Secret,
	// +optional
	awsSecretAccessKey *dagger.Secret,
) (string, error) {
	// Use an image that has terraform preinstalled to avoid extra setup.
	ctr := dag.Container().
		From("hashicorp/terraform:light")

	// Inject AWS creds for S3-compatible backend
	if awsAccessKeyID != nil { // pragma: allowlist secret
		ctr = ctr.WithSecretVariable("AWS_ACCESS_KEY_ID", awsAccessKeyID)
	}
	if awsSecretAccessKey != nil { // pragma: allowlist secret
		ctr = ctr.WithSecretVariable("AWS_SECRET_ACCESS_KEY", awsSecretAccessKey)
	}
	// Prevent attempts to use IMDS, which can cause noisy errors in CI
	ctr = ctr.WithEnvVariable("AWS_EC2_METADATA_DISABLED", "true")

	// Mount terraform directory and execute output command
	ctr = ctr.
		WithDirectory("/src", terraformDir).
		WithWorkdir("/src").
		WithExec([]string{"terraform", "output", "--json"})

	return ctr.Stdout(ctx)
}

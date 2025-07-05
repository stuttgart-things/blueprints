package main

import (
	"context"
	"dagger/vmtemplate/internal/dagger"
	"fmt"
)

func (m *Vmtemplate) CreateTestVM(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	// +default="apply"
	operation string,
	// +optional
	// e.g., "cpu=4,ram=4096,storage=100"
	variables string,
) {
	// RUN TERRAFORM
	terraformDirResult := dag.Terraform().Execute(
		terraformDir,
		dagger.TerraformExecuteOpts{
			Operation: operation,
			Variables: variables,
		})

	fmt.Println(terraformDirResult)
}

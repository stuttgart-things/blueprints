package main

import (
	"context"
	"dagger/vmtemplate/internal/dagger"
	"fmt"
)

func (m *Vmtemplate) RunVsphereWorkflow(
	ctx context.Context,
	// The Packer configuration directory
	// +optional
	packerConfigDir *dagger.Directory,
	// The Packer configuration file
	packerConfig string,
	// The Packer version to use
	// +optional
	// +default="1.13.1"
	packerVersion string,
	// The Packer arch
	// +optional
	// +default="linux_amd64"
	arch string,
	// If true, only init packer w/out build
	// +optional
	// +default=false
	initOnly bool,
	// vaultAddr
	// +optional
	vaultAddr string,
	// vaultRoleID
	// +optional
	vaultRoleID *dagger.Secret,
	// vaultSecretID
	// +optional
	vaultSecretID *dagger.Secret,
	// vaultToken
	// +optional
	vaultToken *dagger.Secret,
	// Source code management (SCM) version to use
	// +optional
	// +default="github"
	scm string,
	// Git repository to clone
	// +optional
	repository string,
	// Ref to checkout in the Git repository
	// +optional
	ref string,
	// Git authentication token
	// +optional
	token *dagger.Secret) {

	var configDir *dagger.Directory

	if repository != "" && token != nil {
		fmt.Println("Cloning Git repository...")
		configDir = m.CloneGitRepository(scm, repository, token)

		configDir = dag.Git().CloneGitHub(
			repository,
			token,
			dagger.GitCloneGitHubOpts{
				Ref: ref,
			},
		)

	} else {
		fmt.Println("Using local directory for Packer config...")
		configDir = packerConfigDir
	}

	// BAKE THE PACKER TEMPLATE
	fmt.Println("Baking Packer template...")
	m.Bake(
		ctx,
		configDir,
		packerConfig,
		packerVersion,
		arch,
		initOnly,
		vaultAddr,
		vaultRoleID,
		vaultSecretID,
		vaultToken,
	)

}

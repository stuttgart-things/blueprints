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
	gitRepository string,
	// Folder in git repository
	// +optional
	gitWorkdir string,
	// Ref to checkout in the Git repository
	// +optional
	gitRef string,
	// Git authentication token
	// +optional
	gitToken *dagger.Secret) {

	var configDir *dagger.Directory

	// CLONE GIT REPOSITORY IF PROVIDED
	if gitRepository != "" && gitToken != nil {
		configDir = dag.Git().CloneGitHub(
			gitRepository,
			gitToken,
			dagger.GitCloneGitHubOpts{
				Ref: gitRef,
			},
		)

		configDir = configDir.Directory(gitWorkdir)

	} else {
		// USE LOCAL DIRECTORY FOR PACKER CONFIG
		fmt.Println("USING LOCAL DIRECTORY FOR PACKER CONFIG...")
		configDir = packerConfigDir
	}

	// BAKE THE PACKER TEMPLATE +
	// GET THE VM-TEMPLATE NAME
	vmTemplateName, error := m.Bake(
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

	if error != nil {
		fmt.Println("Error baking Packer template:", error)
		return
	}

	fmt.Println("VM Template Name:", vmTemplateName)

	// CREATE TEST-VM FROM TEMPLATE

	// RUN ANSIBLE TESTS AGAINST THE TEST-VM

	// DELETE THE TEST-VM

	// RENAME EXISTING VM-TEMPLATE

	// RENAME NEW VM-TEMPLATE

	// MOVE NEW VM-TEMPLATE TO THE FINAL LOCATION

	// DELETE OLD VM-TEMPLATE

}

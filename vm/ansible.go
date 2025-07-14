package main

import (
	"context"
	"dagger/vm/internal/dagger"
)

func (m *Vm) ExecuteAnsible(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	playbooks string,
	// +optional
	requirements *dagger.File,
	// +optional
	inventory *dagger.File,
	// +optional
	parameters string,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// +optional
	sshUser *dagger.Secret,
	// +optional
	sshPassword *dagger.Secret,
) (bool, error) {

	return dag.Ansible().Execute(
		ctx,
		playbooks,
		dagger.AnsibleExecuteOpts{
			Src:            src,
			Inventory:      inventory,
			Parameters:     parameters,
			VaultAppRoleID: vaultAppRoleID,
			VaultSecretID:  vaultSecretID,
			VaultURL:       vaultURL,
			Requirements:   requirements,
			SSHUser:        sshUser,
			SSHPassword:    sshPassword,
		})

}

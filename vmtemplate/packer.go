package main

import (
	"context"
	"dagger/vmtemplate/internal/dagger"
)

func (m *Vmtemplate) Bake(
	ctx context.Context,
	packerConfigDir *dagger.Directory,
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
) error {
	return dag.Packer().
		Bake(
			ctx,
			packerConfig,
			packerConfigDir,
			dagger.PackerBakeOpts{
				PackerVersion: packerVersion,
				Arch:          arch,
				InitOnly:      initOnly,
				VaultAddr:     vaultAddr,
				VaultRoleID:   vaultRoleID,
				VaultSecretID: vaultSecretID,
				VaultToken:    vaultToken,
			})
}

// A generated module for Vmtemplate functions
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

package main

import (
	"context"
	"dagger/vmtemplate/internal/dagger"
)

type Vmtemplate struct{}

func (m *Vmtemplate) BakeTemplatePacker(
	ctx context.Context,
	packerConfigDir *dagger.Directory,
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
	vaultRoleID string,
	// vaultSecretID
	// +optional
	vaultSecretID *dagger.Secret,
	// vaultToken
	// +optional
	vaultToken *dagger.Secret,
	buildPath string,
	localDir *dagger.Directory,
) error {
	return dag.Packer().
		Bake(
			ctx,
			buildPath,
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

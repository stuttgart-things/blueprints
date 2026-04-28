package main

import (
	"context"
	"dagger/secrets/internal/dagger"
)

// EncryptFile encrypts a plaintext file with SOPS using an AGE public key
// and returns the encrypted contents.
func (m *Secrets) EncryptFile(
	ctx context.Context,
	// AGE public key for encryption (age1...)
	agePublicKey *dagger.Secret,
	// Plaintext file to encrypt
	plaintextFile *dagger.File,
	// File extension for SOPS encryption (e.g. "yaml", "json")
	// +optional
	// +default="yaml"
	fileExtension string,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
) (string, error) {
	return dag.Sops().Encrypt(
		agePublicKey,
		plaintextFile,
		dagger.SopsEncryptOpts{
			FileExtension: fileExtension,
			SopsConfig:    sopsConfig,
		},
	).Contents(ctx)
}

// EncryptString encrypts an in-memory string with SOPS using an AGE public
// key. Convenience wrapper around EncryptFile that materializes the input
// as a file first.
func (m *Secrets) EncryptString(
	ctx context.Context,
	// AGE public key for encryption (age1...)
	agePublicKey *dagger.Secret,
	// Plaintext content to encrypt
	plaintext string,
	// File extension for SOPS encryption (e.g. "yaml", "json")
	// +optional
	// +default="yaml"
	fileExtension string,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
) (string, error) {
	plainFile := dag.Directory().
		WithNewFile("payload."+fileExtension, plaintext).
		File("payload." + fileExtension)

	return m.EncryptFile(ctx, agePublicKey, plainFile, fileExtension, sopsConfig)
}

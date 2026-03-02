package main

import (
	"context"
	"dagger/vm/internal/dagger"
)

func (m *Vm) DecryptSops(
	ctx context.Context,
	sopsKey *dagger.Secret,
	encryptedFile *dagger.File,
) (string, error) {

	decryptedContent, err := dag.
		Sops().
		Decrypt(
			sopsKey,
			encryptedFile,
		).
		Contents(ctx)

	if err != nil {
		return "", err
	}

	return decryptedContent, nil
}

// EncryptFile encrypts a plaintext file with SOPS using an AGE public key.
func (m *Vm) EncryptFile(
	ctx context.Context,
	// AGE public key for encryption
	agePublicKey *dagger.Secret,
	// Plaintext file to encrypt
	plaintextFile *dagger.File,
	// File extension for SOPS encryption (e.g., "yaml", "json")
	// +optional
	// +default="yaml"
	fileExtension string,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
) (string, error) {

	encryptedContent, err := dag.Sops().Encrypt(
		agePublicKey,
		plaintextFile,
		dagger.SopsEncryptOpts{
			FileExtension: fileExtension,
			SopsConfig:    sopsConfig,
		},
	).Contents(ctx)

	if err != nil {
		return "", err
	}

	return encryptedContent, nil
}

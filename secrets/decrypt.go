package main

import (
	"context"
	"dagger/secrets/internal/dagger"
)

// Decrypt decrypts a SOPS-encrypted file with the given AGE private key
// and returns the plaintext contents.
func (m *Secrets) Decrypt(
	ctx context.Context,
	// AGE private key (AGE-SECRET-KEY-...)
	sopsKey *dagger.Secret,
	// SOPS-encrypted file (YAML/JSON)
	encryptedFile *dagger.File,
) (string, error) {
	return dag.Sops().
		Decrypt(sopsKey, encryptedFile).
		Contents(ctx)
}

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
		DecryptSops(
			ctx,
			sopsKey,
			encryptedFile,
		)

	if err != nil {
		return "", err
	}

	return decryptedContent, nil
}

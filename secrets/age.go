package main

import (
	"context"
	"dagger/secrets/internal/dagger"
	"fmt"
	"strings"
)

// ValidateAgeKeyPair derives the public key from the given AGE private key
// and verifies it matches the provided public key. Fails fast on mismatch.
//
// Usage:
//
//	dagger call -m secrets validate-age-key-pair --sops-age-key env:SOPS_AGE_KEY --age-public-key env:AGE_PUB
func (m *Secrets) ValidateAgeKeyPair(
	ctx context.Context,
	// AGE private key
	sopsAgeKey *dagger.Secret,
	// AGE public key to validate against
	agePublicKey *dagger.Secret,
) (string, error) {
	pubKeyPlain, err := agePublicKey.Plaintext(ctx)
	if err != nil {
		return "", fmt.Errorf("validate-age-key-pair: read agePublicKey: %w", err)
	}
	pubKeyPlain = strings.TrimSpace(pubKeyPlain)

	derived, err := dag.Container().
		From("alpine:3.21").
		WithExec([]string{"apk", "add", "--no-cache", "age"}).
		WithMountedSecret("/tmp/age-key", sopsAgeKey, dagger.ContainerWithMountedSecretOpts{
			Mode: 0444,
		}).
		WithExec([]string{"sh", "-c", "age-keygen -y /tmp/age-key"}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("validate-age-key-pair: derive public key: %w", err)
	}
	derived = strings.TrimSpace(derived)

	if derived != pubKeyPlain {
		return "", fmt.Errorf("validate-age-key-pair: MISMATCH — derived public key %q does not match provided %q", derived, pubKeyPlain)
	}

	return fmt.Sprintf("AGE key pair valid: %s", derived), nil
}

package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/flux/internal/dagger"
)

// ValidateAgeKeyPair derives the public key from the given AGE private key
// and verifies it matches the provided public key. Fails fast on mismatch.
//
// Usage:
//
//	dagger call validate-age-key-pair --sops-age-key env:SOPS_AGE_KEY --age-public-key env:AGE_PUB
func (m *Flux) ValidateAgeKeyPair(
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

// FluxEncryptSecrets encrypts secret YAML content with SOPS using the given AGE public key.
func (m *Flux) FluxEncryptSecrets(
	ctx context.Context,
	// Plain-text secret YAML content
	secretContent string,
	// AGE public key for encryption
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
) (string, error) {
	plainSecretFile := dag.Directory().
		WithNewFile("secrets.yaml", secretContent).
		File("secrets.yaml")

	encryptedFile := dag.Sops().Encrypt(
		agePublicKey,
		plainSecretFile,
		dagger.SopsEncryptOpts{
			FileExtension: "yaml",
			SopsConfig:    sopsConfig,
		},
	)

	return encryptedFile.Contents(ctx)
}

// FluxApplySecrets applies secret manifests to the cluster.
func (m *Flux) FluxApplySecrets(
	ctx context.Context,
	// Secret YAML content
	secretContent string,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
) (string, error) {
	secretFile := dag.Directory().
		WithNewFile("secrets.yaml", secretContent).
		File("secrets.yaml")

	_, err := dag.Kubernetes().Kubectl(
		ctx,
		dagger.KubernetesKubectlOpts{
			Operation:  "apply",
			SourceFile: secretFile,
			Namespace:  namespace,
			KubeConfig: kubeConfig,
		},
	)
	if err != nil {
		return "", fmt.Errorf("flux-apply-secrets: %w", err)
	}

	return "Secrets applied to cluster", nil
}

// FluxVerifySecrets auto-extracts secret names from the YAML and verifies they
// exist in the cluster.
func (m *Flux) FluxVerifySecrets(
	ctx context.Context,
	// Secret YAML content (multi-document)
	secretContent string,
	// Target namespace
	// +optional
	// +default="flux-system"
	namespace string,
	// Kubeconfig secret for cluster access
	kubeConfig *dagger.Secret,
) (string, error) {
	docs := strings.Split(secretContent, "---")
	var secretNames []string
	for _, doc := range docs {
		if !strings.Contains(doc, "kind: Secret") {
			continue
		}
		inMetadata := false
		for _, line := range strings.Split(doc, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "metadata:" {
				inMetadata = true
				continue
			}
			if inMetadata && strings.HasPrefix(trimmed, "name:") {
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				name = strings.Trim(name, "\"'")
				if name != "" {
					secretNames = append(secretNames, name)
				}
				inMetadata = false
				break
			}
			if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
				inMetadata = false
			}
		}
	}

	if len(secretNames) == 0 {
		return "No secret names found in YAML", nil
	}

	var found, missing []string
	for _, name := range secretNames {
		_, err := dag.Container().
			From("bitnami/kubectl:latest").
			WithMountedSecret("/tmp/kubeconfig", kubeConfig, dagger.ContainerWithMountedSecretOpts{
				Mode: 0444,
			}).
			WithEnvVariable("KUBECONFIG", "/tmp/kubeconfig").
			WithExec([]string{"kubectl", "get", "secret", name, "-n", namespace, "-o", "name"}).
			Stdout(ctx)
		if err != nil {
			missing = append(missing, name)
		} else {
			found = append(found, name)
		}
	}

	var result []string
	if len(found) > 0 {
		result = append(result, fmt.Sprintf("Found secrets: %s", strings.Join(found, ", ")))
	}
	if len(missing) > 0 {
		result = append(result, fmt.Sprintf("Missing secrets: %s", strings.Join(missing, ", ")))
		return strings.Join(result, "\n"), fmt.Errorf("flux-verify-secrets: %d secret(s) missing: %s", len(missing), strings.Join(missing, ", "))
	}

	return strings.Join(result, "\n"), nil
}

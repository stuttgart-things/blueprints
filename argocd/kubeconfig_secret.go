package main

import (
	"context"
	"encoding/base64"
	"fmt"

	"dagger/argocd/internal/dagger"
)

// RenderKubeconfigSecret turns a SOPS-encrypted source file (typically a
// cluster kubeconfig under stuttgart-things/secrets/kubeconfigs/) into a
// Kubernetes v1/Secret manifest. Equivalent to:
//
//	sops --decrypt <sourceFile> > kubeconfig.yaml
//	kubectl create secret generic <name> -n <namespace> \
//	  --from-file=<dataKey>=kubeconfig.yaml \
//	  --dry-run=client -o yaml
//
// By default the rendered Secret is re-encrypted with SOPS using agePublicKey
// so it can be safely committed to git (e.g. via commit-config). Pass
// --encrypt=false to get the plaintext manifest for direct `kubectl apply`
// (e.g. via apply-config).
//
// Returns the manifest as a Dagger File so it composes with apply-config and
// commit-config without shell round-trips.
func (m *Argocd) RenderKubeconfigSecret(
	ctx context.Context,
	// SOPS-encrypted source file (e.g. kubeconfigs/kind-dev-test1.yaml)
	sourceFile *dagger.File,
	// AGE private key for decrypting sourceFile (AGE-SECRET-KEY-...)
	sopsKey *dagger.Secret,
	// Secret name (e.g. "kind-dev-test1" or "target-cluster-kubeconfig")
	name string,
	// Target namespace
	// +optional
	// +default="argocd"
	namespace string,
	// Data key under data: in the rendered Secret (data.<dataKey>)
	// +optional
	// +default="kubeconfig"
	dataKey string,
	// Re-encrypt the rendered Secret with SOPS using agePublicKey
	// +optional
	// +default=true
	encrypt bool,
	// AGE public key for SOPS re-encryption (required when encrypt=true)
	// +optional
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml) used during re-encryption
	// +optional
	sopsConfig *dagger.File,
) (*dagger.File, error) {
	if sourceFile == nil {
		return nil, fmt.Errorf("source-file is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if encrypt && agePublicKey == nil {
		return nil, fmt.Errorf("encrypt=true requires --age-public-key")
	}

	plaintext, err := dag.Secrets().Decrypt(ctx, sopsKey, sourceFile)
	if err != nil {
		return nil, fmt.Errorf("decrypt source: %w", err)
	}

	manifest := fmt.Sprintf(
		"apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s\n  namespace: %s\ntype: Opaque\ndata:\n  %s: %s\n",
		name, namespace, dataKey,
		base64.StdEncoding.EncodeToString([]byte(plaintext)),
	)

	plainFile := dag.Directory().
		WithNewFile("secret.yaml", manifest).
		File("secret.yaml")

	if !encrypt {
		return plainFile, nil
	}

	encrypted, err := dag.Secrets().EncryptFile(
		ctx,
		agePublicKey,
		plainFile,
		dagger.SecretsEncryptFileOpts{
			FileExtension: "yaml",
			SopsConfig:    sopsConfig,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("encrypt manifest: %w", err)
	}

	return dag.Directory().
		WithNewFile("secret.enc.yaml", encrypted).
		File("secret.enc.yaml"), nil
}

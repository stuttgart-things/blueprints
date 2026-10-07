package main

import (
	"context"
	"dagger/secrets/internal/dagger"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// secretManifestTemplate renders a v1/Secret with base64-encoded data entries.
// Rendered via the templating module (dag.Templating().RenderInline).
const secretManifestTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: {{.name}}
  namespace: {{.namespace}}
type: Opaque
data:
{{range $k := .keys}}  {{$k}}: {{index $.items $k}}
{{end}}`

// CreateKubernetesSecret builds a Kubernetes Secret manifest from name,
// namespace, and comma-separated key=value pairs, then encrypts it with SOPS
// using the given AGE public key. Returns the encrypted manifest as a *dagger.File.
//
// Values are base64-encoded and placed under `data:` to match the standard
// Kubernetes Secret layout. Only `data`/`stringData` are encrypted, so
// apiVersion, kind and metadata stay readable and Flux can apply the result.
//
// Usage:
//
//	dagger call -m secrets create-kubernetes-secret \
//	  --name my-secret --namespace default \
//	  --key-values "user=admin,password=s3cret" \ # pragma: allowlist secret
//	  --age-public-key env:AGE_PUB \
//	  export --path ./secret.enc.yaml
func (m *Secrets) CreateKubernetesSecret(
	ctx context.Context,
	name string,
	namespace string,
	// Comma-separated key=value pairs (e.g. "user=admin,password=s3cret") # pragma: allowlist secret
	keyValues string,
	// AGE public key for SOPS encryption
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml). When given, its creation rule decides
	// what is encrypted and --encrypted-regex is not applied.
	// +optional
	sopsConfig *dagger.File,
	// Keys whose values are encrypted. The default keeps apiVersion, kind and
	// metadata readable, which Flux's kustomize-controller needs.
	// +optional
	// +default="^(data|stringData)$"
	encryptedRegex string,
) (*dagger.File, error) {
	manifest, err := renderSecretManifest(ctx, name, namespace, keyValues)
	if err != nil {
		return nil, fmt.Errorf("create-kubernetes-secret: %w", err)
	}

	plainFile := dag.Directory().
		WithNewFile("secret.yaml", manifest).
		File("secret.yaml")

	opts := dagger.SopsEncryptOpts{
		FileExtension: "yaml",
		SopsConfig:    sopsConfig,
	}
	if sopsConfig == nil {
		opts.EncryptedRegex = encryptedRegex
	}

	return dag.Sops().Encrypt(agePublicKey, plainFile, opts), nil
}

// CreateKubernetesSecretString is the string-returning variant of
// CreateKubernetesSecret.
func (m *Secrets) CreateKubernetesSecretString(
	ctx context.Context,
	name string,
	namespace string,
	keyValues string,
	agePublicKey *dagger.Secret,
	// +optional
	sopsConfig *dagger.File,
	// +optional
	// +default="^(data|stringData)$"
	encryptedRegex string,
) (string, error) {
	f, err := m.CreateKubernetesSecret(ctx, name, namespace, keyValues, agePublicKey, sopsConfig, encryptedRegex)
	if err != nil {
		return "", err
	}
	return f.Contents(ctx)
}

func renderSecretManifest(ctx context.Context, name, namespace, keyValues string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(keyValues) == "" {
		return "", fmt.Errorf("keyValues is required")
	}

	items := map[string]string{}
	var keys []string
	for _, pair := range strings.Split(keyValues, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid key=value pair: %q", pair)
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k == "" {
			return "", fmt.Errorf("empty key in pair: %q", pair)
		}
		if _, dup := items[k]; dup {
			return "", fmt.Errorf("duplicate key: %q", k)
		}
		items[k] = base64.StdEncoding.EncodeToString([]byte(v))
		keys = append(keys, k)
	}
	sort.Strings(keys)

	vars, err := json.Marshal(map[string]any{
		"name":      name,
		"namespace": namespace,
		"items":     items,
		"keys":      keys,
	})
	if err != nil {
		return "", fmt.Errorf("marshal template vars: %w", err)
	}

	return dag.Templating().RenderInline(
		ctx,
		secretManifestTemplate,
		dagger.TemplatingRenderInlineOpts{
			Variables:  string(vars),
			StrictMode: true,
		},
	)
}

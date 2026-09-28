package clustersecrets

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"filippo.io/age"
	"gopkg.in/yaml.v3"
)

const (
	// Only the values are encrypted; apiVersion, kind and metadata stay
	// readable, so a PR shows which secrets changed.
	EncryptedRegex = "^(data|stringData)$"

	// The cluster's private key, as the Secret Flux's kustomize-controller
	// reads for decryption (spec.decryption.secretRef).
	AgeKeyFile      = "sops-age.enc.yaml"
	AgeKeySecret    = "sops-age" // pragma: allowlist secret
	AgeKeyNamespace = "flux-system"
	AgeKeyDataKey   = "age.agekey"

	PublicKeyFile = "age.pub"
	SecretsDir    = "secrets" // pragma: allowlist secret
)

// SecretPath is where a secret's encrypted manifest lives in the output.
func SecretPath(namespace, name string) string {
	return path.Join(SecretsDir, namespace, name+".enc.yaml")
}

type manifest struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   manifestMeta      `yaml:"metadata"`
	Type       string            `yaml:"type,omitempty"`
	Data       map[string]string `yaml:"data,omitempty"`
	StringData map[string]string `yaml:"stringData,omitempty"`
}

type manifestMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// RenderSecret renders s as a plaintext v1/Secret using stringData.
func RenderSecret(s Secret) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(manifest{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   manifestMeta{Name: s.Name, Namespace: s.Namespace},
		Type:       s.Type,
		StringData: s.Data,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseSecret reads a decrypted v1/Secret, merging data (base64) and
// stringData the way the API server does (stringData wins).
func ParseSecret(doc []byte) (Secret, error) {
	var m manifest
	if err := yaml.Unmarshal(doc, &m); err != nil {
		return Secret{}, err
	}
	if m.Kind != "Secret" {
		return Secret{}, fmt.Errorf("kind is %q, want Secret", m.Kind)
	}
	s := Secret{Name: m.Metadata.Name, Namespace: m.Metadata.Namespace, Type: m.Type, Data: map[string]string{}}
	for k, v := range m.Data {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return Secret{}, fmt.Errorf("secret %s: data.%s is not base64: %w", s.ID(), k, err)
		}
		s.Data[k] = string(b)
	}
	for k, v := range m.StringData {
		s.Data[k] = v
	}
	return s, nil
}

// RenderKustomization lists the given resources, so a Flux Kustomization
// (with SOPS decryption) can point straight at the secrets directory.
func RenderKustomization(resources []string) []byte {
	sorted := append([]string(nil), resources...)
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString("---\napiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n")
	for _, r := range sorted {
		fmt.Fprintf(&b, "  - %s\n", r)
	}
	return []byte(b.String())
}

// RenderSopsConfig writes the .sops.yaml a human needs to edit the output
// later with plain `sops`: the key file is encrypted for the key custodians
// (CI master + escrow), everything else for the cluster (+ escrow).
func RenderSopsConfig(clusterRecipients, keyRecipients []string) []byte {
	return []byte(fmt.Sprintf(`---
creation_rules:
  - path_regex: (^|/)%s$
    encrypted_regex: %s
    age: %s
  - path_regex: (^|/)%s/.*\.enc\.yaml$
    encrypted_regex: %s
    age: %s
`, regexp.QuoteMeta(AgeKeyFile), EncryptedRegex, strings.Join(keyRecipients, ","),
		SecretsDir, EncryptedRegex, strings.Join(clusterRecipients, ",")))
}

// AgeKeySecret wraps a cluster's private key as the Secret Flux decrypts with.
func NewAgeKeySecret(privateKey string) Secret {
	return Secret{
		Name:      AgeKeySecret,
		Namespace: AgeKeyNamespace,
		Type:      "Opaque",
		Data:      map[string]string{AgeKeyDataKey: strings.TrimSpace(privateKey) + "\n"},
	}
}

// GenerateAgeKey returns a new X25519 key pair. Generated in Go rather
// than with age-keygen in a container: an exec with identical inputs is
// served from the engine cache, and every cluster would get the same key.
func GenerateAgeKey() (public, private string, err error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", err
	}
	return id.Recipient().String(), id.String(), nil
}

// AgePublicKey derives the public key from key material in age key-file
// format (comments allowed). It must hold exactly one X25519 identity, so
// the recipient the key file is encrypted for is unambiguous.
func AgePublicKey(keyFile string) (string, error) {
	ids, err := age.ParseIdentities(strings.NewReader(keyFile))
	if err != nil {
		return "", fmt.Errorf("parse age key: %w", err)
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("age key file holds %d identities, want exactly 1", len(ids))
	}
	x, ok := ids[0].(*age.X25519Identity)
	if !ok {
		return "", fmt.Errorf("age identity is %T, want X25519", ids[0])
	}
	return x.Recipient().String(), nil
}

// ParseRecipients splits and validates a comma-separated list of age
// public keys.
func ParseRecipients(csv string) ([]string, error) {
	var out []string
	for _, r := range strings.Split(csv, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, err := age.ParseX25519Recipient(r); err != nil {
			return nil, fmt.Errorf("%q is not an age public key: %w", r, err)
		}
		out = append(out, r)
	}
	return out, nil
}

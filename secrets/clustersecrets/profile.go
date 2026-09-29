// Package clustersecrets holds the engine-independent half of
// GenerateClusterSecrets: profile parsing, value generation, reference
// resolution and manifest rendering. It has no Dagger import, so everything
// that decides *what* ends up in a cluster's secrets is unit-testable with
// plain `go test`, without an engine.
package clustersecrets

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	KindClusterSecrets = "ClusterSecrets" // pragma: allowlist secret
	KindSecretProfile  = "SecretProfile"  // pragma: allowlist secret
)

// ClusterSecrets is the per-cluster entry point: which profiles apply, plus
// secrets (or single keys) defined only for this cluster.
//
//	kind: ClusterSecrets
//	metadata: { name: edge-01 }
//	spec:
//	  profiles: [platform]
//	  secrets:
//	    - name: harbor-secrets
//	      namespace: flux-system
//	      data:
//	        HARBOR_ADMIN_PASSWORD: { generate: { type: password, length: 24 } }
type ClusterSecrets struct {
	Kind     string   `yaml:"kind"`
	Metadata Metadata `yaml:"metadata"`
	Spec     struct {
		Profiles []string     `yaml:"profiles"`
		Secrets  []SecretSpec `yaml:"secrets"`
	} `yaml:"spec"`
}

// SecretProfile is a reusable set of secrets, referenced by metadata.name.
type SecretProfile struct {
	Kind     string   `yaml:"kind"`
	Metadata Metadata `yaml:"metadata"`
	Spec     struct {
		Secrets []SecretSpec `yaml:"secrets"`
	} `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

// SecretSpec describes one Kubernetes Secret.
type SecretSpec struct {
	Name      string               `yaml:"name"`
	Namespace string               `yaml:"namespace"`
	Type      string               `yaml:"type"`
	Data      map[string]ValueSpec `yaml:"data"`
}

// ValueSpec is exactly one of: a generated value, a reference to an existing
// value (ref+sops:// or ref+vault://), or a literal.
type ValueSpec struct {
	Generate *GenerateSpec `yaml:"generate"`
	Ref      string        `yaml:"ref"`
	Value    *string       `yaml:"value"`
}

// UnmarshalYAML also accepts a bare scalar as shorthand: a string starting
// with "ref+" is a reference, anything else a literal value.
func (v *ValueSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if strings.HasPrefix(node.Value, "ref+") {
			v.Ref = node.Value
		} else {
			s := node.Value
			v.Value = &s
		}
		return nil
	}
	type plain ValueSpec
	return node.Decode((*plain)(v))
}

func (v ValueSpec) validate() error {
	n := 0
	if v.Generate != nil {
		n++
		if err := v.Generate.validate(); err != nil {
			return err
		}
	}
	if v.Ref != "" {
		n++
		if _, err := ParseRef(v.Ref); err != nil {
			return err
		}
	}
	if v.Value != nil {
		n++
	}
	if n != 1 {
		return fmt.Errorf("exactly one of generate, ref or value must be set (got %d)", n)
	}
	return nil
}

// DNS-1123 subdomain, as Kubernetes requires for Secret names.
var dnsSubdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// Keys of a Secret's data map.
var secretKey = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)

func (s SecretSpec) validate() error {
	if !dnsSubdomain.MatchString(s.Name) || len(s.Name) > 253 {
		return fmt.Errorf("invalid secret name %q", s.Name)
	}
	if s.Namespace != "" && !dnsSubdomain.MatchString(s.Namespace) {
		return fmt.Errorf("secret %s: invalid namespace %q", s.Name, s.Namespace)
	}
	if len(s.Data) == 0 {
		return fmt.Errorf("secret %s: data is empty", s.Name)
	}
	for k, v := range s.Data {
		if !secretKey.MatchString(k) {
			return fmt.Errorf("secret %s: invalid key %q", s.Name, k)
		}
		if err := v.validate(); err != nil {
			return fmt.Errorf("secret %s, key %s: %w", s.Name, k, err)
		}
	}
	return nil
}

// ID is the namespace/name pair a secret is identified by.
func (s SecretSpec) ID() string { return s.Namespace + "/" + s.Name }

// ParseClusterSecrets parses and validates a ClusterSecrets document.
func ParseClusterSecrets(data []byte) (*ClusterSecrets, error) {
	var c ClusterSecrets
	if err := decodeStrict(data, &c); err != nil {
		return nil, fmt.Errorf("parse ClusterSecrets: %w", err)
	}
	if c.Kind != KindClusterSecrets {
		return nil, fmt.Errorf("parse ClusterSecrets: kind is %q, want %q", c.Kind, KindClusterSecrets)
	}
	if c.Metadata.Name == "" {
		return nil, fmt.Errorf("parse ClusterSecrets: metadata.name is required")
	}
	return &c, nil
}

// ParseSecretProfile parses and validates a SecretProfile document.
func ParseSecretProfile(data []byte) (*SecretProfile, error) {
	var p SecretProfile
	if err := decodeStrict(data, &p); err != nil {
		return nil, fmt.Errorf("parse SecretProfile: %w", err)
	}
	if p.Kind != KindSecretProfile {
		return nil, fmt.Errorf("parse SecretProfile: kind is %q, want %q", p.Kind, KindSecretProfile)
	}
	if p.Metadata.Name == "" {
		return nil, fmt.Errorf("parse SecretProfile: metadata.name is required")
	}
	return &p, nil
}

// PeekKind returns a YAML document's kind, or "" if it has none.
func PeekKind(data []byte) string {
	var head struct {
		Kind string `yaml:"kind"`
	}
	if yaml.Unmarshal(data, &head) != nil {
		return ""
	}
	return head.Kind
}

// decodeStrict rejects unknown fields, so a typo like `genrate:` fails
// instead of silently producing a secret without that key.
func decodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	return dec.Decode(out)
}

// Merge resolves the cluster's profiles (in the listed order) and its inline
// secrets into one list. Later definitions override earlier ones key by key,
// so a cluster can replace a single value of a profile secret without
// repeating the rest. Secrets without a namespace get defaultNamespace.
// The result is sorted by namespace/name and validated.
func Merge(c *ClusterSecrets, profiles map[string]*SecretProfile, defaultNamespace string) ([]SecretSpec, error) {
	merged := map[string]*SecretSpec{}
	add := func(specs []SecretSpec) {
		for _, s := range specs {
			if s.Namespace == "" {
				s.Namespace = defaultNamespace
			}
			cur, ok := merged[s.ID()]
			if !ok {
				cp := s
				cp.Data = map[string]ValueSpec{}
				merged[s.ID()] = &cp
				cur = &cp
			}
			if s.Type != "" {
				cur.Type = s.Type
			}
			for k, v := range s.Data {
				cur.Data[k] = v
			}
		}
	}

	for _, name := range c.Spec.Profiles {
		p, ok := profiles[name]
		if !ok {
			return nil, fmt.Errorf("profile %q referenced by %s not found", name, c.Metadata.Name)
		}
		add(p.Spec.Secrets)
	}
	add(c.Spec.Secrets)

	out := make([]SecretSpec, 0, len(merged))
	for _, s := range merged {
		if s.Type == "" {
			s.Type = "Opaque"
		}
		if err := s.validate(); err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	if len(out) == 0 {
		return nil, fmt.Errorf("%s defines no secrets", c.Metadata.Name)
	}
	return out, nil
}

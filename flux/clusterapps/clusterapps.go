// Package clusterapps turns a ClusterApps file plus the catalog's AppProfiles
// into what a cluster repo needs to run those apps: one Flux Kustomization per
// catalog bundle (components + postBuild.substitute), the catalog
// GitRepository, and the inputs for secrets' generate-cluster-secrets.
//
// An AppProfile describes both halves of one catalog app, so they cannot
// drift apart:
//
//	kind: AppProfile
//	metadata: { name: keycloak }
//	spec:
//	  bundle: apps-platform
//	  component: ../components/keycloak
//	  vars:
//	    KEYCLOAK_STORAGE_CLASS: { required: true }
//	    KEYCLOAK_HOSTNAME: {}
//	  secrets:
//	    - name: keycloak-secrets
//	      data:
//	        ADMIN_USER: admin
//	        ADMIN_PASSWORD: { generate: { type: password, length: 24 } }
//
// The catalog reads secrets through postBuild.substituteFrom, which only sees
// Secrets in the Kustomization's own namespace, so every secret goes to
// flux-system.
//
// No Dagger import: the module wraps it, the tests run without an engine.
package clusterapps

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	KindAppProfile  = "AppProfile"
	KindClusterApps = "ClusterApps"

	ModeSeparate = "separate"
	ModeInline   = "inline"

	// SecretsNamespace is where substituteFrom can read the secrets.
	SecretsNamespace = "flux-system" // pragma: allowlist secret

	defaultSourceName       = "flux-apps"
	defaultSecretsKs        = "cluster-secrets" // pragma: allowlist secret
	defaultDecryptionSecret = "sops-age"        // pragma: allowlist secret
	defaultClusterSource    = "flux-system"
)

// bundle is one catalog bundle: its root and the variable its components
// read the GitRepository name from.
type bundle struct {
	root      string
	sourceVar string
}

var bundles = map[string]bundle{
	"apps-platform":  {root: "./apps/platform/root", sourceVar: "APPS_SOURCE"},
	"cicd-platform":  {root: "./cicd/platform/root", sourceVar: "APPS_SOURCE"},
	"infra-platform": {root: "./infra/platform/root", sourceVar: "FLUX_SOURCE"},
}

type Metadata struct {
	Name string `yaml:"name"`
}

// AppProfile describes one catalog app. It lives in the catalog, next to the
// app's component.
type AppProfile struct {
	Kind     string   `yaml:"kind"`
	Metadata Metadata `yaml:"metadata"`
	Spec     struct {
		Bundle    string             `yaml:"bundle"`
		Component string             `yaml:"component"`
		Vars      map[string]VarSpec `yaml:"vars"`
		Secrets   []SecretSpec       `yaml:"secrets"`
	} `yaml:"spec"`
}

// VarSpec declares a postBuild.substitute variable the app reads. Only
// declared variables may be set per app, so a typo fails the run instead of
// silently falling back to the catalog default.
type VarSpec struct {
	Required bool    `yaml:"required"`
	Default  *string `yaml:"default"`
}

// SecretSpec is a Secret in SecretProfile format, minus the namespace.
// Values stay YAML nodes: secrets' generate-cluster-secrets validates them.
type SecretSpec struct {
	Name string               `yaml:"name"`
	Type string               `yaml:"type,omitempty"`
	Data map[string]yaml.Node `yaml:"data"`
}

// ClusterApps is the per-cluster file: which apps run, with which values.
//
//	kind: ClusterApps
//	metadata: { name: cicd-test4 }
//	spec:
//	  source: { url: https://github.com/stuttgart-things/flux.git, tag: v1.99.2 }
//	  secrets: { path: ./secrets/clusters/cicd-test4 }
//	  vars: { INFRA_DOMAIN: cicd-test4.example.com }
//	  apps:
//	    keycloak:
//	      vars: { KEYCLOAK_STORAGE_CLASS: openebs-hostpath }
//	      secrets:
//	        keycloak-secrets: { ADMIN_PASSWORD: ref+vault://secret/kc#/admin }
type ClusterApps struct {
	Kind     string   `yaml:"kind"`
	Metadata Metadata `yaml:"metadata"`
	Spec     struct {
		Source  Source            `yaml:"source"`
		Secrets SecretsSpec       `yaml:"secrets"`
		Bundles map[string]Bundle `yaml:"bundles"`
		Vars    map[string]string `yaml:"vars"`
		Apps    map[string]App    `yaml:"apps"`
	} `yaml:"spec"`
}

// Source is the catalog GitRepository the bundles pull from.
type Source struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Tag      string `yaml:"tag"`
	Branch   string `yaml:"branch"`
	Interval string `yaml:"interval"`
}

// SecretsSpec says where the generate-cluster-secrets output lives in the
// cluster repo, and who applies it.
//
// mode separate (default): a Kustomization of its own applies spec.secrets.path
// and decrypts with decryptionSecret. For a cluster that already has a
// sops-age key of its own.
//
// mode inline: the output sits in the cluster's own path, next to apps.yaml,
// and the cluster's root Kustomization applies it with its sops-age key --
// which then has to be the cluster key. For clusters bootstrapped with it.
type SecretsSpec struct {
	Mode string `yaml:"mode"`
	// separate: repo path the generate-cluster-secrets output is exported to.
	// It must lie outside every path another Kustomization decrypts with a
	// different key.
	Path             string `yaml:"path"`
	Kustomization    string `yaml:"kustomization"`
	DecryptionSecret string `yaml:"decryptionSecret"`
	// GitRepository of the cluster repo itself
	SourceRef string `yaml:"sourceRef"`
}

// Bundle renames a bundle's Kustomization, e.g. to run next to an existing one.
type Bundle struct {
	Name string `yaml:"name"`
}

// App enables one catalog app and sets its values.
type App struct {
	Vars map[string]string `yaml:"vars"`
	// secret name -> key -> value (generate, ref or literal)
	Secrets map[string]map[string]yaml.Node `yaml:"secrets"`
}

// Result is what Build renders.
type Result struct {
	// Flux manifests for the cluster's own path: the catalog GitRepository
	// and, in mode separate, the secrets Kustomization.
	Manifests []byte
	// One parameters file per bundle for claim-flux-kustomizations
	// templateName=bundle, which renders the bundle Kustomization. The
	// structured values (components, substitute) survive only through a
	// parameters file, not through the comma-separated --parameters form.
	Bundles []BundleParams
	// ClusterSecrets and SecretProfile documents for generate-cluster-secrets.
	// Nil when no enabled app has secrets.
	ClusterSecrets []byte
	SecretProfiles map[string][]byte
	// Inline: the secrets go next to Manifests, see SecretsSpec.
	Inline bool
}

// BundleParams is a parameters file for claim-flux-kustomizations.
type BundleParams map[string]any

// SourceIgnore keeps the files the cluster must not apply out of its root
// Kustomization in inline mode: the key file is encrypted for master and
// escrow only, and .sops.yaml is no Kubernetes object. Placed in the output
// directory; source-controller honours .sourceignore in subdirectories.
const SourceIgnore = "# written by render-cluster-apps\nsops-age.enc.yaml\n.sops.yaml\n"

// ParseClusterApps decodes a ClusterApps document; unknown fields fail.
func ParseClusterApps(b []byte) (*ClusterApps, error) {
	var c ClusterApps
	if err := strictDecode(b, &c); err != nil {
		return nil, fmt.Errorf("ClusterApps: %w", err)
	}
	if c.Kind != KindClusterApps {
		return nil, fmt.Errorf("kind is %q, want %s", c.Kind, KindClusterApps)
	}
	if c.Metadata.Name == "" {
		return nil, fmt.Errorf("ClusterApps: metadata.name is empty")
	}
	return &c, nil
}

// ParseProfiles reads every AppProfile from the given files (path ->
// content). Documents of other kinds are skipped; a name used twice fails.
func ParseProfiles(files map[string][]byte) (map[string]*AppProfile, error) {
	out := map[string]*AppProfile{}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		dec := yaml.NewDecoder(bytes.NewReader(files[p]))
		for {
			var node yaml.Node
			if err := dec.Decode(&node); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			var head struct {
				Kind string `yaml:"kind"`
			}
			if err := node.Decode(&head); err != nil || head.Kind != KindAppProfile {
				continue
			}
			raw, err := yaml.Marshal(&node)
			if err != nil {
				return nil, err
			}
			var ap AppProfile
			if err := strictDecode(raw, &ap); err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			if err := ap.validate(); err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			if _, dup := out[ap.Metadata.Name]; dup {
				return nil, fmt.Errorf("%s: AppProfile %q defined twice", p, ap.Metadata.Name)
			}
			out[ap.Metadata.Name] = &ap
		}
	}
	return out, nil
}

func (ap *AppProfile) validate() error {
	name := ap.Metadata.Name
	if name == "" {
		return fmt.Errorf("AppProfile: metadata.name is empty")
	}
	if _, ok := bundles[ap.Spec.Bundle]; !ok {
		return fmt.Errorf("AppProfile %s: unknown bundle %q (known: %s)", name, ap.Spec.Bundle, strings.Join(bundleNames(), ", "))
	}
	if ap.Spec.Component == "" {
		return fmt.Errorf("AppProfile %s: spec.component is empty", name)
	}
	seen := map[string]bool{}
	for _, s := range ap.Spec.Secrets {
		if s.Name == "" || len(s.Data) == 0 {
			return fmt.Errorf("AppProfile %s: every secret needs a name and data", name)
		}
		if seen[s.Name] {
			return fmt.Errorf("AppProfile %s: secret %q listed twice", name, s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

// Build checks the cluster file against the profiles and renders the result.
func Build(c *ClusterApps, profiles map[string]*AppProfile) (*Result, error) {
	spec := &c.Spec
	if spec.Source.URL == "" || (spec.Source.Tag == "") == (spec.Source.Branch == "") {
		return nil, fmt.Errorf("%s: spec.source needs a url and exactly one of tag or branch", c.Metadata.Name)
	}
	if len(spec.Apps) == 0 {
		return nil, fmt.Errorf("%s: spec.apps is empty", c.Metadata.Name)
	}
	for b := range spec.Bundles {
		if _, ok := bundles[b]; !ok {
			return nil, fmt.Errorf("%s: spec.bundles: unknown bundle %q", c.Metadata.Name, b)
		}
	}
	sourceName := or(spec.Source.Name, defaultSourceName)

	type bundleState struct {
		components []string
		substitute map[string]string
		setBy      map[string]string // var -> app that set it, for conflicts
		secrets    bool
	}
	state := map[string]*bundleState{}
	var withSecrets []*AppProfile
	var errs []error

	for _, appName := range sortedKeys(spec.Apps) {
		app := spec.Apps[appName]
		ap, ok := profiles[appName]
		if !ok {
			errs = append(errs, fmt.Errorf("app %s: no AppProfile in the catalog", appName))
			continue
		}
		bs := state[ap.Spec.Bundle]
		if bs == nil {
			bs = &bundleState{substitute: map[string]string{}, setBy: map[string]string{}}
			state[ap.Spec.Bundle] = bs
		}
		bs.components = append(bs.components, ap.Spec.Component)

		for k := range app.Vars {
			if _, ok := ap.Spec.Vars[k]; !ok {
				errs = append(errs, fmt.Errorf("app %s: var %s is not declared by its AppProfile", appName, k))
			}
		}
		for _, k := range sortedKeys(ap.Spec.Vars) {
			v := ap.Spec.Vars[k]
			val, set := app.Vars[k]
			if !set {
				val, set = spec.Vars[k]
			}
			if !set && v.Default != nil {
				val, set = *v.Default, true
			}
			if !set {
				if v.Required {
					errs = append(errs, fmt.Errorf("app %s: required var %s is not set", appName, k))
				}
				continue
			}
			if prev, ok := bs.substitute[k]; ok && prev != val {
				errs = append(errs, fmt.Errorf("app %s: var %s=%q conflicts with %q set by %s in the same bundle", appName, k, val, prev, bs.setBy[k]))
				continue
			}
			bs.substitute[k] = val
			bs.setBy[k] = appName
		}

		if err := checkSecretOverrides(appName, ap, app); err != nil {
			errs = append(errs, err)
		}
		if len(ap.Spec.Secrets) > 0 {
			bs.secrets = true // pragma: allowlist secret
			withSecrets = append(withSecrets, ap)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	inline := false
	switch spec.Secrets.Mode {
	case "", ModeSeparate:
		if len(withSecrets) > 0 && spec.Secrets.Path == "" {
			return nil, fmt.Errorf("%s: apps with secrets are enabled, so spec.secrets.path is required", c.Metadata.Name)
		}
	case ModeInline:
		inline = true
		if spec.Secrets.Path != "" || spec.Secrets.Kustomization != "" || spec.Secrets.DecryptionSecret != "" || spec.Secrets.SourceRef != "" {
			return nil, fmt.Errorf("%s: spec.secrets: mode inline takes no path, kustomization, decryptionSecret or sourceRef", c.Metadata.Name)
		}
	default:
		return nil, fmt.Errorf("%s: spec.secrets.mode %q: want %s or %s", c.Metadata.Name, spec.Secrets.Mode, ModeSeparate, ModeInline)
	}

	secretsKs := or(spec.Secrets.Kustomization, defaultSecretsKs)
	docs := []any{gitRepository(sourceName, spec.Source)}
	if len(withSecrets) > 0 && !inline {
		docs = append(docs, kustomization(ksSpec{
			name:       secretsKs,
			source:     or(spec.Secrets.SourceRef, defaultClusterSource),
			path:       "./" + path.Join(strings.TrimPrefix(spec.Secrets.Path, "./"), "secrets"),
			timeout:    "2m",
			decryption: or(spec.Secrets.DecryptionSecret, defaultDecryptionSecret),
		}))
	}
	var bundleParams []BundleParams
	for _, b := range sortedKeys(state) {
		bs := state[b]
		sub := map[string]string{bundles[b].sourceVar: sourceName}
		for k, v := range spec.Vars {
			sub[k] = v
		}
		for k, v := range bs.substitute {
			sub[k] = v
		}
		sort.Strings(bs.components)
		params := BundleParams{
			"templateName":  "bundle",
			"name":          or(spec.Bundles[b].Name, b),
			"namespace":     "flux-system",
			"interval":      "1h",
			"retryInterval": "1m",
			"timeout":       "15m",
			"sourceRefName": sourceName,
			"path":          bundles[b].root,
			"components":    bs.components,
			"substitute":    sub,
		}
		// inline: the root Kustomization applies the secrets together with
		// this one; substituteFrom's optional: false covers the gap.
		if bs.secrets && !inline {
			params["dependsOnNames"] = []string{secretsKs}
		}
		bundleParams = append(bundleParams, params)
	}

	res := &Result{Inline: inline, Bundles: bundleParams}
	var err error
	if res.Manifests, err = marshalDocs(docs); err != nil {
		return nil, err
	}
	if len(withSecrets) > 0 {
		if res.ClusterSecrets, res.SecretProfiles, err = secretsInput(c, withSecrets); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// checkSecretOverrides allows overriding only secrets and keys the profile
// declares: the catalog's substituteFrom-keys list is the contract, and an
// extra key would never be read.
func checkSecretOverrides(appName string, ap *AppProfile, app App) error {
	declared := map[string]map[string]bool{}
	for _, s := range ap.Spec.Secrets {
		declared[s.Name] = map[string]bool{}
		for k := range s.Data {
			declared[s.Name][k] = true
		}
	}
	var errs []error
	for _, sName := range sortedKeys(app.Secrets) {
		keys, ok := declared[sName]
		if !ok {
			errs = append(errs, fmt.Errorf("app %s: secret %s is not declared by its AppProfile", appName, sName))
			continue
		}
		for _, k := range sortedKeys(app.Secrets[sName]) {
			if !keys[k] {
				errs = append(errs, fmt.Errorf("app %s: secret %s has no key %s in its AppProfile", appName, sName, k))
			}
		}
	}
	return errors.Join(errs...)
}

// secretsInput renders one SecretProfile per app and a ClusterSecrets that
// enables them and carries the cluster's overrides.
func secretsInput(c *ClusterApps, apps []*AppProfile) ([]byte, map[string][]byte, error) {
	type secret struct {
		Name      string               `yaml:"name"`
		Namespace string               `yaml:"namespace"`
		Type      string               `yaml:"type,omitempty"`
		Data      map[string]yaml.Node `yaml:"data"`
	}
	type doc struct {
		Kind     string   `yaml:"kind"`
		Metadata Metadata `yaml:"metadata"`
		Spec     struct {
			Profiles []string `yaml:"profiles,omitempty"`
			Secrets  []secret `yaml:"secrets,omitempty"`
		} `yaml:"spec"`
	}

	profiles := map[string][]byte{}
	cs := doc{Kind: "ClusterSecrets", Metadata: Metadata{Name: c.Metadata.Name}}
	for _, ap := range apps {
		p := doc{Kind: "SecretProfile", Metadata: Metadata{Name: ap.Metadata.Name}}
		for _, s := range ap.Spec.Secrets {
			p.Spec.Secrets = append(p.Spec.Secrets, secret{Name: s.Name, Namespace: SecretsNamespace, Type: s.Type, Data: s.Data})
		}
		b, err := yaml.Marshal(p)
		if err != nil {
			return nil, nil, err
		}
		profiles[ap.Metadata.Name+".yaml"] = b
		cs.Spec.Profiles = append(cs.Spec.Profiles, ap.Metadata.Name)

		overrides := c.Spec.Apps[ap.Metadata.Name].Secrets
		for _, sName := range sortedKeys(overrides) {
			cs.Spec.Secrets = append(cs.Spec.Secrets, secret{Name: sName, Namespace: SecretsNamespace, Data: overrides[sName]})
		}
	}
	b, err := yaml.Marshal(cs)
	if err != nil {
		return nil, nil, err
	}
	return b, profiles, nil
}

type ksSpec struct {
	name, source, path, timeout, decryption string
}

// The secrets Kustomization; bundles go through KCL (Result.Bundles).
// The manifests are plain maps rather than Flux's Go types: the module stays
// free of the Flux API dependency tree, and the tests pin the exact output.
func kustomization(k ksSpec) map[string]any {
	spec := map[string]any{
		"interval":      "1h",
		"retryInterval": "1m",
		"timeout":       k.timeout,
		"prune":         true,
		"wait":          true,
		"sourceRef":     map[string]any{"kind": "GitRepository", "name": k.source},
		"path":          k.path,
	}
	if k.decryption != "" {
		spec["decryption"] = map[string]any{"provider": "sops", "secretRef": map[string]string{"name": k.decryption}}
	}
	return map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": k.name, "namespace": "flux-system"},
		"spec":       spec,
	}
}

func gitRepository(name string, s Source) map[string]any {
	ref := map[string]string{}
	if s.Tag != "" {
		ref["tag"] = s.Tag
	} else {
		ref["branch"] = s.Branch
	}
	return map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "GitRepository",
		"metadata":   map[string]any{"name": name, "namespace": "flux-system"},
		"spec": map[string]any{
			"interval": or(s.Interval, "10m"),
			"url":      s.URL,
			"ref":      ref,
		},
	}
}

func marshalDocs(docs []any) ([]byte, error) {
	var buf bytes.Buffer
	for _, d := range docs {
		b, err := yaml.Marshal(d)
		if err != nil {
			return nil, err
		}
		buf.WriteString("---\n")
		buf.Write(b)
	}
	return buf.Bytes(), nil
}

func strictDecode(b []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(v)
}

func or(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func bundleNames() []string {
	return sortedKeys(bundles)
}

package clusterapps

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const keycloakProfile = `
kind: AppProfile
metadata: { name: keycloak }
spec:
  bundle: apps-platform
  component: ../components/keycloak
  vars:
    KEYCLOAK_STORAGE_CLASS: { required: true }
    KEYCLOAK_HOSTNAME: { default: keycloak }
    KEYCLOAK_VERSION: {}
  secrets:
    - name: keycloak-secrets
      data:
        ADMIN_USER: admin
        ADMIN_PASSWORD: { generate: { type: password, length: 24 } }
`

const headlampProfile = `
kind: AppProfile
metadata: { name: headlamp }
spec:
  bundle: apps-platform
  component: ../components/headlamp
  vars:
    KEYCLOAK_HOSTNAME: {}
`

const cluster = `
kind: ClusterApps
metadata: { name: cicd-test4 }
spec:
  source: { url: https://github.com/stuttgart-things/flux.git, tag: v1.99.2 }
  secrets:
    path: ./secrets/clusters/cicd-test4
    decryptionSecret: sops-age-apps
  bundles:
    apps-platform: { name: apps-mvp }
  vars:
    INFRA_DOMAIN: cicd-test4.example.com
  apps:
    keycloak:
      vars:
        KEYCLOAK_STORAGE_CLASS: openebs-hostpath
      secrets:
        keycloak-secrets:
          ADMIN_PASSWORD: ref+vault://secret/kc#/admin
`

func build(t *testing.T, clusterYAML string, profiles ...string) (*Result, error) {
	t.Helper()
	c, err := ParseClusterApps([]byte(clusterYAML))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for i, p := range profiles {
		files[string(rune('a'+i))+".yaml"] = []byte(p)
	}
	ps, err := ParseProfiles(files)
	if err != nil {
		t.Fatal(err)
	}
	return Build(c, ps)
}

func docs(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, part := range strings.Split(string(b), "---\n") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		var m map[string]any
		if err := yaml.Unmarshal([]byte(part), &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func dig(m any, keys ...string) any {
	for _, k := range keys {
		mm, ok := m.(map[string]any)
		if !ok {
			return nil
		}
		m = mm[k]
	}
	return m
}

func TestBuildRendersSourceSecretsAndBundle(t *testing.T) {
	res, err := build(t, cluster, keycloakProfile)
	if err != nil {
		t.Fatal(err)
	}
	d := docs(t, res.Manifests)
	if len(d) != 2 {
		t.Fatalf("want GitRepository + secrets Kustomization, got %d docs:\n%s", len(d), res.Manifests)
	}

	if d[0]["kind"] != "GitRepository" || dig(d[0], "metadata", "name") != "flux-apps" || dig(d[0], "spec", "ref", "tag") != "v1.99.2" {
		t.Errorf("source: %v", d[0])
	}

	sec := d[1]
	if dig(sec, "metadata", "name") != "cluster-secrets" ||
		dig(sec, "spec", "path") != "./secrets/clusters/cicd-test4/secrets" ||
		dig(sec, "spec", "sourceRef", "name") != "flux-system" ||
		dig(sec, "spec", "decryption", "secretRef", "name") != "sops-age-apps" {
		t.Errorf("secrets kustomization: %v", sec)
	}

	if len(res.Bundles) != 1 {
		t.Fatalf("want 1 bundle, got %v", res.Bundles)
	}
	b := res.Bundles[0]
	for k, v := range map[string]any{
		"templateName":  "bundle",
		"name":          "apps-mvp",
		"namespace":     "flux-system",
		"path":          "./apps/platform/root",
		"sourceRefName": "flux-apps",
		"timeout":       "15m",
	} {
		if b[k] != v {
			t.Errorf("bundle %s = %v, want %v", k, b[k], v)
		}
	}
	if c := b["components"].([]string); len(c) != 1 || c[0] != "../components/keycloak" {
		t.Errorf("components: %v", c)
	}
	if deps := b["dependsOnNames"].([]string); len(deps) != 1 || deps[0] != "cluster-secrets" {
		t.Errorf("dependsOnNames: %v", deps)
	}
	want := map[string]string{
		"APPS_SOURCE":            "flux-apps",
		"INFRA_DOMAIN":           "cicd-test4.example.com",
		"KEYCLOAK_STORAGE_CLASS": "openebs-hostpath",
		"KEYCLOAK_HOSTNAME":      "keycloak", // profile default
	}
	sub := b["substitute"].(map[string]string)
	if len(sub) != len(want) {
		t.Errorf("substitute = %v, want %v", sub, want)
	}
	for k, v := range want {
		if sub[k] != v {
			t.Errorf("substitute[%s] = %v, want %v", k, sub[k], v)
		}
	}

	// The parameters file must keep the structure: that is why it is a file.
	out, err := yaml.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"components:\n    - ../components/keycloak", "substitute:\n    APPS_SOURCE: flux-apps"} {
		if !strings.Contains(string(out), w) {
			t.Errorf("parameters file lacks %q:\n%s", w, out)
		}
	}
}

func TestBuildSecretsInput(t *testing.T) {
	res, err := build(t, cluster, keycloakProfile)
	if err != nil {
		t.Fatal(err)
	}
	var cs struct {
		Kind     string
		Metadata Metadata
		Spec     struct {
			Profiles []string
			Secrets  []struct {
				Name, Namespace string
				Data            map[string]string
			}
		}
	}
	if err := yaml.Unmarshal(res.ClusterSecrets, &cs); err != nil {
		t.Fatal(err)
	}
	if cs.Kind != "ClusterSecrets" || cs.Metadata.Name != "cicd-test4" || len(cs.Spec.Profiles) != 1 || cs.Spec.Profiles[0] != "keycloak" {
		t.Errorf("ClusterSecrets: %s", res.ClusterSecrets)
	}
	if len(cs.Spec.Secrets) != 1 || cs.Spec.Secrets[0].Namespace != "flux-system" ||
		cs.Spec.Secrets[0].Data["ADMIN_PASSWORD"] != "ref+vault://secret/kc#/admin" { // pragma: allowlist secret
		t.Errorf("override: %s", res.ClusterSecrets)
	}

	p := string(res.SecretProfiles["keycloak.yaml"])
	for _, want := range []string{"kind: SecretProfile", "name: keycloak-secrets", "namespace: flux-system", "ADMIN_USER: admin", "type: password"} {
		if !strings.Contains(p, want) {
			t.Errorf("SecretProfile lacks %q:\n%s", want, p)
		}
	}
}

func TestBuildWithoutSecretsNeedsNoSecretsPath(t *testing.T) {
	res, err := build(t, `
kind: ClusterApps
metadata: { name: c }
spec:
  source: { url: https://x, branch: main }
  apps: { headlamp: {} }
`, headlampProfile)
	if err != nil {
		t.Fatal(err)
	}
	if res.ClusterSecrets != nil || len(docs(t, res.Manifests)) != 1 || len(res.Bundles) != 1 { // pragma: allowlist secret
		t.Fatalf("want source + one bundle only:\n%s\n%v", res.Manifests, res.Bundles)
	}
	if _, ok := res.Bundles[0]["dependsOnNames"]; ok {
		t.Errorf("bundle without secrets must not depend on the secrets kustomization: %v", res.Bundles[0])
	}
}

func TestBuildErrors(t *testing.T) {
	base := `
kind: ClusterApps
metadata: { name: c }
spec:
  source: { url: https://x, tag: v1 }
  secrets: { path: ./s }
`
	cases := map[string]struct {
		cluster string
		want    []string
	}{
		"unknown app": {base + "  apps: { nope: {} }", []string{"app nope: no AppProfile"}},
		"required var missing": {base + "  apps: { keycloak: {} }",
			[]string{"required var KEYCLOAK_STORAGE_CLASS is not set"}},
		"undeclared var": {base + "  apps: { keycloak: { vars: { KEYCLOAK_STORAGE_CLASS: x, KEYCLAOK_HOSTNAME: y } } }",
			[]string{"var KEYCLAOK_HOSTNAME is not declared"}},
		"undeclared secret key": {base + `  apps:
    keycloak:
      vars: { KEYCLOAK_STORAGE_CLASS: x }
      secrets: { keycloak-secrets: { ADMIN_PASS: y } }`,
			[]string{"secret keycloak-secrets has no key ADMIN_PASS"}},
		"undeclared secret": {base + `  apps:
    keycloak:
      vars: { KEYCLOAK_STORAGE_CLASS: x }
      secrets: { other: { A: y } }`,
			[]string{"secret other is not declared"}},
		"var conflict in one bundle": {base + `  apps:
    keycloak: { vars: { KEYCLOAK_STORAGE_CLASS: x, KEYCLOAK_HOSTNAME: a } }
    headlamp: { vars: { KEYCLOAK_HOSTNAME: b } }`,
			[]string{"conflicts with"}},
		"all errors at once": {base + "  apps: { nope: {}, keycloak: {} }",
			[]string{"app nope", "KEYCLOAK_STORAGE_CLASS"}},
		"no tag or branch": {"kind: ClusterApps\nmetadata: { name: c }\nspec:\n  source: { url: https://x }\n  apps: { headlamp: {} }",
			[]string{"exactly one of tag or branch"}},
		"secrets without path": {"kind: ClusterApps\nmetadata: { name: c }\nspec:\n  source: { url: https://x, tag: v1 }\n  apps: { keycloak: { vars: { KEYCLOAK_STORAGE_CLASS: x } } }",
			[]string{"spec.secrets.path is required"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := build(t, tc.cluster, keycloakProfile, headlampProfile)
			if err == nil {
				t.Fatal("want an error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
		})
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := ParseClusterApps([]byte("kind: ClusterApps\nmetadata: { name: c }\nspec: { aps: {} }")); err == nil {
		t.Error("typo in ClusterApps accepted")
	}
	_, err := ParseProfiles(map[string][]byte{"p.yaml": []byte(strings.Replace(keycloakProfile, "component:", "componnet:", 1))})
	if err == nil {
		t.Error("typo in AppProfile accepted")
	}
}

func TestParseProfilesSkipsOtherKindsAndRejectsDuplicates(t *testing.T) {
	ps, err := ParseProfiles(map[string][]byte{
		"ks.yaml": []byte("apiVersion: v1\nkind: ConfigMap\nmetadata: { name: x }\n---\n" + keycloakProfile),
	})
	if err != nil || len(ps) != 1 || ps["keycloak"] == nil {
		t.Fatalf("got %v, %v", ps, err)
	}
	if _, err := ParseProfiles(map[string][]byte{"a.yaml": []byte(keycloakProfile), "b.yaml": []byte(keycloakProfile)}); err == nil {
		t.Error("duplicate profile accepted")
	}
	if _, err := ParseProfiles(map[string][]byte{"a.yaml": []byte(strings.Replace(keycloakProfile, "apps-platform", "apps-nope", 1))}); err == nil {
		t.Error("unknown bundle accepted")
	}
}

func TestBuildInlineHasNoSecretsKustomization(t *testing.T) {
	res, err := build(t, `
kind: ClusterApps
metadata: { name: edge-01 }
spec:
  source: { url: https://x, tag: v1 }
  secrets: { mode: inline }
  apps: { keycloak: { vars: { KEYCLOAK_STORAGE_CLASS: x } } }
`, keycloakProfile)
	if err != nil {
		t.Fatal(err)
	}
	d := docs(t, res.Manifests)
	if !res.Inline || len(d) != 1 || len(res.Bundles) != 1 || res.Bundles[0]["name"] != "apps-platform" {
		t.Fatalf("want source only in Manifests and one bundle:\n%s\n%v", res.Manifests, res.Bundles)
	}
	if _, ok := res.Bundles[0]["dependsOnNames"]; ok {
		t.Errorf("inline bundle must not depend on a secrets kustomization: %v", res.Bundles[0])
	}
	if res.ClusterSecrets == nil { // pragma: allowlist secret
		t.Error("inline still needs the secrets input")
	}
	for _, f := range []string{"sops-age.enc.yaml", ".sops.yaml"} {
		if !strings.Contains(SourceIgnore, f+"\n") {
			t.Errorf(".sourceignore lacks %s", f)
		}
	}
}

func TestBuildSecretsModeErrors(t *testing.T) {
	for name, secrets := range map[string]string{ // pragma: allowlist secret
		"unknown mode":     "{ mode: sideways }",
		"inline with path": "{ mode: inline, path: ./s }",
		"inline with key":  "{ mode: inline, decryptionSecret: x }",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := build(t, "kind: ClusterApps\nmetadata: { name: c }\nspec:\n  source: { url: https://x, tag: v1 }\n  secrets: "+secrets+"\n  apps: { keycloak: { vars: { KEYCLOAK_STORAGE_CLASS: x } } }", keycloakProfile)
			if err == nil || !strings.Contains(err.Error(), "spec.secrets") {
				t.Errorf("got %v", err)
			}
		})
	}
}

const ociCluster = `
kind: ClusterApps
metadata: { name: edge }
spec:
  source:
    kind: OCIRepository
    name: flux-repo
    url: oci://ghcr.io/stuttgart-things/flux/repo
    tag: v1.117.1
    interval: 1h
  secrets: { mode: inline }
  path: ./clusters/edge
  layers:
    edge-infra:
      dir: infra
      timeout: 15m
      substituteFrom: [{ kind: ConfigMap, name: cluster-vars, optional: false }]
    edge-apps:
      dir: apps
      dependsOn: [edge-infra]
      wait: false
      decryptionSecret: sops-age
      substituteFrom: [{ kind: ConfigMap, name: cluster-vars }]
      labels: { kustomization.stuttgart-things.com/type: apps }
  bundles:
    apps-platform: { layer: edge-apps }
  apps:
    keycloak: { vars: { KEYCLOAK_STORAGE_CLASS: local-path } }
`

func TestBuildOCISourceAndLayers(t *testing.T) {
	res, err := build(t, ociCluster, keycloakProfile)
	if err != nil {
		t.Fatal(err)
	}
	d := docs(t, res.Manifests)
	if len(d) != 3 {
		t.Fatalf("want OCIRepository + 2 layers, got %d docs:\n%s", len(d), res.Manifests)
	}
	if d[0]["kind"] != "OCIRepository" || dig(d[0], "metadata", "name") != "flux-repo" ||
		dig(d[0], "spec", "url") != "oci://ghcr.io/stuttgart-things/flux/repo" ||
		dig(d[0], "spec", "ref", "tag") != "v1.117.1" || dig(d[0], "spec", "interval") != "1h" {
		t.Errorf("source: %v", d[0])
	}

	// sorted by name: edge-apps, edge-infra
	apps, infra := d[1], d[2]
	if dig(apps, "metadata", "name") != "edge-apps" || dig(apps, "spec", "path") != "./clusters/edge/apps" ||
		dig(apps, "spec", "wait") != false || dig(apps, "spec", "timeout") != "5m" ||
		dig(apps, "spec", "decryption", "secretRef", "name") != "sops-age" ||
		dig(apps, "spec", "sourceRef", "name") != "flux-system" {
		t.Errorf("edge-apps: %v", apps)
	}
	if deps, _ := dig(apps, "spec", "dependsOn").([]any); len(deps) != 1 || dig(deps[0], "name") != "edge-infra" {
		t.Errorf("edge-apps dependsOn: %v", dig(apps, "spec", "dependsOn"))
	}
	if sf, _ := dig(apps, "spec", "postBuild", "substituteFrom").([]any); len(sf) != 1 ||
		dig(sf[0], "name") != "cluster-vars" || dig(sf[0], "optional") != nil {
		t.Errorf("edge-apps substituteFrom: %v", sf)
	}
	if dig(apps, "metadata", "labels", "kustomization.stuttgart-things.com/type") != "apps" || dig(infra, "metadata", "labels") != nil {
		t.Errorf("layer labels: %v / %v", dig(apps, "metadata"), dig(infra, "metadata"))
	}
	if dig(infra, "spec", "wait") != true || dig(infra, "spec", "timeout") != "15m" || dig(infra, "spec", "decryption") != nil {
		t.Errorf("edge-infra: %v", infra)
	}
	if sf, _ := dig(infra, "spec", "postBuild", "substituteFrom").([]any); len(sf) != 1 || dig(sf[0], "optional") != false {
		t.Errorf("edge-infra substituteFrom: %v", sf)
	}

	if len(res.Bundles) != 1 {
		t.Fatalf("want 1 bundle, got %v", res.Bundles)
	}
	b := res.Bundles[0]
	if b["sourceRefKind"] != "OCIRepository" || b["sourceRefName"] != "flux-repo" {
		t.Errorf("bundle source: %v", b)
	}
	if sub, _ := b["substitute"].(map[string]string); sub["APPS_SOURCE"] != "flux-repo" {
		t.Errorf("bundle APPS_SOURCE: %v", b["substitute"])
	}
	patches, _ := b["patches"].([]any)
	if len(patches) != 1 || !strings.Contains(dig(patches[0], "patch").(string), "value: OCIRepository") ||
		dig(patches[0], "target", "kind") != "Kustomization" {
		t.Errorf("bundle patches: %v", b["patches"])
	}
	if res.BundleDirs["apps-platform"] != "apps" {
		t.Errorf("bundle dir: %v", res.BundleDirs)
	}
}

func TestBuildGitSourceAddsNoOCIParams(t *testing.T) {
	res, err := build(t, cluster, keycloakProfile)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"sourceRefKind", "patches"} {
		if _, ok := res.Bundles[0][k]; ok {
			t.Errorf("git source must leave %s unset (the existing renders stay identical): %v", k, res.Bundles[0])
		}
	}
	if len(res.BundleDirs) != 0 {
		t.Errorf("no layers, no bundle dirs: %v", res.BundleDirs)
	}
}

func TestBuildLayerErrors(t *testing.T) {
	head := "kind: ClusterApps\nmetadata: { name: c }\nspec:\n  apps: { headlamp: {} }\n"
	git := "  source: { url: https://x, tag: v1 }\n"
	cases := map[string]struct{ spec, want string }{
		"oci without tag":     {"  source: { kind: OCIRepository, url: oci://x }\n", "needs an oci:// url and a tag"},
		"oci with branch":     {"  source: { kind: OCIRepository, url: oci://x, tag: v1, branch: main }\n", "needs an oci:// url and a tag"},
		"oci with git url":    {"  source: { kind: OCIRepository, url: https://x, tag: v1 }\n", "needs an oci:// url and a tag"},
		"unknown source kind": {"  source: { kind: Bucket, url: s3://x, tag: v1 }\n", "spec.source.kind"},
		"layers without path": {git + "  layers: { l: { dir: a } }\n", "needs spec.path"},
		"layer dir escapes":   {git + "  path: ./c\n  layers: { l: { dir: ../a } }\n", "must be a subdirectory"},
		"layer dir empty":     {git + "  path: ./c\n  layers: { l: {} }\n", "must be a subdirectory"},
		"layers share dir":    {git + "  path: ./c\n  layers: { l: { dir: a }, m: { dir: a/ } }\n", "share dir"},
		"bad substituteFrom":  {git + "  path: ./c\n  layers: { l: { dir: a, substituteFrom: [{ kind: Bucket, name: x }] } }\n", "substituteFrom needs"},
		"unknown layer":       {git + "  path: ./c\n  layers: { l: { dir: a } }\n  bundles: { apps-platform: { layer: m } }\n", "unknown layer"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := build(t, head+tc.spec, headlampProfile)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want %q", err, tc.want)
			}
		})
	}
}

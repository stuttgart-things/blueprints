package clustersecrets

import (
	"bytes"
	"crypto/rand"
	"regexp"
	"strings"
	"testing"
)

const testProfile = `
kind: SecretProfile
metadata: { name: platform }
spec:
  secrets:
    - name: harbor-secrets
      data:
        HARBOR_ADMIN_PASSWORD: { generate: { type: password, length: 24 } }
        HARBOR_SECRET_KEY:     { generate: { type: alnum, length: 16 } }
    - name: velero-secrets
      namespace: velero
      data:
        AWS_ACCESS_KEY_ID: ref+sops://secrets/minio.enc.yaml#/accessKey
        AWS_SECRET_ACCESS_KEY: { ref: "ref+vault://secret/minio/edge#/secretKey" }
        BUCKET: velero
`

const testCluster = `
kind: ClusterSecrets
metadata: { name: edge-01 }
spec:
  profiles: [platform]
  secrets:
    - name: harbor-secrets
      data:
        HARBOR_SECRET_KEY: { value: "fixed-for-edge01" }
    - name: grafana
      namespace: monitoring
      type: kubernetes.io/basic-auth
      data:
        username: admin
        password: { generate: { type: hex, length: 40 } }
`

func mergedSpecs(t *testing.T) []SecretSpec {
	t.Helper()
	c, err := ParseClusterSecrets([]byte(testCluster))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseSecretProfile([]byte(testProfile))
	if err != nil {
		t.Fatal(err)
	}
	specs, err := Merge(c, map[string]*SecretProfile{"platform": p}, "flux-system")
	if err != nil {
		t.Fatal(err)
	}
	return specs
}

func TestMerge(t *testing.T) {
	specs := mergedSpecs(t)
	var ids []string
	for _, s := range specs {
		ids = append(ids, s.ID())
	}
	want := "flux-system/harbor-secrets monitoring/grafana velero/velero-secrets"
	if got := strings.Join(ids, " "); got != want {
		t.Fatalf("ids = %q, want %q", got, want)
	}

	harbor := specs[0]
	if harbor.Data["HARBOR_SECRET_KEY"].Value == nil || *harbor.Data["HARBOR_SECRET_KEY"].Value != "fixed-for-edge01" {
		t.Errorf("cluster override not applied: %+v", harbor.Data["HARBOR_SECRET_KEY"])
	}
	if harbor.Data["HARBOR_ADMIN_PASSWORD"].Generate == nil {
		t.Errorf("profile key lost by override")
	}
	if specs[1].Type != "kubernetes.io/basic-auth" || harbor.Type != "Opaque" {
		t.Errorf("types = %q, %q", specs[1].Type, harbor.Type)
	}
	if specs[2].Data["AWS_ACCESS_KEY_ID"].Ref == "" || specs[2].Data["BUCKET"].Value == nil {
		t.Errorf("scalar shorthand not parsed: %+v", specs[2].Data)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"unknown field": `
kind: ClusterSecrets
metadata: { name: x }
spec:
  secrets:
    - name: a
      data: { k: { genrate: { type: hex } } }`,
		"two sources": `
kind: ClusterSecrets
metadata: { name: x }
spec:
  secrets:
    - name: a
      data: { k: { value: v, generate: { type: hex } } }`,
		"bad type": `
kind: ClusterSecrets
metadata: { name: x }
spec:
  secrets:
    - name: a
      data: { k: { generate: { type: words } } }`,
		"bad ref": `
kind: ClusterSecrets
metadata: { name: x }
spec:
  secrets:
    - name: a
      data: { k: "ref+env://FOO#/x" }`,
		"bad name": `
kind: ClusterSecrets
metadata: { name: x }
spec:
  secrets:
    - name: Harbor_Secrets
      data: { k: v }`,
		"missing profile": `
kind: ClusterSecrets
metadata: { name: x }
spec:
  profiles: [nope]`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := ParseClusterSecrets([]byte(doc))
			if err == nil {
				_, err = Merge(c, nil, "flux-system")
			}
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	r, err := ParseRef("ref+vault://secret/minio/edge#/secretKey")
	if err != nil {
		t.Fatal(err)
	}
	if r.VaultAPIPath() != "secret/data/minio/edge" || r.Pointer != "/secretKey" {
		t.Errorf("kv2: %+v -> %s", r, r.VaultAPIPath())
	}
	r, err = ParseRef("ref+vault://kv1/legacy?kv=1#/token")
	if err != nil {
		t.Fatal(err)
	}
	if r.VaultAPIPath() != "kv1/legacy" {
		t.Errorf("kv1 path = %s", r.VaultAPIPath())
	}
	r, err = ParseRef("ref+sops://secrets/a/b.enc.yaml#/x/y")
	if err != nil || r.Path != "secrets/a/b.enc.yaml" || r.Pointer != "/x/y" {
		t.Errorf("sops: %+v %v", r, err)
	}
	for _, bad := range []string{
		"ref+sops://../etc/passwd#/x",
		"ref+sops://a.yaml",
		"ref+vault://nomount#/x",
		"ref+vault://secret/x?kv=3#/x",
		"sops://a.yaml#/x",
	} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("%s: expected error", bad)
		}
	}
}

func TestResolveRefs(t *testing.T) {
	refs := CollectRefs(mergedSpecs(t))
	if got := SopsFiles(refs); len(got) != 1 || got[0] != "secrets/minio.enc.yaml" {
		t.Fatalf("SopsFiles = %v", got)
	}
	if got := VaultPaths(refs); len(got) != 1 || got[0] != "secret/data/minio/edge" {
		t.Fatalf("VaultPaths = %v", got)
	}

	resolved, err := ResolveRefs(refs,
		map[string][]byte{"secrets/minio.enc.yaml": []byte("accessKey: AKIA123\nport: 9000\n")},
		map[string][]byte{"secret/data/minio/edge": []byte(`{"data":{"data":{"secretKey":"s3cr3t"},"metadata":{"version":3}}}`)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["ref+sops://secrets/minio.enc.yaml#/accessKey"] != "AKIA123" ||
		resolved["ref+vault://secret/minio/edge#/secretKey"] != "s3cr3t" { // pragma: allowlist secret
		t.Errorf("resolved = %v", resolved)
	}

	_, err = ResolveRefs([]Ref{mustRef(t, "ref+sops://f.yaml#/missing")}, map[string][]byte{"f.yaml": []byte("a: 1")}, nil)
	if err == nil || !strings.Contains(err.Error(), `"missing" not found`) {
		t.Errorf("missing key: %v", err)
	}
	_, err = ResolveRefs([]Ref{mustRef(t, "ref+sops://f.yaml#/a")}, map[string][]byte{"f.yaml": []byte("a: {b: 1}")}, nil)
	if err == nil {
		t.Error("map value accepted as scalar")
	}
	v, err := ResolveRefs([]Ref{mustRef(t, "ref+sops://f.json#/a~1b/0")}, map[string][]byte{"f.json": []byte(`{"a/b": [42]}`)}, nil)
	if err != nil || v["ref+sops://f.json#/a~1b/0"] != "42" {
		t.Errorf("escaped pointer / array: %v %v", v, err)
	}
}

func mustRef(t *testing.T, s string) Ref {
	t.Helper()
	r, err := ParseRef(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestGenerate(t *testing.T) {
	cases := []struct {
		spec GenerateSpec
		re   string
	}{
		{GenerateSpec{Type: "password", Length: 24}, `^[a-zA-Z0-9\-_.!#%+=]{24}$`},
		{GenerateSpec{Type: "alnum", Length: 16}, `^[a-zA-Z0-9]{16}$`},
		{GenerateSpec{Type: "alnum"}, `^[a-zA-Z0-9]{32}$`},
		{GenerateSpec{Type: "alnum", Length: 8, Charset: "ab"}, `^[ab]{8}$`},
		{GenerateSpec{Type: "hex", Length: 7}, `^[0-9a-f]{7}$`},
		{GenerateSpec{Type: "base64", Length: 32}, `^[A-Za-z0-9+/]{43}=$`},
		{GenerateSpec{Type: "uuid"}, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`},
	}
	for _, c := range cases {
		for i := 0; i < 50; i++ {
			v, err := Generate(c.spec, rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			if !regexp.MustCompile(c.re).MatchString(v) {
				t.Fatalf("%+v -> %q does not match %s", c.spec, v, c.re)
			}
		}
	}
	for i := 0; i < 200; i++ {
		v, _ := Generate(GenerateSpec{Type: "password", Length: 4}, nil)
		for _, class := range []string{lower, upper, digits, symbols} {
			if !strings.ContainsAny(v, class) {
				t.Fatalf("password %q lacks a character from %q", v, class)
			}
		}
	}
}

func TestBuildKeepsAndRotates(t *testing.T) {
	specs := mergedSpecs(t)
	resolved := map[string]string{
		"ref+sops://secrets/minio.enc.yaml#/accessKey": "AKIA123",
		"ref+vault://secret/minio/edge#/secretKey":     "s3cr3t",
	}

	first, err := Build(specs, nil, resolved, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	existing := Existing{}
	for _, s := range first {
		existing[s.ID()] = s.Data
	}

	// A new run with changed references keeps generated values, picks up the
	// new reference value.
	resolved2 := map[string]string{
		"ref+sops://secrets/minio.enc.yaml#/accessKey": "AKIA123",
		"ref+vault://secret/minio/edge#/secretKey":     "rotated-in-vault",
	}
	second, err := Build(specs, existing, resolved2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Data["HARBOR_ADMIN_PASSWORD"] != first[0].Data["HARBOR_ADMIN_PASSWORD"] ||
		second[1].Data["password"] != first[1].Data["password"] {
		t.Error("generated values changed without rotation")
	}
	if second[2].Data["AWS_SECRET_ACCESS_KEY"] != "rotated-in-vault" { // pragma: allowlist secret
		t.Error("ref value not refreshed")
	}

	third, err := Build(specs, existing, resolved, Rotation{"harbor-secrets:HARBOR_ADMIN_PASSWORD"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if third[0].Data["HARBOR_ADMIN_PASSWORD"] == first[0].Data["HARBOR_ADMIN_PASSWORD"] {
		t.Error("rotated value unchanged")
	}
	if third[1].Data["password"] != first[1].Data["password"] {
		t.Error("unrelated value rotated")
	}

	if _, err := Build(specs, existing, resolved, ParseRotation("harbor-secret"), nil); err == nil {
		t.Error("rotate typo accepted")
	}
	if _, err := Build(specs, nil, map[string]string{}, nil, nil); err == nil {
		t.Error("unresolved ref accepted")
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	in := Secret{Name: "a", Namespace: "b", Type: "Opaque", Data: map[string]string{
		"plain":      "x",
		"looksBool":  "true",
		"leading0":   "0123",
		"multiline":  "line1\nline2\n",
		"age.agekey": "AGE-SECRET-KEY-1QQQ",
	}}
	doc, err := RenderSecret(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseSecret(doc)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range in.Data {
		if out.Data[k] != v {
			t.Errorf("%s: %q -> %q\n%s", k, v, out.Data[k], doc)
		}
	}

	// data (base64) is read too, stringData wins on conflict
	out, err = ParseSecret([]byte("kind: Secret\nmetadata: {name: a, namespace: b}\ndata: {k: dg==, both: eA==}\nstringData: {both: y}\n"))
	if err != nil || out.Data["k"] != "v" || out.Data["both"] != "y" {
		t.Errorf("data merge: %+v %v", out.Data, err)
	}
}

func TestAgeKeys(t *testing.T) {
	pub, priv, err := GenerateAgeKey()
	if err != nil {
		t.Fatal(err)
	}
	pub2, priv2, _ := GenerateAgeKey()
	if pub == pub2 || priv == priv2 {
		t.Fatal("two generated keys are equal")
	}
	got, err := AgePublicKey("# created: 2026-01-01\n# public key: " + pub + "\n" + priv + "\n")
	if err != nil || got != pub {
		t.Fatalf("AgePublicKey = %q, %v; want %q", got, err, pub)
	}
	if _, err := AgePublicKey(priv + "\n" + priv2 + "\n"); err == nil {
		t.Error("two identities accepted")
	}
	if _, err := AgePublicKey("# only a comment\n"); err == nil {
		t.Error("empty key file accepted")
	}
	if r, err := ParseRecipients(pub + ", " + pub2 + ","); err != nil || len(r) != 2 {
		t.Errorf("ParseRecipients: %v %v", r, err)
	}
	if _, err := ParseRecipients("age1short"); err == nil {
		t.Error("bad recipient accepted")
	}
}

func TestUnchanged(t *testing.T) {
	enc := []byte("stringData:\n  k: ENC[...]\nsops:\n  age:\n    - recipient: age1b\n      enc: x\n    - recipient: age1a\n      enc: y\n")
	s := Secret{Name: "a", Namespace: "b", Type: "Opaque", Data: map[string]string{"k": "v"}}
	if !Unchanged(s, enc, s, []string{"age1a", "age1b"}) {
		t.Error("identical secret and recipients reported as changed")
	}
	if Unchanged(s, enc, s, []string{"age1a"}) {
		t.Error("recipient removal not detected")
	}
	s2 := s
	s2.Data = map[string]string{"k": "v2"}
	if Unchanged(s, enc, s2, []string{"age1a", "age1b"}) {
		t.Error("value change not detected")
	}
	s2.Data = map[string]string{"k": "v", "extra": ""}
	if Unchanged(s, enc, s2, []string{"age1a", "age1b"}) {
		t.Error("added key not detected")
	}
}

func TestRenderSopsConfigAndKustomization(t *testing.T) {
	cfg := string(RenderSopsConfig([]string{"age1c"}, []string{"age1m", "age1e"}))
	for _, want := range []string{`sops-age\.enc\.yaml$`, "age: age1m,age1e", "age: age1c", EncryptedRegex} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".sops.yaml lacks %q:\n%s", want, cfg)
		}
	}
	k := RenderKustomization([]string{"b/x.enc.yaml", "a/y.enc.yaml"})
	if !bytes.Contains(k, []byte("  - a/y.enc.yaml\n  - b/x.enc.yaml\n")) {
		t.Errorf("kustomization:\n%s", k)
	}
}

// Generated passwords start with '!', '#', '%' or '-' often enough that a
// YAML quoting bug would show up here.
func TestRenderRoundTripGenerated(t *testing.T) {
	for i := 0; i < 2000; i++ {
		v, err := Generate(GenerateSpec{Type: "password", Length: 6}, nil)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := RenderSecret(Secret{Name: "a", Namespace: "b", Data: map[string]string{"k": v}})
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseSecret(doc)
		if err != nil || got.Data["k"] != v {
			t.Fatalf("%q -> %q (%v)\n%s", v, got.Data["k"], err, doc)
		}
	}
}

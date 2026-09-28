// Tests for the secrets module's GenerateClusterSecrets, run end to end
// against a real Vault started as a Dagger service (seeded by
// vault-seed.sh). Needs nothing from the host: every key is generated here.
//
//	dagger call -m secrets/tests all --progress plain
package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"

	"dagger/tests/internal/dagger"

	"gopkg.in/yaml.v3"
)

//go:embed vault-seed.sh
var vaultSeed string

const (
	vaultImage = "hashicorp/vault:1.20"
	toolImage  = "cgr.dev/chainguard/wolfi-base:latest"
)

// Every file GenerateClusterSecrets writes for edge-01.
var edge01Files = []string{
	".sops.yaml",
	"age.pub",
	"sops-age.enc.yaml",
	"secrets/kustomization.yaml",
	"secrets/flux-system/harbor-secrets.enc.yaml",
	"secrets/monitoring/alertmanager-secrets.enc.yaml",
	"secrets/monitoring/grafana-admin.enc.yaml",
	"secrets/velero/velero-secrets.enc.yaml",
}

type Tests struct{}

type fixture struct {
	master, escrow       *dagger.Secret
	masterPub, escrowPub string
	testdata             *dagger.Directory
	refs                 *dagger.Directory
	vault                *dagger.Service
}

// All runs every scenario and returns one PASS line per scenario.
//
// +cache="never"
func (t *Tests) All(ctx context.Context) (string, error) {
	f, err := setup(ctx)
	if err != nil {
		return "", fmt.Errorf("setup: %w", err)
	}

	first := f.generate("edge-01.yaml", dagger.SecretsGenerateClusterSecretsOpts{
		VaultService:  f.vault,
		VaultRoleID:   dag.SetSecret("vault-role-id", "test-role"),
		VaultSecretID: dag.SetSecret("vault-secret-id", "test-secret"),
	})

	scenarios := []struct {
		name string
		run  func() error
	}{
		{"fresh run resolves generate, sops and vault (approle)", func() error { return f.checkFresh(ctx, first) }},
		{"key file is for master+escrow only, secrets for cluster+escrow", func() error { return f.checkRecipients(ctx, first) }},
		{"re-run with existing output changes no file", func() error { return f.checkRerun(ctx, first) }},
		{"rotate regenerates exactly the named value", func() error { return f.checkRotate(ctx, first) }},
		{"cluster-age-key returns the key recorded in age.pub", func() error { return f.checkClusterAgeKey(ctx, first) }},
		{"edge cluster without vault works, with its own key", func() error { return f.checkNoVault(ctx, first) }},
		{"vault ref without vault connection fails clearly", func() error { return f.checkVaultMissing(ctx) }},
		{"wrong master key cannot take over existing output", func() error { return f.checkWrongMaster(ctx, first) }},
	}

	var report []string
	for _, s := range scenarios {
		if err := s.run(); err != nil {
			return strings.Join(report, "\n"), fmt.Errorf("FAIL %s: %w", s.name, err)
		}
		report = append(report, "PASS "+s.name)
	}
	return strings.Join(report, "\n"), nil
}

// VaultService is the seeded Vault the tests use, exposed for poking at it:
//
//	dagger call -m secrets/tests vault-service up --ports 8200:8200
func (t *Tests) VaultService() *dagger.Service {
	return dag.Container().
		From(vaultImage).
		WithEnvVariable("SKIP_SETCAP", "1").
		WithNewFile("/seed.sh", vaultSeed).
		WithExposedPort(8200).
		AsService(dagger.ContainerAsServiceOpts{Args: []string{"sh", "/seed.sh"}})
}

func setup(ctx context.Context) (*fixture, error) {
	f := &fixture{testdata: dag.CurrentModule().Source().Directory("testdata")}
	var err error
	if f.master, f.masterPub, err = ageKey(ctx, "master"); err != nil {
		return nil, err
	}
	if f.escrow, f.escrowPub, err = ageKey(ctx, "escrow"); err != nil {
		return nil, err
	}

	// The central store: encrypted for the master key only.
	central, err := dag.Secrets().EncryptString(ctx,
		dag.SetSecret("master-pub", f.masterPub),
		"accessKey: AKIA-central\nsecretKey: secret-from-central-sops\n",
		dagger.SecretsEncryptStringOpts{FileExtension: "yaml"})
	if err != nil {
		return nil, fmt.Errorf("encrypt central file: %w", err)
	}
	f.refs = dag.Directory().WithNewFile("secrets/minio.enc.yaml", central)

	// Start Vault once and wait for the seed's last write; every later
	// binding of the same service reuses this instance.
	if f.vault, err = (&Tests{}).VaultService().Start(ctx); err != nil {
		return nil, fmt.Errorf("start vault: %w", err)
	}
	_, err = dag.Container().
		From(toolImage).
		WithExec([]string{"apk", "add", "--no-cache", "curl"}).
		WithServiceBinding("vault", f.vault).
		WithEnvVariable("CACHEBUST", nonce()).
		WithExec([]string{"sh", "-c", `
for i in $(seq 1 60); do
  curl -sf -H "X-Vault-Token: root" http://vault:8200/v1/secret/data/ready >/dev/null && exit 0
  sleep 1
done
echo "vault was not seeded within 60s" >&2
exit 1`}).
		Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("wait for vault seed: %w", err)
	}
	return f, nil
}

func (f *fixture) generate(cluster string, opts dagger.SecretsGenerateClusterSecretsOpts) *dagger.Directory {
	opts.ProfileDir = f.testdata.Directory("profiles")
	opts.SopsRefDir = f.refs
	opts.EscrowRecipients = f.escrowPub
	return dag.Secrets().GenerateClusterSecrets(f.testdata.File(cluster), f.master, opts)
}

func (f *fixture) checkFresh(ctx context.Context, out *dagger.Directory) error {
	for _, p := range edge01Files {
		if _, err := out.File(p).Contents(ctx); err != nil {
			return fmt.Errorf("missing %s: %w", p, err)
		}
	}
	key, err := f.clusterKey(ctx, out)
	if err != nil {
		return err
	}

	harbor, err := stringData(ctx, key, out.File("secrets/flux-system/harbor-secrets.enc.yaml"))
	if err != nil {
		return err
	}
	if len(harbor["HARBOR_ADMIN_PASSWORD"]) != 24 || len(harbor["HARBOR_SECRET_KEY"]) != 16 {
		return fmt.Errorf("harbor lengths: %d, %d", len(harbor["HARBOR_ADMIN_PASSWORD"]), len(harbor["HARBOR_SECRET_KEY"]))
	}

	return expect(ctx, key, out, map[string]map[string]string{
		"secrets/velero/velero-secrets.enc.yaml": {
			"AWS_ACCESS_KEY_ID":     "AKIA-central",     // ref+sops
			"AWS_SECRET_ACCESS_KEY": "minio-from-vault", // ref+vault, KV v2
			"BUCKET":                "velero",           // literal
		},
		"secrets/monitoring/alertmanager-secrets.enc.yaml": {
			"MSTEAMS_WEBHOOK": "https://example.invalid/hook",
			"LEGACY_TOKEN":    "kv1-token", // KV v1
		},
		"secrets/monitoring/grafana-admin.enc.yaml": {"username": "admin"},
	})
}

func (f *fixture) checkRecipients(ctx context.Context, out *dagger.Directory) error {
	key, err := f.clusterKey(ctx, out)
	if err != nil {
		return err
	}
	if _, err := dag.Secrets().Decrypt(ctx, key, out.File("sops-age.enc.yaml")); err == nil {
		return fmt.Errorf("the cluster key decrypted its own key file; it must be readable by master/escrow only")
	}
	if _, err := dag.Secrets().Decrypt(ctx, f.escrow, out.File("sops-age.enc.yaml")); err != nil {
		return fmt.Errorf("escrow key cannot decrypt the key file: %w", err)
	}
	if _, err := dag.Secrets().Decrypt(ctx, f.escrow, out.File("secrets/velero/velero-secrets.enc.yaml")); err != nil {
		return fmt.Errorf("escrow key cannot decrypt a secret: %w", err)
	}
	if _, err := dag.Secrets().Decrypt(ctx, f.master, out.File("secrets/velero/velero-secrets.enc.yaml")); err == nil {
		return fmt.Errorf("the master key decrypted a cluster secret; only cluster+escrow should")
	}
	enc, err := out.File("secrets/velero/velero-secrets.enc.yaml").Contents(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(enc, "AKIA-central") || !strings.Contains(enc, "name: velero-secrets") {
		return fmt.Errorf("expected encrypted values and readable metadata:\n%s", enc)
	}
	return nil
}

func (f *fixture) checkRerun(ctx context.Context, first *dagger.Directory) error {
	second := f.generate("edge-01.yaml", dagger.SecretsGenerateClusterSecretsOpts{
		Existing:     first,
		VaultService: f.vault,
		VaultToken:   dag.SetSecret("vault-token", "root"),
	})
	changed, err := diff(ctx, first, second)
	if err != nil {
		return err
	}
	if len(changed) > 0 {
		return fmt.Errorf("files changed without any input change: %v", changed)
	}
	return nil
}

func (f *fixture) checkRotate(ctx context.Context, first *dagger.Directory) error {
	rotated := f.generate("edge-01.yaml", dagger.SecretsGenerateClusterSecretsOpts{
		Existing:     first,
		VaultService: f.vault,
		VaultToken:   dag.SetSecret("vault-token", "root"),
		Rotate:       "harbor-secrets:HARBOR_ADMIN_PASSWORD",
	})
	changed, err := diff(ctx, first, rotated)
	if err != nil {
		return err
	}
	if len(changed) != 1 || changed[0] != "secrets/flux-system/harbor-secrets.enc.yaml" {
		return fmt.Errorf("changed files = %v, want only harbor-secrets", changed)
	}
	key, err := f.clusterKey(ctx, first)
	if err != nil {
		return err
	}
	before, err := stringData(ctx, key, first.File("secrets/flux-system/harbor-secrets.enc.yaml"))
	if err != nil {
		return err
	}
	after, err := stringData(ctx, key, rotated.File("secrets/flux-system/harbor-secrets.enc.yaml"))
	if err != nil {
		return err
	}
	if before["HARBOR_ADMIN_PASSWORD"] == after["HARBOR_ADMIN_PASSWORD"] {
		return fmt.Errorf("HARBOR_ADMIN_PASSWORD not rotated")
	}
	if before["HARBOR_SECRET_KEY"] != after["HARBOR_SECRET_KEY"] {
		return fmt.Errorf("HARBOR_SECRET_KEY rotated although not named")
	}
	return nil
}

func (f *fixture) checkClusterAgeKey(ctx context.Context, out *dagger.Directory) error {
	key := dag.Secrets().ClusterAgeKey(out, f.master)
	_, err := dag.Secrets().Decrypt(ctx, key, out.File("secrets/monitoring/grafana-admin.enc.yaml"))
	return err
}

func (f *fixture) checkNoVault(ctx context.Context, edge01 *dagger.Directory) error {
	out := f.generate("edge-02-novault.yaml", dagger.SecretsGenerateClusterSecretsOpts{})
	key, err := f.clusterKey(ctx, out)
	if err != nil {
		return err
	}
	if err := expect(ctx, key, out, map[string]map[string]string{
		"secrets/velero/velero-secrets.enc.yaml": {
			"AWS_ACCESS_KEY_ID":     "AKIA-central",
			"AWS_SECRET_ACCESS_KEY": "secret-from-central-sops",
		},
	}); err != nil {
		return err
	}

	pub1, err := edge01.File("age.pub").Contents(ctx)
	if err != nil {
		return err
	}
	pub2, err := out.File("age.pub").Contents(ctx)
	if err != nil {
		return err
	}
	if pub1 == pub2 {
		return fmt.Errorf("edge-01 and edge-02 got the same cluster key %s", pub1)
	}
	if _, err := dag.Secrets().Decrypt(ctx, key, edge01.File("secrets/velero/velero-secrets.enc.yaml")); err == nil {
		return fmt.Errorf("edge-02's key decrypted an edge-01 secret")
	}
	return nil
}

func (f *fixture) checkVaultMissing(ctx context.Context) error {
	_, err := f.generate("edge-03-vault-missing.yaml", dagger.SecretsGenerateClusterSecretsOpts{}).Sync(ctx)
	if err == nil {
		return fmt.Errorf("succeeded without a Vault connection")
	}
	if !strings.Contains(err.Error(), "neither vaultAddr nor vaultService") {
		return fmt.Errorf("unhelpful error: %w", err)
	}
	return nil
}

func (f *fixture) checkWrongMaster(ctx context.Context, first *dagger.Directory) error {
	other, _, err := ageKey(ctx, "other")
	if err != nil {
		return err
	}
	_, err = dag.Secrets().GenerateClusterSecrets(f.testdata.File("edge-02-novault.yaml"), other,
		dagger.SecretsGenerateClusterSecretsOpts{
			ProfileDir: f.testdata.Directory("profiles"),
			Existing:   first,
		}).Sync(ctx)
	if err == nil {
		return fmt.Errorf("a foreign master key re-used existing output")
	}
	if !strings.Contains(err.Error(), "decrypt cluster key with master key") {
		return fmt.Errorf("unexpected error: %w", err)
	}
	return nil
}

// clusterKey decrypts the cluster's private key with the master key.
func (f *fixture) clusterKey(ctx context.Context, out *dagger.Directory) (*dagger.Secret, error) {
	data, err := stringData(ctx, f.master, out.File("sops-age.enc.yaml"))
	if err != nil {
		return nil, fmt.Errorf("decrypt key file with master key: %w", err)
	}
	k := strings.TrimSpace(data["age.agekey"])
	if !strings.HasPrefix(k, "AGE-SECRET-KEY-1") {
		return nil, fmt.Errorf("sops-age.enc.yaml holds no age key")
	}
	return dag.SetSecret("cluster-key-"+nonce(), k), nil
}

func stringData(ctx context.Context, key *dagger.Secret, f *dagger.File) (map[string]string, error) {
	plain, err := dag.Secrets().Decrypt(ctx, key, f)
	if err != nil {
		return nil, err
	}
	var doc struct {
		StringData map[string]string `yaml:"stringData"`
	}
	if err := yaml.Unmarshal([]byte(plain), &doc); err != nil {
		return nil, err
	}
	return doc.StringData, nil
}

func expect(ctx context.Context, key *dagger.Secret, out *dagger.Directory, want map[string]map[string]string) error {
	for p, kv := range want {
		got, err := stringData(ctx, key, out.File(p))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		for k, v := range kv {
			if got[k] != v {
				return fmt.Errorf("%s: %s = %q, want %q", p, k, got[k], v)
			}
		}
	}
	return nil
}

// diff lists the edge-01 files whose bytes differ between a and b.
func diff(ctx context.Context, a, b *dagger.Directory) ([]string, error) {
	var changed []string
	for _, p := range edge01Files {
		ca, err := a.File(p).Contents(ctx)
		if err != nil {
			return nil, err
		}
		cb, err := b.File(p).Contents(ctx)
		if err != nil {
			return nil, err
		}
		if ca != cb {
			changed = append(changed, p)
		}
	}
	return changed, nil
}

// ageKey generates a key pair with age-keygen; the nonce keeps two calls
// from being served the same cached key.
func ageKey(ctx context.Context, name string) (*dagger.Secret, string, error) {
	content, err := dag.Container().
		From(toolImage).
		WithExec([]string{"apk", "add", "--no-cache", "age"}).
		WithEnvVariable("CACHEBUST", nonce()).
		WithExec([]string{"age-keygen", "-o", "/key.txt"}).
		File("/key.txt").
		Contents(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("age-keygen %s: %w", name, err)
	}
	var pub string
	for _, line := range strings.Split(content, "\n") {
		if p, ok := strings.CutPrefix(line, "# public key: "); ok {
			pub = p
		}
	}
	if pub == "" {
		return nil, "", fmt.Errorf("age-keygen %s: no public key in output", name)
	}
	return dag.SetSecret(name+"-age-key-"+nonce(), content), pub, nil
}

func nonce() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

package clustersecrets

// Tests against the real tools. They skip unless the tools are there:
//
//	sops + age-keygen in PATH        -> TestSopsRoundTrip
//	CLUSTERSECRETS_VAULT_ADDR set    -> TestVaultFetchScript (a Vault dev
//	                                    server with root token "root", seeded
//	                                    by tests/vault-seed.sh; the commands
//	                                    are in that script's header)
//
// The Dagger test module (secrets/tests) runs the same flow end to end
// with Vault as a service; these are the fast, engine-free subset.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}

func need(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not in PATH", tool)
		}
	}
}

func run(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func keygen(t *testing.T, dir, name string) (pub, keyFile string) {
	t.Helper()
	keyFile = filepath.Join(dir, name+".key")
	if out, err := run(t, nil, "age-keygen", "-o", keyFile); err != nil {
		t.Fatalf("age-keygen: %v\n%s", err, out)
	}
	content, _ := os.ReadFile(keyFile)
	pub, err := AgePublicKey(string(content))
	if err != nil {
		t.Fatal(err)
	}
	return pub, keyFile
}

func TestSopsRoundTrip(t *testing.T) {
	need(t, "sops", "age-keygen")
	dir := t.TempDir()
	clusterPub, clusterKey := keygen(t, dir, "cluster")
	escrowPub, escrowKey := keygen(t, dir, "escrow")
	_, otherKey := keygen(t, dir, "other")

	in := Secret{Name: "harbor-secrets", Namespace: "flux-system", Type: "Opaque",
		Data: map[string]string{"HARBOR_ADMIN_PASSWORD": "p@ss!word", "multi": "a\nb\n"}}
	doc, err := RenderSecret(in)
	if err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain.yaml")
	enc := filepath.Join(dir, "harbor-secrets.enc.yaml")
	mustWrite(t, plain, doc, 0o600)

	if out, err := run(t, nil, SopsEncryptArgs([]string{clusterPub, escrowPub}, plain, enc)...); err != nil {
		t.Fatalf("encrypt: %v\n%s", err, out)
	}
	encrypted, _ := os.ReadFile(enc)
	if strings.Contains(string(encrypted), "p@ss!word") {
		t.Fatal("plaintext value in encrypted file")
	}
	if !strings.Contains(string(encrypted), "name: harbor-secrets") {
		t.Error("metadata should stay readable (encrypted_regex)")
	}
	if !Unchanged(in, encrypted, in, []string{escrowPub, clusterPub}) {
		t.Error("recipients not read back from real sops metadata")
	}

	for _, key := range []string{clusterKey, escrowKey} {
		dec := filepath.Join(dir, "dec.yaml")
		if out, err := run(t, []string{"SOPS_AGE_KEY_FILE=" + key}, SopsDecryptArgs(enc, dec)...); err != nil {
			t.Fatalf("decrypt with %s: %v\n%s", filepath.Base(key), err, out)
		}
		raw, _ := os.ReadFile(dec)
		got, err := ParseSecret(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got.Data["HARBOR_ADMIN_PASSWORD"] != "p@ss!word" || got.Data["multi"] != "a\nb\n" { // pragma: allowlist secret
			t.Errorf("decrypted with %s: %+v", filepath.Base(key), got.Data)
		}
	}

	if _, err := run(t, []string{"SOPS_AGE_KEY_FILE=" + otherKey}, SopsDecryptArgs(enc, filepath.Join(dir, "x.yaml"))...); err == nil {
		t.Error("a key that is not a recipient decrypted the file")
	}
}

func TestVaultFetchScript(t *testing.T) {
	addr := os.Getenv("CLUSTERSECRETS_VAULT_ADDR")
	if addr == "" {
		t.Skip("CLUSTERSECRETS_VAULT_ADDR not set")
	}
	need(t, "sh", "curl", "jq")
	dir := t.TempDir()
	script := filepath.Join(dir, "fetch.sh")
	mustWrite(t, script, []byte(VaultFetchScript), 0o700)

	refs := []Ref{
		mustRef(t, "ref+vault://secret/edge/teams#/webhook"),
		mustRef(t, "ref+vault://secret/edge/minio#/secretKey"),
		mustRef(t, "ref+vault://kv1/legacy?kv=1#/token"),
	}
	paths := filepath.Join(dir, "paths")
	mustWrite(t, paths, []byte(strings.Join(VaultPaths(refs), "\n")), 0o600)

	fetch := func(env ...string) (map[string]string, error) {
		out, err := run(t, append([]string{"VAULT_ADDR=" + addr}, env...), "sh", script, paths)
		if err != nil {
			return nil, &scriptErr{out}
		}
		docs, err := ParseVaultBatch([]byte(out))
		if err != nil {
			return nil, err
		}
		return ResolveRefs(refs, nil, docs)
	}

	want := map[string]string{
		"ref+vault://secret/edge/teams#/webhook":   "https://example.invalid/hook",
		"ref+vault://secret/edge/minio#/secretKey": "minio-from-vault",
		"ref+vault://kv1/legacy?kv=1#/token":       "kv1-token",
	}
	check := func(name string, got map[string]string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %s = %q, want %q", name, k, got[k], v)
			}
		}
	}

	got, err := fetch("VAULT_TOKEN=root")
	check("token", got, err)
	got, err = fetch("VAULT_ROLE_ID=test-role", "VAULT_SECRET_ID=test-secret")
	check("approle", got, err)

	mustWrite(t, paths, []byte("secret/data/does/not/exist\n"), 0o600)
	if _, err := fetch("VAULT_TOKEN=root"); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("missing path: %v", err)
	}
	if _, err := fetch("VAULT_ROLE_ID=test-role", "VAULT_SECRET_ID=wrong"); err == nil || !strings.Contains(err.Error(), "approle login") {
		t.Errorf("bad approle: %v", err)
	}
}

type scriptErr struct{ out string }

func (e *scriptErr) Error() string { return e.out }

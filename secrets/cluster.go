package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"

	"dagger/secrets/clustersecrets"
	"dagger/secrets/internal/dagger"
)

// Same base as dagger/sops, plus the CLIs the cluster-secrets flow needs.
const toolboxImage = "cgr.dev/chainguard/wolfi-base:latest"

// GenerateClusterSecrets renders every Secret a cluster needs from a
// ClusterSecrets profile and returns them SOPS-encrypted with a key of that
// cluster's own:
//
//	.sops.yaml                          rules for editing the output with plain sops
//	age.pub                             the cluster's public key
//	sops-age.enc.yaml                   flux-system/sops-age with the cluster's private
//	                                    key, encrypted for the master key + escrow only
//	secrets/kustomization.yaml          lists every secret below
//	secrets/<namespace>/<name>.enc.yaml encrypted for the cluster key + escrow
//
// Pass the previous output as `existing` on every later run: the cluster
// key and every generated value are then kept (unless named in `rotate`),
// and files whose content and recipients did not change are returned
// byte-for-byte, so a re-run yields an empty diff. Export with --wipe so
// secrets removed from the profile disappear.
//
// Values come from `generate:` (crypto/rand), `value:` literals,
// `ref+sops://<path>#/<pointer>` (files below sopsRefDir, decrypted with
// the master key) or `ref+vault://<mount>/<path>#/<pointer>`. Vault is only
// contacted when the profile references it, so a cluster without Vault
// needs nothing but the master key.
//
// Usage:
//
//	dagger call -m secrets generate-cluster-secrets \
//	  --cluster-profile clusters/edge-01/cluster-secrets.yaml \
//	  --profile-dir profiles/secrets \
//	  --master-age-key file:~/.config/sops/age/master.txt \
//	  --escrow-recipients age1... \
//	  --existing clusters/edge-01/cluster-secrets \
//	  --sops-ref-dir . \
//	  export --path clusters/edge-01/cluster-secrets --wipe
//
// Cached per session only: within one `dagger call` every use of the result
// must see the same run (one key, one set of values), while the next call
// must read Vault again.
// +cache="session"
func (m *Secrets) GenerateClusterSecrets(
	ctx context.Context,
	// ClusterSecrets document (kind: ClusterSecrets)
	clusterProfile *dagger.File,
	// AGE key of the CI / key custodian (key-file format, one identity).
	// Encrypts and decrypts the cluster key; decrypts ref+sops:// files.
	masterAgeKey *dagger.Secret,
	// Directory searched (recursively) for SecretProfile documents
	// +optional
	profileDir *dagger.Directory,
	// Comma-separated break-glass AGE public keys every file is also encrypted for
	// +optional
	escrowRecipients string,
	// Output of a previous run for this cluster
	// +optional
	existing *dagger.Directory,
	// Base directory for ref+sops:// paths
	// +optional
	sopsRefDir *dagger.Directory,
	// Vault address; defaults to http://vault:8200 when vaultService is set
	// +optional
	vaultAddr string,
	// Vault as a Dagger service, bound under the hostname "vault" (tests, local runs)
	// +optional
	vaultService *dagger.Service,
	// Vault token
	// +optional
	vaultToken *dagger.Secret,
	// Vault AppRole role ID (used when no token is given)
	// +optional
	vaultRoleID *dagger.Secret,
	// Vault AppRole secret ID
	// +optional
	vaultSecretID *dagger.Secret,
	// CA certificate for a Vault with a private PKI
	// +optional
	vaultCACert *dagger.File,
	// Generated values to regenerate: comma-separated <secret> or <secret>:<key>
	// +optional
	rotate string,
	// Namespace for secrets that do not set one
	// +optional
	// +default="flux-system"
	defaultNamespace string,
) (*dagger.Directory, error) {
	if defaultNamespace == "" {
		defaultNamespace = clustersecrets.AgeKeyNamespace
	}

	raw, err := clusterProfile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: read cluster profile: %w", err)
	}
	cluster, err := clustersecrets.ParseClusterSecrets([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	profiles, err := loadProfiles(ctx, profileDir)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	specs, err := clustersecrets.Merge(cluster, profiles, defaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	escrow, err := clustersecrets.ParseRecipients(escrowRecipients)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: escrowRecipients: %w", err)
	}

	masterPlain, err := masterAgeKey.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: read master key: %w", err)
	}
	masterPub, err := clustersecrets.AgePublicKey(masterPlain)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: master key: %w", err)
	}

	tools := toolbox()

	prev, err := readExisting(ctx, tools, existing, masterAgeKey)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	clusterPub, clusterPriv := prev.pub, prev.priv
	if clusterPriv == "" {
		if clusterPub, clusterPriv, err = clustersecrets.GenerateAgeKey(); err != nil {
			return nil, fmt.Errorf("generate-cluster-secrets: generate cluster key: %w", err)
		}
	}

	refs := clustersecrets.CollectRefs(specs)
	sopsDocs, err := fetchSopsRefs(ctx, tools, clustersecrets.SopsFiles(refs), masterAgeKey, sopsRefDir)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	vaultDocs, err := fetchVault(ctx, tools, clustersecrets.VaultPaths(refs), vaultConn{
		addr: vaultAddr, service: vaultService, token: vaultToken,
		roleID: vaultRoleID, secretID: vaultSecretID, caCert: vaultCACert,
	})
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	resolved, err := clustersecrets.ResolveRefs(refs, sopsDocs, vaultDocs)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}

	previous := clustersecrets.Existing{}
	for id, f := range prev.secrets {
		previous[id] = f.secret.Data
	}
	secrets, err := clustersecrets.Build(specs, previous, resolved, clustersecrets.ParseRotation(rotate), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}

	clusterRecipients := uniq(append([]string{clusterPub}, escrow...))
	keyRecipients := uniq(append([]string{masterPub}, escrow...))

	out := dag.Directory()
	var jobs []encryptJob
	plan := func(s clustersecrets.Secret, p string, recipients []string, old *prevFile) error {
		if old != nil && clustersecrets.Unchanged(old.secret, old.encrypted, s, recipients) {
			out = out.WithNewFile(p, string(old.encrypted))
			return nil
		}
		doc, err := clustersecrets.RenderSecret(s)
		if err != nil {
			return fmt.Errorf("render %s: %w", s.ID(), err)
		}
		jobs = append(jobs, encryptJob{plaintext: string(doc), path: p, recipients: recipients})
		return nil
	}

	if err := plan(clustersecrets.NewAgeKeySecret(clusterPriv), clustersecrets.AgeKeyFile, keyRecipients, prev.key); err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
	}
	var resources []string
	for _, s := range secrets {
		p := clustersecrets.SecretPath(s.Namespace, s.Name)
		var old *prevFile
		if f, ok := prev.secrets[s.ID()]; ok {
			old = &f
		}
		if err := plan(s, p, clusterRecipients, old); err != nil {
			return nil, fmt.Errorf("generate-cluster-secrets: %w", err)
		}
		resources = append(resources, strings.TrimPrefix(p, clustersecrets.SecretsDir+"/"))
	}

	out = out.
		WithDirectory(".", encrypt(tools, jobs)).
		WithNewFile(".sops.yaml", string(clustersecrets.RenderSopsConfig(clusterRecipients, keyRecipients))).
		WithNewFile(clustersecrets.PublicKeyFile, clusterPub+"\n").
		WithNewFile(path.Join(clustersecrets.SecretsDir, "kustomization.yaml"), string(clustersecrets.RenderKustomization(resources)))

	// Resolve now, so a sops failure surfaces as this function's error
	// instead of at export time.
	if _, err := out.Sync(ctx); err != nil {
		return nil, fmt.Errorf("generate-cluster-secrets: encrypt: %w", err)
	}
	return out, nil
}

// ClusterAgeKey returns the cluster's private key from a
// GenerateClusterSecrets output, e.g. to hand to flux bootstrap as
// --sops-age-key. Fails if the key does not match the recorded age.pub.
//
// +cache="session"
func (m *Secrets) ClusterAgeKey(
	ctx context.Context,
	// Output of GenerateClusterSecrets
	existing *dagger.Directory,
	// AGE key the cluster key is encrypted for
	masterAgeKey *dagger.Secret,
) (*dagger.Secret, error) {
	prev, err := readExisting(ctx, toolbox(), existing, masterAgeKey)
	if err != nil {
		return nil, fmt.Errorf("cluster-age-key: %w", err)
	}
	if prev.priv == "" {
		return nil, fmt.Errorf("cluster-age-key: %s not found", clustersecrets.AgeKeyFile)
	}
	return dag.SetSecret("cluster-age-key-"+nonce(), prev.priv), nil
}

func toolbox() *dagger.Container {
	return dag.Container().
		From(toolboxImage).
		WithExec([]string{"apk", "add", "--no-cache", "sops", "curl", "jq"})
}

// nonce keeps SetSecret names unique and busts the exec cache where a
// cached result would be stale (Vault reads).
func nonce() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func loadProfiles(ctx context.Context, dir *dagger.Directory) (map[string]*clustersecrets.SecretProfile, error) {
	out := map[string]*clustersecrets.SecretProfile{}
	if dir == nil {
		return out, nil
	}
	seen := map[string]bool{}
	var paths []string
	for _, pattern := range []string{"*.yaml", "*.yml", "**/*.yaml", "**/*.yml"} {
		matches, err := dir.Glob(ctx, pattern)
		if err != nil {
			return nil, fmt.Errorf("list profiles: %w", err)
		}
		for _, p := range matches {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	sort.Strings(paths)

	from := map[string]string{}
	for _, p := range paths {
		content, err := dir.File(p).Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		if clustersecrets.PeekKind([]byte(content)) != clustersecrets.KindSecretProfile {
			continue
		}
		prof, err := clustersecrets.ParseSecretProfile([]byte(content))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if other, dup := from[prof.Metadata.Name]; dup {
			return nil, fmt.Errorf("profile %q defined in both %s and %s", prof.Metadata.Name, other, p)
		}
		from[prof.Metadata.Name] = p
		out[prof.Metadata.Name] = prof
	}
	return out, nil
}

type prevFile struct {
	secret    clustersecrets.Secret
	encrypted []byte
}

type previousRun struct {
	pub, priv string
	key       *prevFile
	secrets   map[string]prevFile // by namespace/name
}

// readExisting decrypts a previous run: the cluster key with the master
// key, then every secret with the cluster key.
func readExisting(ctx context.Context, tools *dagger.Container, existing *dagger.Directory, master *dagger.Secret) (*previousRun, error) {
	run := &previousRun{secrets: map[string]prevFile{}}
	if existing == nil {
		return run, nil
	}
	secretFiles, err := existing.Glob(ctx, clustersecrets.SecretsDir+"/**/*.enc.yaml")
	if err != nil {
		return nil, fmt.Errorf("list existing secrets: %w", err)
	}
	keyFiles, err := existing.Glob(ctx, clustersecrets.AgeKeyFile)
	if err != nil {
		return nil, fmt.Errorf("list existing key: %w", err)
	}
	if len(keyFiles) == 0 {
		if len(secretFiles) > 0 {
			// Generating a new key here would orphan every existing file.
			return nil, fmt.Errorf("existing output has secrets but no %s", clustersecrets.AgeKeyFile)
		}
		return run, nil
	}

	keyPlain, err := decryptFiles(ctx, tools, master, existing, []string{clustersecrets.AgeKeyFile})
	if err != nil {
		return nil, fmt.Errorf("decrypt cluster key with master key: %w", err)
	}
	keySecret, err := clustersecrets.ParseSecret(keyPlain[clustersecrets.AgeKeyFile])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", clustersecrets.AgeKeyFile, err)
	}
	run.priv = strings.TrimSpace(keySecret.Data[clustersecrets.AgeKeyDataKey])
	if run.pub, err = clustersecrets.AgePublicKey(run.priv); err != nil {
		return nil, fmt.Errorf("%s: %w", clustersecrets.AgeKeyFile, err)
	}
	if recorded, err := existing.File(clustersecrets.PublicKeyFile).Contents(ctx); err == nil {
		if strings.TrimSpace(recorded) != run.pub {
			return nil, fmt.Errorf("%s (%s) does not match the key in %s (%s)",
				clustersecrets.PublicKeyFile, strings.TrimSpace(recorded), clustersecrets.AgeKeyFile, run.pub)
		}
	}
	keyEnc, err := existing.File(clustersecrets.AgeKeyFile).Contents(ctx)
	if err != nil {
		return nil, err
	}
	run.key = &prevFile{secret: keySecret, encrypted: []byte(keyEnc)}

	if len(secretFiles) == 0 {
		return run, nil
	}
	clusterKey := dag.SetSecret("cluster-age-key-"+nonce(), run.priv)
	plain, err := decryptFiles(ctx, tools, clusterKey, existing, secretFiles)
	if err != nil {
		return nil, fmt.Errorf("decrypt existing secrets with cluster key: %w", err)
	}
	for _, p := range secretFiles {
		s, err := clustersecrets.ParseSecret(plain[p])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		enc, err := existing.File(p).Contents(ctx)
		if err != nil {
			return nil, err
		}
		run.secrets[s.ID()] = prevFile{secret: s, encrypted: []byte(enc)}
	}
	return run, nil
}

// decryptFiles decrypts paths (relative to src) with key. Output goes to
// files, never stdout, so values do not show up in progress logs.
func decryptFiles(ctx context.Context, tools *dagger.Container, key *dagger.Secret, src *dagger.Directory, paths []string) (map[string][]byte, error) {
	ctr := tools.
		WithMountedDirectory("/src", src).
		WithSecretVariable("SOPS_AGE_KEY", key)
	for _, p := range paths {
		ctr = ctr.
			WithExec([]string{"mkdir", "-p", path.Dir(path.Join("/out", p))}).
			WithExec(clustersecrets.SopsDecryptArgs(path.Join("/src", p), path.Join("/out", p)))
	}
	out := map[string][]byte{}
	for _, p := range paths {
		c, err := ctr.File(path.Join("/out", p)).Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out[p] = []byte(c)
	}
	return out, nil
}

func fetchSopsRefs(ctx context.Context, tools *dagger.Container, files []string, master *dagger.Secret, dir *dagger.Directory) (map[string][]byte, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if dir == nil {
		return nil, fmt.Errorf("profile references sops files (%s) but sopsRefDir is not set", strings.Join(files, ", "))
	}
	docs, err := decryptFiles(ctx, tools, master, dir, files)
	if err != nil {
		return nil, fmt.Errorf("decrypt ref+sops file with master key: %w", err)
	}
	return docs, nil
}

type vaultConn struct {
	addr                    string
	service                 *dagger.Service
	token, roleID, secretID *dagger.Secret
	caCert                  *dagger.File
}

func fetchVault(ctx context.Context, tools *dagger.Container, paths []string, v vaultConn) (map[string][]byte, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	addr := v.addr
	if addr == "" && v.service != nil {
		addr = "http://vault:8200"
	}
	if addr == "" {
		return nil, fmt.Errorf("profile references Vault (%s) but neither vaultAddr nor vaultService is set", strings.Join(paths, ", "))
	}
	if v.token == nil && (v.roleID == nil || v.secretID == nil) { // pragma: allowlist secret
		return nil, fmt.Errorf("profile references Vault but neither vaultToken nor vaultRoleID+vaultSecretID is set")
	}

	ctr := tools.
		WithNewFile("/vault/fetch.sh", clustersecrets.VaultFetchScript).
		WithNewFile("/vault/paths", strings.Join(paths, "\n")+"\n").
		WithEnvVariable("VAULT_ADDR", addr).
		// Vault content can change between runs with identical inputs.
		WithEnvVariable("CACHEBUST", nonce())
	if v.service != nil {
		ctr = ctr.WithServiceBinding("vault", v.service)
	}
	if v.caCert != nil {
		ctr = ctr.
			WithMountedFile("/vault/ca.crt", v.caCert).
			WithEnvVariable("VAULT_CACERT", "/vault/ca.crt")
	}
	if v.token != nil {
		ctr = ctr.WithSecretVariable("VAULT_TOKEN", v.token)
	} else {
		ctr = ctr.
			WithSecretVariable("VAULT_ROLE_ID", v.roleID).
			WithSecretVariable("VAULT_SECRET_ID", v.secretID)
	}

	body, err := ctr.
		WithExec([]string{"sh", "/vault/fetch.sh", "/vault/paths"}, dagger.ContainerWithExecOpts{
			RedirectStdout: "/vault/out.json",
		}).
		File("/vault/out.json").
		Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read from vault: %w", err)
	}
	return clustersecrets.ParseVaultBatch([]byte(body))
}

type encryptJob struct {
	plaintext  string
	path       string
	recipients []string
}

// encrypt runs sops for every job. Plaintext enters as a mounted secret, so
// it never lands in a cached layer; only the encrypted files do.
func encrypt(tools *dagger.Container, jobs []encryptJob) *dagger.Directory {
	ctr := tools.WithExec([]string{"mkdir", "-p", "/out"})
	for i, j := range jobs {
		in := fmt.Sprintf("/in/%d.yaml", i)
		out := path.Join("/out", j.path)
		ctr = ctr.
			WithMountedSecret(in, dag.SetSecret("cluster-secret-"+nonce(), j.plaintext)).
			WithExec([]string{"mkdir", "-p", path.Dir(out)}).
			WithExec(clustersecrets.SopsEncryptArgs(j.recipients, in, out))
	}
	return ctr.Directory("/out")
}

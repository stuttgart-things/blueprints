package main

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"dagger/flux/clusterapps"
	"dagger/flux/internal/dagger"

	"gopkg.in/yaml.v3"
)

// catalogProfiles is where the catalog keeps its AppProfiles.
const catalogProfiles = "*/platform/components/*/profile.yaml"

// RenderClusterApps renders the catalog apps a cluster runs, from one
// ClusterApps file and the catalog's AppProfiles.
//
// Output, spec.secrets.mode separate (default):
//
//	flux/apps.yaml    catalog GitRepository, the secrets Kustomization and one
//	                  Kustomization per bundle -> the cluster's own path
//	cluster-secrets/  generate-cluster-secrets output -> spec.secrets.path,
//	                  which must lie outside every path decrypted with another key
//
// Output, mode inline (the cluster's sops-age is the cluster key):
//
//	flux/apps.yaml                    -> the cluster's own path, as a whole
//	flux/cluster-secrets/             applied by the cluster's root Kustomization
//	flux/cluster-secrets/.sourceignore  keeps sops-age.enc.yaml and .sops.yaml out
//
// The AppProfiles come from the catalog itself, at the tag (or branch) in
// spec.source -- the same revision the cluster's bundles pull. --profile-dir
// replaces that, e.g. to try a profile before it is in the catalog. Each
// bundle Kustomization is rendered by claim-flux-kustomizations
// (templateName=bundle).
//
// Usage:
//
//	dagger call -m flux render-cluster-apps \
//	  --cluster-apps clusters/labda/vsphere/cicd-test4/apps.yaml \
//	  --master-age-key env:SOPS_AGE_KEY \
//	  --existing-secrets secrets/clusters/labda/vsphere/cicd-test4 \
//	  export --path out
//
// Cached per session only, like generate-cluster-secrets: within one call
// every use of the result sees the same keys and values.
//
// +cache="session"
func (m *Flux) RenderClusterApps(
	ctx context.Context,
	// ClusterApps document (kind: ClusterApps)
	clusterApps *dagger.File,
	// Directory searched for AppProfile documents instead of the catalog at spec.source
	// +optional
	profileDir *dagger.Directory,
	// Glob for the files to read in profileDir (default **/*.yaml; the catalog: */platform/components/*/profile.yaml)
	// +optional
	profileGlob string,
	// OCI KCL module rendering the bundle Kustomizations
	// +optional
	// +default="ghcr.io/stuttgart-things/claim-flux-kustomizations?tag=0.4.0"
	bundleModule string,
	// AGE key of the CI / key custodian; needed when an enabled app has secrets
	// +optional
	masterAgeKey *dagger.Secret,
	// Comma-separated break-glass AGE public keys
	// +optional
	escrowRecipients string,
	// cluster-secrets/ of the previous run (flux/cluster-secrets/ in mode inline); omit only on the very first run
	// +optional
	existingSecrets *dagger.Directory,
	// Base directory for ref+sops:// paths
	// +optional
	sopsRefDir *dagger.Directory,
	// Vault address
	// +optional
	vaultAddr string,
	// Vault token
	// +optional
	vaultToken *dagger.Secret,
	// Vault AppRole role ID
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
) (*dagger.Directory, error) {
	raw, err := clusterApps.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("read cluster apps: %w", err)
	}
	c, err := clusterapps.ParseClusterApps([]byte(raw))
	if err != nil {
		return nil, err
	}

	if profileDir == nil {
		src := c.Spec.Source
		profileDir = catalogCheckout(src.URL, src.Tag, src.Branch)
		if profileGlob == "" {
			profileGlob = catalogProfiles
		}
	}
	if profileGlob == "" {
		profileGlob = "**/*.yaml"
	}
	paths, err := profileDir.Glob(ctx, profileGlob)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	files := map[string][]byte{}
	for _, p := range paths {
		s, err := profileDir.File(p).Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		files[p] = []byte(s)
	}
	profiles, err := clusterapps.ParseProfiles(files)
	if err != nil {
		return nil, err
	}

	res, err := clusterapps.Build(c, profiles)
	if err != nil {
		return nil, err
	}
	manifests := string(res.Manifests)
	for _, b := range res.Bundles {
		ks, err := renderBundle(ctx, bundleModule, b)
		if err != nil {
			return nil, fmt.Errorf("render bundle %v: %w", b["name"], err)
		}
		manifests += "---\n" + strings.TrimPrefix(strings.TrimSpace(ks), "---\n") + "\n"
	}
	out := dag.Directory().WithNewFile("flux/apps.yaml", manifests)
	if res.ClusterSecrets == nil { // pragma: allowlist secret
		return out, nil
	}
	if masterAgeKey == nil {
		return nil, fmt.Errorf("enabled apps have secrets, so masterAgeKey is required")
	}

	secretProfiles := dag.Directory()
	for name, b := range res.SecretProfiles {
		secretProfiles = secretProfiles.WithNewFile(path.Join("profiles", name), string(b))
	}
	secrets := dag.Secrets().GenerateClusterSecrets(
		dag.Directory().WithNewFile("cluster-secrets.yaml", string(res.ClusterSecrets)).File("cluster-secrets.yaml"),
		masterAgeKey,
		dagger.SecretsGenerateClusterSecretsOpts{
			ProfileDir:       secretProfiles,
			EscrowRecipients: escrowRecipients,
			Existing:         existingSecrets,
			SopsRefDir:       sopsRefDir,
			VaultAddr:        vaultAddr,
			VaultToken:       vaultToken,
			VaultRoleID:      vaultRoleID,
			VaultSecretID:    vaultSecretID,
			VaultCacert:      vaultCACert,
			Rotate:           rotate,
		})
	if res.Inline {
		return out.WithDirectory("flux/cluster-secrets", secrets.WithNewFile(".sourceignore", clusterapps.SourceIgnore)), nil
	}
	return out.WithDirectory("cluster-secrets", secrets), nil
}

// renderBundle renders one bundle Kustomization with claim-flux-kustomizations.
// The parameters go as a file: components and substitute are a list and a
// map, which the comma-separated --parameters form cannot carry.
func renderBundle(ctx context.Context, module string, params clusterapps.BundleParams) (string, error) {
	b, err := yaml.Marshal(params)
	if err != nil {
		return "", err
	}
	file := dag.Directory().WithNewFile("parameters.yaml", string(b)).File("parameters.yaml")
	return dag.Kcl().Run(dagger.KclRunOpts{
		OciSource:      module,
		ParametersFile: file,
		Entrypoint:     "main.k",
	}).Contents(ctx)
}

// catalogCheckout clones the catalog at a tag or branch. A plain git clone
// rather than the engine's git API: in this module the git dependency shadows
// dag.Git, and dag.Address(...).GitRepository() fails in the engine with
// "assign ModuleObject to GitRepository".
func catalogCheckout(url, tag, branch string) *dagger.Directory {
	ref := tag
	ctr := dag.Container().From("alpine/git:latest")
	if ref == "" {
		// A branch moves; a tag does not, so only the branch busts the cache.
		ref = branch
		ctr = ctr.WithEnvVariable("CACHEBUST", time.Now().String())
	}
	return ctr.
		WithExec([]string{"git", "clone", "--depth", "1", "--branch", ref, url, "/catalog"}).
		Directory("/catalog")
}

package main

import (
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
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
// spec.wiring.generate: true adds flux/kustomization.yaml (bootstrap files,
// spec.wiring.extraResources, apps.yaml, cluster-secrets/secrets in mode
// inline) and flux/<layer dir>/kustomization.yaml (the layer's bundle files,
// spec.layers.<name>.extraResources) for each layer that has either.
//
// With spec.layers, apps.yaml also carries one Kustomization per layer, and a
// bundle placed in a layer goes to flux/<layer dir>/<bundle name>.yaml
// instead: export flux/ into spec.path.
//
// spec.source.kind OCIRepository: the catalog is the whole repo as one OCI
// artifact. The bundles read it with sourceRef.kind OCIRepository and a patch
// for their children; the profiles come from the same artifact.
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
	// Directory with more AppProfiles (**/*.yaml), added to the catalog's or profileDir's -- e.g. a cluster's own, before they move into the catalog. A name in both fails.
	// +optional
	extraProfileDir *dagger.Directory,
	// OCI KCL module rendering the bundle Kustomizations
	// +optional
	// +default="ghcr.io/stuttgart-things/claim-flux-kustomizations?tag=0.5.0"
	bundleModule string,
	// Local checkout of that KCL module, used instead of bundleModule -- to try a template change before it is published
	// +optional
	bundleSource *dagger.Directory,
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
		if src.IsOCI() {
			profileDir = catalogPull(src.URL, src.Tag)
		} else {
			profileDir = catalogCheckout(src.URL, src.Tag, src.Branch)
		}
		if profileGlob == "" {
			profileGlob = catalogProfiles
		}
	}
	if profileGlob == "" {
		profileGlob = "**/*.yaml"
	}
	files := map[string][]byte{}
	if err := readFiles(ctx, profileDir, profileGlob, "", files); err != nil {
		return nil, err
	}
	if extraProfileDir != nil {
		// prefixed, so a profile in both is a duplicate name, not a lost file
		if err := readFiles(ctx, extraProfileDir, "**/*.yaml", "extra:", files); err != nil {
			return nil, err
		}
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
	out := dag.Directory()
	for _, b := range res.Bundles {
		ks, err := renderBundle(ctx, bundleModule, bundleSource, b)
		if err != nil {
			return nil, fmt.Errorf("render bundle %v: %w", b["name"], err)
		}
		doc := clusterapps.AllowlistSecretKeywords("---\n" + strings.TrimPrefix(strings.TrimSpace(ks), "---\n") + "\n")
		name, _ := b["name"].(string)
		if dir, ok := res.BundleDirs[name]; ok {
			out = out.WithNewFile(path.Join("flux", dir, name+".yaml"), doc)
			continue
		}
		manifests += doc
	}
	out = out.WithNewFile("flux/apps.yaml", clusterapps.AllowlistSecretKeywords(manifests))
	if res.Wiring != nil {
		from, err := clusterApps.Name(ctx)
		if err != nil {
			return nil, fmt.Errorf("cluster apps file name: %w", err)
		}
		files := res.WiringFiles(c.Metadata.Name, path.Base(from))
		for _, p := range slices.Sorted(maps.Keys(files)) {
			out = out.WithNewFile(path.Join("flux", p), clusterapps.AllowlistSecretKeywords(string(files[p])))
		}
	}
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
func renderBundle(ctx context.Context, module string, source *dagger.Directory, params clusterapps.BundleParams) (string, error) {
	b, err := yaml.Marshal(params)
	if err != nil {
		return "", err
	}
	file := dag.Directory().WithNewFile("parameters.yaml", string(b)).File("parameters.yaml")
	opts := dagger.KclRunOpts{
		OciSource:      module,
		ParametersFile: file,
		Entrypoint:     "main.k",
	}
	if source != nil {
		opts.Source, opts.OciSource = source, ""
	}
	return dag.Kcl().Run(opts).Contents(ctx)
}

// readFiles adds every file of dir matching glob to files, keyed prefix+path.
func readFiles(ctx context.Context, dir *dagger.Directory, glob, prefix string, files map[string][]byte) error {
	paths, err := dir.Glob(ctx, glob)
	if err != nil {
		return fmt.Errorf("list profiles: %w", err)
	}
	for _, p := range paths {
		s, err := dir.File(p).Contents(ctx)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		files[prefix+p] = []byte(s)
	}
	return nil
}

// fluxCLIImage pulls the catalog artifact; the same flux as the clusters run.
const fluxCLIImage = "ghcr.io/fluxcd/flux-cli:v2.9.6"

// catalogPull extracts the catalog's OCI artifact (the whole repo) at a tag.
// The artifact is public; a tag is immutable, so it caches like a git tag.
func catalogPull(url, tag string) *dagger.Directory {
	return dag.Container().From(fluxCLIImage).
		// flux pull wants the output directory to exist and writable; the
		// image runs as an unprivileged user
		WithUser("root").
		WithDirectory("/catalog", dag.Directory()).
		WithExec([]string{"flux", "pull", "artifact", url + ":" + tag, "--output", "/catalog"}).
		Directory("/catalog")
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

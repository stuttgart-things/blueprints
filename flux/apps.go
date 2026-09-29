package main

import (
	"context"
	"fmt"
	"path"

	"dagger/flux/clusterapps"
	"dagger/flux/internal/dagger"
)

// RenderClusterApps renders the catalog apps a cluster runs, from one
// ClusterApps file and the catalog's AppProfiles.
//
// Output:
//
//	flux/apps.yaml    catalog GitRepository, the secrets Kustomization and one
//	                  Kustomization per bundle -> the cluster's own path
//	cluster-secrets/  generate-cluster-secrets output -> spec.secrets.path,
//	                  which must lie outside every path decrypted with another key
//
// Usage:
//
//	dagger call -m flux render-cluster-apps \
//	  --cluster-apps clusters/labda/vsphere/cicd-test4/apps.yaml \
//	  --profile-dir <catalog checkout> \
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
	// Directory searched for AppProfile documents, e.g. the catalog
	profileDir *dagger.Directory,
	// Glob for the files to read in profileDir
	// +optional
	// +default="**/*.yaml"
	profileGlob string,
	// AGE key of the CI / key custodian; needed when an enabled app has secrets
	// +optional
	masterAgeKey *dagger.Secret,
	// Comma-separated break-glass AGE public keys
	// +optional
	escrowRecipients string,
	// cluster-secrets/ of the previous run; omit only on the very first run
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
	out := dag.Directory().WithNewFile("flux/apps.yaml", string(res.Manifests))
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
	return out.WithDirectory("cluster-secrets", secrets), nil
}

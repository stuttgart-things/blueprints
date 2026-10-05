# FLUX

Bootstrap, render, apply, verify, and destroy workflows for Flux CD on
Kubernetes. Extracted from `kubernetes-deployment` (#143). The redundant
`flux-` prefix on every function was dropped in favour of the dominant
verb-first convention used elsewhere in the repo. AGE-key validation and
SOPS encryption now live in the [`secrets`](../secrets/README.md) module.

```bash
# BOOTSTRAP - FULL LIFECYCLE (validate keys, render, deploy operator, apply config, apply secrets, verify, wait)
dagger call -m flux bootstrap \
  --kube-config file:///home/sthings/.kube/vre2.yaml \
  --deploy-operator=true \
  --commit-to-git=true \
  --repository stuttgart-things/stuttgart-things \
  --destination-path "clusters/labul/vsphere/vre2" \
  --git-username env:GITHUB_USER \
  --git-password env:GITHUB_TOKEN \
  --git-token env:GITHUB_TOKEN \
  --sops-age-key env:SOPS_AGE_KEY \
  --age-public-key env:AGE_PUB \
  --render-secrets=true \
  --apply-secrets=true \
  --apply-config=true \
  --encrypt-secrets=true \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@cicd/flux-operator.yaml.gotmpl" \
  --operator-version "0.61.0" \
  --wait-for-reconciliation=true \
  --progress plain
```

```bash
# BOOTSTRAP - RENDER + ENCRYPT + COMMIT TO GIT (no cluster deploy)
dagger call -m flux bootstrap \
  --kube-config file:///home/sthings/.kube/cluster \
  --repository "my-org/fleet" \
  --destination-path "clusters/staging/" \
  --render-secrets \
  --git-username env:GIT_USERNAME \
  --git-password env:GIT_PASSWORD \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypt-secrets \
  --age-public-key env:AGE_PUBLIC_KEY \
  --commit-to-git \
  --git-token env:GITHUB_TOKEN \
  --deploy-operator=false \
  --wait-for-reconciliation=false \
  --progress plain
```

```bash
# BOOTSTRAP - DEPLOY OPERATOR ONLY (skip rendering and git)
dagger call -m flux bootstrap \
  --kube-config file:///home/sthings/.kube/cluster \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@cicd/flux-operator.yaml.gotmpl" \
  --operator-version "0.61.0" \
  --apply-secrets=false \
  --commit-to-git=false \
  --wait-for-reconciliation=false \
  --progress plain
```

```bash
# ONLY CREATE SECRETS ON CLUSTER
dagger call -m flux bootstrap \
  --kube-config file:///home/sthings/.kube/vre2.yaml \
  --deploy-operator=false \
  --commit-to-git=false \
  --repository stuttgart-things/stuttgart-things \
  --destination-path "clusters/labul/vsphere/vre2" \
  --git-username env:GITHUB_USER \
  --git-password env:GITHUB_TOKEN \
  --git-token env:GITHUB_TOKEN \
  --sops-age-key env:SOPS_AGE_KEY \
  --age-public-key env:AGE_PUB \
  --render-secrets=true \
  --apply-secrets=true \
  --apply-config=false \
  --encrypt-secrets=false \
  --wait-for-reconciliation=false \
  --progress plain
```

```bash
# DESTROY - FULL TEARDOWN (delete FluxInstance, secrets, operator, namespace)
dagger call -m flux destroy \
  --kube-config file:///home/sthings/.kube/cluster \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@cicd/flux-operator.yaml.gotmpl" \
  --progress plain
```

```bash
# INDIVIDUAL PHASE FUNCTIONS (each callable standalone via dagger call)

# Render config only
dagger call -m flux render-config \
  --config-parameters "name=flux-system,namespace=flux-system,version=2.8" \
  --progress plain

# Apply config to cluster
dagger call -m flux apply-config \
  --config-content "$(cat config.yaml)" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Apply secrets to cluster
dagger call -m flux apply-secrets \
  --secret-content "$(cat secrets.yaml)" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Verify secrets exist in cluster
dagger call -m flux verify-secrets \
  --secret-content "$(cat secrets.yaml)" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Deploy operator only
dagger call -m flux deploy-operator \
  --kube-config file:///home/sthings/.kube/cluster \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@cicd/flux-operator.yaml.gotmpl" \
  --state-values "version=0.61.0" \
  --progress plain

# Wait for reconciliation
dagger call -m flux wait-for-reconciliation \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Commit rendered config to git
dagger call -m flux commit-config \
  --config-content "$(cat config.yaml)" \
  --repository my-org/fleet \
  --destination-path clusters/staging/ \
  --git-token env:GITHUB_TOKEN \
  --progress plain
```

## Catalog apps with their secrets (render-cluster-apps)

One `AppProfile` per catalog app describes both halves: the bundle component
plus the vars it reads, and the Secret its `postBuild.substituteFrom` needs.
A `ClusterApps` file per cluster enables apps and sets values. Examples:
`examples/apps/`. Design and plan: [#206](https://github.com/stuttgart-things/blueprints/issues/206).

- **Profiles:** they live in the catalog (`<bundle>/components/<app>/profile.yaml`,
  from stuttgart-things/flux v1.100.0 on) and are read at the tag or branch in
  `spec.source`, the same revision the cluster's bundles pull.
  `--profile-dir` replaces that, e.g. to try a profile before it is in the catalog.
- **Bundle Kustomizations:** rendered by `claim-flux-kustomizations`
  (`templateName: bundle`, `--bundle-module`, default 0.5.0). Its parameters go
  as a file, so `components` and `substitute` stay a list and a map.

```bash
dagger call -m flux render-cluster-apps \
  --cluster-apps examples/apps/cicd-test4.yaml \
  --master-age-key env:SOPS_AGE_KEY \
  --existing-secrets <repo>/secrets/clusters/labda/vsphere/cicd-test4 \
  export --path out
```

- `out/flux/apps.yaml` goes into the cluster's path. It contains the catalog
  GitRepository, the `cluster-secrets` Kustomization and one Kustomization per
  bundle; each bundle `dependsOn` the secrets.
- `out/cluster-secrets/` goes to `spec.secrets.path`. That path must lie
  **outside** every path another Kustomization decrypts with a different key.
  Otherwise that Kustomization fails on files it cannot decrypt.
- Omit `--existing-secrets` only on the first run, otherwise every generated
  value and the cluster key are replaced.
- Lines in the generated Kustomizations whose key contains a detect-secrets
  keyword (`INFRA_TLS_SECRET: wildcard-tls`) get `# pragma: allowlist secret`.
  They only ever hold names (values are in `cluster-secrets/`), and a pragma
  added by hand would be lost on the next render.
- The run fails early on an unknown app, a missing required var, a var the
  profile does not declare, or a secret key the profile does not declare.
- The cluster key goes into the Secret named by `spec.secrets.decryptionSecret`:
  `dagger call -m secrets cluster-age-key --existing out/cluster-secrets --master-age-key env:SOPS_AGE_KEY plaintext`.

### Catalog from an OCI artifact, bundles in layers

`examples/apps/oci-layers.yaml`; the full case is `clusters/edge/cluster-apps.yaml`
in stuttgart-things/harvester ([harvester#364](https://github.com/stuttgart-things/harvester/issues/364)).

- **`spec.source.kind: OCIRepository`** (default `GitRepository`): the whole
  catalog repo as one OCI artifact, e.g. `oci://ghcr.io/stuttgart-things/flux/repo`,
  with a `tag`. The output has an `OCIRepository` instead of the GitRepository.
  Each bundle reads it with `sourceRef.kind: OCIRepository` and patches its
  children to that kind (they hard-code `GitRepository`; claim-flux-kustomizations
  0.5.0 `patches`). The profiles are read from the same artifact
  (`flux pull artifact`). A per-directory artifact does not work: the children
  use paths from the repo root.
- **`spec.layers`** + **`spec.path`**: one Kustomization per layer, applying
  `<spec.path>/<dir>` from the cluster's own repo (`flux-system`), with
  `dependsOn`, `wait` (default `true`), `timeout` (default `5m`),
  `decryptionSecret`, `substituteFrom` and `labels`. They go into `flux/apps.yaml`.
- **`spec.bundles.<bundle>.layer`**: the bundle Kustomization goes to
  `flux/<dir>/<bundle name>.yaml` instead of `flux/apps.yaml`. It gets the
  layer's substitution, so a var can be `${CLUSTER_DOMAIN}` from a ConfigMap
  the layer reads. A layer without bundles applies only what the cluster
  keeps there by hand.
- **`spec.bundles.<bundle>.timeout`**: `spec.timeout` of the bundle
  Kustomization (default `15m`), e.g. `10m` for a small infra bundle. The
  bundle waits for every selected app, so this is how long they all may take.
- **`--extra-profile-dir`**: AppProfiles added to the catalog's, e.g. for
  catalog components that have no `profile.yaml` yet. A name in both fails.
- **`--bundle-source`**: a local checkout of claim-flux-kustomizations instead
  of `--bundle-module`, to try a template change before it is published.

Two layouts, set with `spec.secrets.mode`:

| Mode | Where the secrets go | Who applies them | For |
|---|---|---|---|
| `separate` (default, `examples/apps/cicd-test4.yaml`) | `spec.secrets.path`, outside the cluster's path | a `cluster-secrets` Kustomization with its own decryption Secret | clusters that already have a `sops-age` key of their own |
| `inline` (`examples/apps/inline.yaml`) | `flux/cluster-secrets/`, next to `apps.yaml`: export `flux/` into the cluster's path | the cluster's root Kustomization, with `sops-age` | new clusters: bootstrap with the cluster key as `sops-age` |

In `inline` mode, `cluster-secrets/.sourceignore` keeps two files out of the
root Kustomization: `sops-age.enc.yaml` (master and escrow only) and
`.sops.yaml` (not a Kubernetes object). Pass `--existing-secrets
<cluster path>/cluster-secrets` on later runs.

## Moved out of this module

| Old call | New call |
|---|---|
| `dagger call -m flux validate-age-key-pair` | `dagger call -m secrets validate-age-key-pair` |
| `dagger call -m flux flux-encrypt-secrets --secret-content $X` | `dagger call -m secrets encrypt-string --plaintext $X` |

# ARGOCD

Bootstrap, render, apply, and commit workflows for Argo CD on Kubernetes.
KCL-based cluster registration (clusterbook) lives here; SOPS / AGE
helpers live in the [`secrets`](../secrets/README.md) module.

## Functions

| Function | Purpose |
|---|---|
| `render-clusterbook-cluster-config` | Render the [`clusterbook-cluster-gen`](https://github.com/stuttgart-things/clusterbook-cluster-gen) KCL module. Returns the rendered manifests as a Dagger `File`. |
| `render-kubeconfig-secret` | Wrap a SOPS-encrypted source file (e.g. a cluster kubeconfig) in a `v1/Secret` manifest under `data.<key>`; optionally re-encrypts the manifest with SOPS for safe git commit. Returns the manifest as a Dagger `File`. |
| `detect-network-key` | Run `kubectl get nodes -o json` against the target cluster and return the dominant /24 prefix of the nodes' `InternalIP` addresses (e.g. `10.31.102`) — the format expected by `--network-key`. |
| `apply-config` | Apply a rendered config file to a cluster (creates the target namespace first). |
| `commit-config` | Commit a rendered file to a Git repo at `<destinationPath>/<fileName>` on a branch; optionally open a PR against a base branch and optionally merge it. |
| `create-vault-issuer` | Bootstrap a cert-manager `vault-pki` ClusterIssuer on a target cluster: live-fetches the Vault CA, templates a `vault-base-setup` Terraform invocation inline, and applies it. State persists as a k8s Secret on the target cluster — no artefact in the caller's filesystem. Closes #162. |
| `bootstrap-clusterbook-cluster` | Orchestrator: render → optional `--deploy` → optional `--commit-to-git`. Returns the rendered file. |

The functions are designed to compose: `render-clusterbook-cluster-config`
returns a `*File`, which `apply-config` and `commit-config` consume directly
as `--config-file`. No bash-variable round-tripping; the file stays inside
the Dagger engine and remains content-addressable / cacheable.

## Render

```bash
# RENDER — minimal, export rendered YAML to disk
dagger call -m argocd render-clusterbook-cluster-config \
  --name=philly --network-key=10.31.101 \
  export --path=/tmp/argocd/philly.yaml
```

```bash
# RENDER — with custom labels and a pinned OCI tag
dagger call -m argocd render-clusterbook-cluster-config \
  --name=philly \
  --network-key=10.31.101 \
  --cluster-labels='{"env":"lab","role":"mgmt","auto-project":"true"}' \
  --oci-source=ghcr.io/stuttgart-things/clusterbook-cluster-gen:0.1.0 \
  export --path=/tmp/argocd/philly.yaml
```

```bash
# RENDER — all KCL params come from a YAML/JSON values file
cat > /tmp/philly-values.yaml <<'EOF'
name: philly
networkKey: 10.31.101
clusterName: philly
createDNS: true
preserveKubeconfigServer: true
releaseOnDelete: true
kubeconfigSecretName: philly
kubeconfigSecretNamespace: argocd
argocdNamespace: argocd
providerConfigName: default
clusterLabels:
  env: lab
  role: mgmt
  auto-project: "true"
EOF

dagger call -m argocd render-clusterbook-cluster-config \
  --values-file=/tmp/philly-values.yaml \
  export --path=/tmp/argocd/philly.yaml
```

```bash
# RENDER — values file plus CLI override (flags override file values)
dagger call -m argocd render-clusterbook-cluster-config \
  --values-file=/tmp/philly-values.yaml \
  --argocd-namespace=argocd-prod \
  export --path=/tmp/argocd/philly.yaml
```

## Render a kubeconfig Secret from a SOPS-encrypted file

`render-kubeconfig-secret` decrypts a SOPS-encrypted source file (e.g. a
cluster kubeconfig under `stuttgart-things/secrets/kubeconfigs/`) and wraps
the plaintext into a `v1/Secret` manifest under `data.<dataKey>`. Equivalent
to:

```bash
sops --decrypt kind-dev-test1.yaml > kubeconfig.yaml
kubectl create secret generic kind-dev-test1 -n argocd \
  --from-file=kubeconfig=kubeconfig.yaml \
  --dry-run=client -o yaml
```

By default the rendered manifest is re-encrypted with SOPS using
`--age-public-key` so it can be safely committed to git via `commit-config`.
Pass `--encrypt=false` to get the plaintext manifest for direct
`kubectl apply` via `apply-config`.

```bash
# RENDER — encrypted Secret manifest, ready to commit to git
dagger call -m argocd render-kubeconfig-secret \
  --source-file=/home/sthings/projects/stuttgart-things/secrets/kubeconfigs/kind-dev-test1.yaml \
  --sops-key=env:SOPS_AGE_KEY \
  --age-public-key=env:AGE_PUBLIC_KEY \
  --name=kind-dev-test1 \
  --namespace=argocd \
  export --path=/tmp/argocd/kind-dev-test1.secret.enc.yaml
```

```bash
# RENDER — plaintext manifest for direct kubectl apply (no encryption)
dagger call -m argocd render-kubeconfig-secret \
  --source-file=/home/sthings/projects/stuttgart-things/secrets/kubeconfigs/kind-dev-test1.yaml \
  --sops-key=env:SOPS_AGE_KEY \
  --name=target-cluster-kubeconfig \
  --namespace=argocd \
  --data-key=kubeconfig \
  --encrypt=false \
  export --path=/tmp/argocd/kind-dev-test1.secret.yaml
```

```bash
# RENDER + COMMIT — encrypted Secret straight to git via commit-config
dagger call -m argocd render-kubeconfig-secret \
  --source-file=/home/sthings/projects/stuttgart-things/secrets/kubeconfigs/kind-dev-test1.yaml \
  --sops-key=env:SOPS_AGE_KEY \
  --age-public-key=env:AGE_PUBLIC_KEY \
  --name=kind-dev-test1 \
  export --path=/tmp/argocd/kind-dev-test1.secret.enc.yaml

dagger call -m argocd commit-config \
  --config-file=/tmp/argocd/kind-dev-test1.secret.enc.yaml \
  --repository stuttgart-things/fleet \
  --branch-name argocd/kubeconfig-kind-dev-test1 \
  --destination-path argocd/kubeconfigs/ \
  --file-name kind-dev-test1.yaml \
  --commit-message "Add kubeconfig Secret: kind-dev-test1" \
  --git-token env:GITHUB_TOKEN \
  --create-pr=true --base-branch main \
  --progress plain
```

### Parameters

| Flag | Required | Default | Notes |
|---|---|---|---|
| `--source-file` | yes | — | SOPS-encrypted source (e.g. a kubeconfig YAML). |
| `--sops-key` | yes | — | AGE private key (`AGE-SECRET-KEY-…`) used to decrypt `--source-file`. |
| `--name` | yes | — | Name of the rendered `v1/Secret`. |
| `--namespace` | no | `argocd` | Namespace of the rendered Secret. |
| `--data-key` | no | `kubeconfig` | Key under `data:` (i.e. `data.<dataKey>` holds the base64 payload). |
| `--encrypt` | no | `true` | Re-encrypt the manifest with SOPS. Set `false` for direct `kubectl apply`. |
| `--age-public-key` | yes¹ | — | AGE public key (`age1…`) for re-encryption. |
| `--sops-config` | no | _(none)_ | `.sops.yaml` to drive recipient/regex selection during re-encryption. |

¹ Only required when `--encrypt=true` (the default).

## Detect the cluster network key

`detect-network-key` queries the target cluster and returns the dominant
`/24` prefix of the nodes' `InternalIP` addresses. Useful for piping
straight into `render-clusterbook-cluster-config --network-key`.

```bash
# Detect the network key (e.g. "10.31.102")
dagger call -m argocd detect-network-key \
  --kube-config=env:KUBECONFIG
```

```bash
# Detect and feed into render in one shell pipeline
NETWORK_KEY=$(dagger call -m argocd detect-network-key --kube-config=env:KUBECONFIG)
dagger call -m argocd render-clusterbook-cluster-config \
  --name=platform-sthings \
  --network-key="${NETWORK_KEY}" \
  export --path=/tmp/argocd/platform-sthings.yaml
```

## Apply rendered manifests to a cluster

```bash
# APPLY — file-typed input (no shell round-trip)
dagger call -m argocd apply-config \
  --config-file=/tmp/argocd/philly.yaml \
  --kube-config env:KUBECONFIG \
  --progress plain
```

```bash
# APPLY — into a custom namespace
dagger call -m argocd apply-config \
  --config-file=/tmp/argocd/philly.yaml \
  --kube-config env:KUBECONFIG \
  --namespace argocd-prod \
  --progress plain
```

## Commit rendered manifests to Git

```bash
# COMMIT — single file straight onto a branch (no PR)
dagger call -m argocd commit-config \
  --config-file=/tmp/argocd/philly.yaml \
  --repository stuttgart-things/fleet \
  --branch-name argocd/cluster-philly \
  --destination-path argocd/clusters/ \
  --file-name philly.yaml \
  --commit-message "Register Argo CD cluster: philly" \
  --git-token env:GITHUB_TOKEN \
  --progress plain
```

```bash
# COMMIT + PR — open a pull request against main
dagger call -m argocd commit-config \
  --config-file=/tmp/argocd/philly.yaml \
  --repository stuttgart-things/fleet \
  --branch-name argocd/cluster-philly \
  --destination-path argocd/clusters/ \
  --file-name philly.yaml \
  --git-token env:GITHUB_TOKEN \
  --create-pr=true \
  --base-branch main \
  --pr-title "Register Argo CD cluster: philly" \
  --pr-body "Adds clusterbook-rendered registration manifests for the philly cluster." \
  --progress plain
```

```bash
# COMMIT + PR + AUTO-MERGE — squash and delete branch
dagger call -m argocd commit-config \
  --config-file=/tmp/argocd/philly.yaml \
  --repository stuttgart-things/fleet \
  --branch-name argocd/cluster-philly \
  --destination-path argocd/clusters/ \
  --file-name philly.yaml \
  --git-token env:GITHUB_TOKEN \
  --create-pr=true \
  --merge-pr=true \
  --merge-method squash \
  --progress plain
```

## Create a Vault-backed cert-manager ClusterIssuer

Bootstraps a `vault-pki` ClusterIssuer on a target cluster using the
[`vault-base-setup`](https://github.com/stuttgart-things/vault-base-setup)
Terraform module (`pki_enabled=false`, `certmanager_vault_issuer_enabled=true`).
The Terraform source is **inlined** in this module and rendered via
`dagger/templating` at runtime — there's nothing to check out, vendor, or
commit.

### What it creates on the target cluster

- `ClusterIssuer/vault-pki` (cert-manager)
- `Secret/vault-pki-ca` in `cert-manager` namespace (the Vault PKI root CA,
  fetched live from `${vault_addr}/v1/pki/ca/pem` at apply time)
- A Vault token secret + RBAC pieces wired up by `vault-base-setup`
- Terraform state as `Secret/tfstate-default-vault-<cluster-name>` in
  `kube-system`

### Prerequisites

- The TF creates a `ClusterIssuer` (cert-manager CRD) and a Secret in the
  `cert-manager` namespace. By default `--install-cert-manager-crds=true`
  pre-applies the upstream cert-manager CRDs and ensures the namespace
  exists — both server-side, idempotent, safe to leave on even when the
  `cert-manager-install` AppSet is also running. We pre-install **just
  the CRDs**, not the controller / webhook config — that way the
  validating webhook isn't yet registered when the TF applies the
  ClusterIssuer, sidestepping the admission-webhook race that breaks
  bare `kubectl wait`-style approaches. The full cert-manager install
  (controller, webhook, validating webhook config) lands later via the
  AppSet; the existing ClusterIssuer becomes Ready as soon as the
  controller picks it up. Use `--cert-manager-version` (default
  `v1.19.2`) to match the chart version installed by the AppSet — keeps
  CRD schema in lockstep.
- A Vault PKI mount + role + policy must already exist on the target
  Vault (typically set up out-of-band per lab via `infra-sthings/vault-ca`
  Terraform). `--pki-role` (default `sthings-vsphere`) and
  `--policy-name` (default `pki-issue`) must match that out-of-band setup.

### SOPS env file

The `--vault-env-file` is a SOPS-encrypted YAML keyed as:

```yaml
vault_addr: https://vault.infra.sthings-vsphere.labul.sva.de
vault_token: hvs.xxxx
vault_skip_verify: true   # optional, defaults to true
```

### Usage

```bash
# CREATE — minimal call against an existing cluster
dagger call -m argocd create-vault-issuer \
  --cluster-name homerun2-dev \
  --kubeconfig-source-file secrets/kubeconfigs/homerun2-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --progress plain
```

```bash
# CREATE — override PKI role/policy (e.g. for a different lab)
dagger call -m argocd create-vault-issuer \
  --cluster-name philly \
  --kubeconfig-source-file secrets/kubeconfigs/philly.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --pki-role sthings-vsphere \
  --policy-name pki-issue \
  --progress plain
```

```bash
# CREATE — skip the cert-manager CRD pre-install (only if you're sure
# the CRDs and namespace already exist on the target cluster)
dagger call -m argocd create-vault-issuer \
  --cluster-name homerun2-dev \
  --kubeconfig-source-file secrets/kubeconfigs/homerun2-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --install-cert-manager-crds=false \
  --progress plain
```

### Verify

```bash
kubectl get clusterissuer vault-pki
kubectl -n cert-manager get secret vault-pki-ca
```

Both should appear within seconds of the Terraform run completing. The
function is idempotent — re-running against the same cluster is a Terraform
no-op.

## Bootstrap (orchestrator)

`bootstrap-clusterbook-cluster` runs the full pipeline in a single Dagger
session: optional `detect-network-key` → `render-clusterbook-cluster-config`
→ optional `render-kubeconfig-secret` → `apply-config` (when `--deploy`) →
`commit-config` (when `--commit-to-git`). The rendered cluster-config file
is returned so you can also `export` it locally on the same call.

Two boolean gates wire in the helpers:

| Flag | Effect |
|---|---|
| `--detect-network-key` | When `--network-key` is empty, populate it via `kubectl get nodes -o json` against `--kube-config`. |
| `--render-kubeconfig-secret` | Render a `v1/Secret` from `--kubeconfig-source-file`. On `--deploy=true` the plaintext Secret is applied alongside the cluster config; on `--commit-to-git=true` the SOPS-encrypted Secret is committed alongside it as `<destination-path>/<kubeconfig-file-name>`. |

```bash
# BOOTSTRAP — render only, export rendered YAML
dagger call -m argocd bootstrap-clusterbook-cluster \
  --name=philly --network-key=10.31.101 \
  export --path=/tmp/argocd/philly.yaml
```

```bash
# BOOTSTRAP — render + deploy to cluster
dagger call -m argocd bootstrap-clusterbook-cluster \
  --name=philly --network-key=10.31.101 \
  --deploy=true \
  --kube-config env:KUBECONFIG \
  --progress plain
```

```bash
# BOOTSTRAP — render + commit to git with PR
dagger call -m argocd bootstrap-clusterbook-cluster \
  --name=philly --network-key=10.31.101 \
  --commit-to-git=true \
  --repository stuttgart-things/fleet \
  --git-token env:GITHUB_TOKEN \
  --branch-name argocd/cluster-philly \
  --destination-path argocd/clusters/ \
  --file-name philly.yaml \
  --create-pr=true \
  --base-branch main \
  --progress plain
```

```bash
# BOOTSTRAP — full lifecycle: render + deploy + commit + PR + auto-merge
dagger call -m argocd bootstrap-clusterbook-cluster \
  --values-file=/tmp/philly-values.yaml \
  --deploy=true --kube-config env:KUBECONFIG \
  --commit-to-git=true \
  --repository stuttgart-things/fleet \
  --git-token env:GITHUB_TOKEN \
  --branch-name argocd/cluster-philly \
  --destination-path argocd/clusters/ \
  --file-name philly.yaml \
  --create-pr=true --merge-pr=true --merge-method squash \
  --progress plain
```

```bash
# BOOTSTRAP — auto-detect network-key + render & commit kubeconfig Secret
dagger call -m argocd bootstrap-clusterbook-cluster \
  --name=kind-dev-test1 \
  --kube-config=env:KUBECONFIG \
  --detect-network-key=true \
  --render-kubeconfig-secret=true \
  --kubeconfig-source-file=/home/sthings/projects/stuttgart-things/secrets/kubeconfigs/kind-dev-test1.yaml \
  --sops-key=env:SOPS_AGE_KEY \
  --age-public-key=env:AGE_PUBLIC_KEY \
  --commit-to-git=true \
  --repository=stuttgart-things/fleet \
  --git-token=env:GITHUB_TOKEN \
  --branch-name=argocd/cluster-kind-dev-test1 \
  --destination-path=argocd/clusters/ \
  --file-name=kind-dev-test1.yaml \
  --kubeconfig-file-name=kind-dev-test1.kubeconfig.yaml \
  --create-pr=true --base-branch=main \
  --progress plain
```

## Render parameters

When `--values-file` is **not** set, the listed defaults are applied and
`--name`/`--network-key` are required. When `--values-file` **is** set, all
defaults are taken from the file and any CLI flag below acts as an override
for that key.

| Flag | Required | Default | Notes |
|---|---|---|---|
| `--name` | yes¹ | — | Cluster name (KCL `-D name`). |
| `--network-key` | yes¹ | — | /24 network key, e.g. `10.31.101`. |
| `--values-file` | no | _(none)_ | YAML/JSON file passed as KCL `--parametersFile`. CLI flags override matching keys. |
| `--oci-source` | no | `ghcr.io/stuttgart-things/clusterbook-cluster-gen:0.1.0` | OCI ref of the KCL module. |
| `--cluster-name` | no | falls back to `--name` | Argo CD-side cluster name. |
| `--create-dns` | no | `true` | Create a DNS record for the cluster. |
| `--preserve-kubeconfig-server` | no | `true` | Keep existing `server` field from the kubeconfig Secret. |
| `--release-on-delete` | no | `true` | Release the Argo CD cluster Secret on resource delete. |
| `--kubeconfig-secret-name` | no | falls back to `--name` | Secret holding the cluster kubeconfig. |
| `--kubeconfig-secret-namespace` | no | `argocd` | Namespace of the kubeconfig Secret. |
| `--argocd-namespace` | no | `argocd` | Argo CD installation namespace. |
| `--provider-config-name` | no | `default` | Crossplane ProviderConfig name. |
| `--cluster-labels` | no | _(empty)_ | JSON object literal, e.g. `{"env":"lab","role":"mgmt"}`. |
| `--entrypoint` | no | `main.k` | KCL entrypoint file. |

¹ Only required when `--values-file` is not provided.

## Equivalent `kcl` CLI

The render function wraps the following invocation

```bash
kcl run oci://ghcr.io/stuttgart-things/clusterbook-cluster-gen --tag 0.1.0 \
  -D name=philly \
  -D networkKey=10.31.101 \
  -D clusterName=philly \
  -D createDNS=true \
  -D preserveKubeconfigServer=true \
  -D releaseOnDelete=true \
  -D kubeconfigSecretName=philly \
  -D kubeconfigSecretNamespace=argocd \
  -D argocdNamespace=argocd \
  -D providerConfigName=default \
  -D 'clusterLabels={"env":"lab","role":"mgmt","auto-project":"true"}'
```

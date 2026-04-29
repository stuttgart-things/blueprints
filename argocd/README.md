# ARGOCD

Bootstrap, render, apply, and commit workflows for Argo CD on Kubernetes.
KCL-based cluster registration (clusterbook) lives here; SOPS / AGE
helpers live in the [`secrets`](../secrets/README.md) module.

## Functions

| Function | Purpose |
|---|---|
| `render-clusterbook-cluster-config` | Render the [`clusterbook-cluster-gen`](https://github.com/stuttgart-things/clusterbook-cluster-gen) KCL module. Returns the rendered manifests as a Dagger `File`. |
| `apply-config` | Apply a rendered config file to a cluster (creates the target namespace first). |
| `commit-config` | Commit a rendered file to a Git repo at `<destinationPath>/<fileName>` on a branch; optionally open a PR against a base branch and optionally merge it. |
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

## Bootstrap (orchestrator)

`bootstrap-clusterbook-cluster` runs the full pipeline in a single Dagger
session: render → `apply-config` (when `--deploy`) → `commit-config` (when
`--commit-to-git`). The rendered file is returned so you can also `export`
it locally on the same call.

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

The render function wraps the following invocation:

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

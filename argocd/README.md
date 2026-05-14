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
| `create-vault-issuer` | Prepare the cluster-side prerequisites cert-manager needs to authenticate against a remote Vault PKI: applies the policy + mints a token in Vault directly (HTTP API), reads the CA, then `kubectl apply`s a 3-document YAML (Namespace + 2 Secrets) directly to the target cluster using the supplied kubeconfig. Does NOT create the `ClusterIssuer` itself — that's the `cert-manager-vault-pki` AppSet's job. Closes #162. |
| `create-vault-k8s-auth` | Provision a Vault Kubernetes auth backend for an in-cluster ServiceAccount (typically ESO): `kubectl apply`s a 4-document YAML (Namespace + ServiceAccount + non-expiring SA-token Secret + ClusterRoleBinding→`system:auth-delegator`) to the target cluster, then drives Vault HTTP directly to mount `auth/<cluster-name>-<auth-name>`, write its config (`kubernetes_host` + reviewer JWT + CA + `disable_iss_validation=true` + `disable_local_ca_jwt=true`), and upsert a role binding the SA to one or more pre-existing policies. Replaces the Terraform path in `argocd/clusters/<cluster>/vault-k8s-auth/`. |
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

## Create Vault-issuer prerequisites (cert-manager + remote Vault PKI)

`create-vault-issuer` provisions the cluster-side prerequisites
cert-manager needs to use a remote Vault PKI as a `ClusterIssuer`.
The function does **not** create the ClusterIssuer itself — that's
the `cert-manager-vault-pki` AppSet's job, which references the two
Secrets this function lands.

### What it does (in one Dagger session)

1. Decrypts `--vault-env-file` and `--kubeconfig-source-file` (both
   SOPS-encrypted; same `--sops-key`).
2. Calls Vault's HTTP API directly (no Terraform, no kubernetes
   provider):
   - `PUT /v1/sys/policies/acl/<policy-name>` — upserts an ACL
     granting `create/update` on `pki/issue/*` + `pki/sign/*`.
     Idempotent.
   - `POST /v1/auth/token/create` — mints a renewable token bound to
     that policy, with `display_name=cert-manager-<cluster-name>`,
     `ttl=<token-ttl>` (default `8760h`).
   - `GET /v1/pki/ca/pem` — reads the current PKI root CA bundle.
3. `kubectl apply`s a 3-document YAML to the target cluster:
   - `Namespace/<target-namespace>` (default `cert-manager`)
   - `Secret/<token-secret-name>` (default `cert-manager-vault-token`,
     `data.token = <vault-token>`)
   - `Secret/<ca-secret-name>` (default `vault-pki-ca`,
     `data["ca.crt"] = <PKI CA PEM>`)
   Server-side apply via `dag.Kubernetes().Kubectl()`.

Only plain `core/v1` resources are created — no cert-manager CRDs, no
admission-webhook coupling. The AppSet that creates the ClusterIssuer
runs whenever it's ready; order between this function and the AppSet
doesn't matter.

### Vault env file

`--vault-env-file` is a SOPS-encrypted YAML:

```yaml
vaultAddr: https://vault.infra.sthings-vsphere.labul.sva.de
vaultToken: hvs.xxxx
vaultSkipVerify: true     # optional, defaults to true
vaultCaBundle: LS0tLS1C…  # optional, base64-encoded PKI root CA PEM;
                          # when set, used directly instead of
                          # live-fetching GET /v1/pki/ca/pem
```

The `vaultToken` here is the **admin token** used to apply the policy
and mint cert-manager's token; it never lands in any applied manifest.

`vaultCaBundle` is optional: when present, it short-circuits the live
fetch and is used directly as `data["ca.crt"]` on the rendered
`vault-pki-ca` Secret. Convenient when the same env file is reused for
other Vault flows that already carry the CA. Re-fetching from
`${vaultAddr}/v1/pki/ca/pem` is the fallback when omitted.

### Usage

```bash
# CREATE — minimal call against Vault + downstream cluster
dagger call -m argocd create-vault-issuer \
  --cluster-name homerun2-dev \
  --kubeconfig-source-file secrets/kubeconfigs/homerun2-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --progress plain
```

```bash
# CREATE — override Secret names / namespace / TTL
dagger call -m argocd create-vault-issuer \
  --cluster-name homerun2-dev \
  --kubeconfig-source-file secrets/kubeconfigs/homerun2-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --target-namespace cert-manager \
  --token-secret-name cert-manager-vault-token \
  --ca-secret-name vault-pki-ca \
  --token-ttl 8760h \
  --progress plain
```

### Token rotation

Each invocation mints a **fresh** Vault token and replaces the
`Secret/<token-secret-name>` on the target cluster. The previous
token is left to expire on its TTL (default `8760h`, renewable).
Re-running the function rotates; cert-manager picks up the new value
on the next `Issue`/`CertificateRequest`.

### Verify (after the `cert-manager-vault-pki` AppSet has reconciled)

```bash
kubectl -n cert-manager get secret cert-manager-vault-token vault-pki-ca
kubectl get clusterissuer vault-pki
```

## Create Vault Kubernetes auth backend (for ESO)

`create-vault-k8s-auth` provisions everything an in-cluster
ServiceAccount needs to authenticate to a remote Vault and consume one
or more pre-existing policies. It's the Dagger-native replacement for
the `argocd/clusters/<cluster>/vault-k8s-auth/` Terraform module
(`module "vault-base-setup"` → `k8s_auths`).

### What it does (in one Dagger session)

1. Decrypts `--vault-env-file` and `--kubeconfig-source-file` (both
   SOPS-encrypted; same `--sops-key`) and parses
   `clusters[0].cluster.server` out of the kubeconfig host-side — it's
   the `kubernetes_host` value Vault stores in the backend config.
2. `kubectl apply`s a 4-document YAML to the target cluster (server-side):
   - `Namespace/<namespace>` (default `external-secrets`)
   - `ServiceAccount/<auth-name>` (default `eso`,
     `automountServiceAccountToken: true`)
   - `Secret/<auth-name>` of type `kubernetes.io/service-account-token`
     with the `kubernetes.io/service-account.name` annotation — kubelet
     populates `data.token` + `data["ca.crt"]` once the SA exists
   - `ClusterRoleBinding/<auth-name>` → `system:auth-delegator` so
     Vault can call TokenReview using the SA's JWT
3. In one `alpine/k8s` container (kubectl + curl + jq), waits up to 60s
   for the SA-token Secret to be populated, then drives the Vault HTTP
   API directly:
   - `POST /v1/sys/auth/<cluster-name>-<auth-name>` — mounts the
     Kubernetes auth backend. Idempotent: a 400 with
     `path is already in use` is treated as success.
   - `POST /v1/auth/<cluster-name>-<auth-name>/config` —
     `kubernetes_host` + `kubernetes_ca_cert` (from the SA Secret) +
     `token_reviewer_jwt` (from the SA Secret) +
     `disable_iss_validation=true` + `disable_local_ca_jwt=true`.
   - `POST /v1/auth/<cluster-name>-<auth-name>/role/<auth-name>` —
     `bound_service_account_names`, `bound_service_account_namespaces`,
     `token_ttl`, `token_policies`. Upsert.

Re-runs upsert config + role and skip the auth-mount step.

### Policies — where they're created

This function only **binds** policies; it does not create them.
Policies are owned by the per-Vault-mount pipeline:

- For homerun2-dev's `read-homerun2-pr`:
  `clusters/labul/vsphere/infra-sthings/vault-homerun2-secrets/`
  (Terraform `module "vault-base-setup"` → `kv_policies`).
- For a new cluster `foo-dev` you'd either:
  - **Reuse** an existing policy — pass `--token-policies read-homerun2-pr`
    if it should read the same KV mount.
  - **Add a new policy** — extend the `kv_policies` list in the same
    `vault-*-secrets` Terraform module (or add a sibling
    `clusters/labul/vsphere/infra-sthings/vault-foo-secrets/`) so the
    KV mount + read policy land together. Then pass
    `--token-policies read-foo-pr` to this function.

Multiple policies per role are supported — pass a comma-separated list
(`--token-policies read-homerun2-pr,read-shared-config`). The function
splits on `,` and forwards as `token_policies` in the role payload.

### Vault env file

Same shape as `create-vault-issuer`:

```yaml
vaultAddr: https://vault.infra.sthings-vsphere.labul.sva.de
vaultToken: hvs.xxxx
vaultSkipVerify: true     # optional, defaults to true
```

The `vaultToken` here must have permission to mount auth backends and
write roles (typically the admin token, same one used for
`create-vault-issuer`).

### Usage

```bash
# CREATE — minimal call (defaults: auth-name=eso, namespace=external-secrets,
# policy=read-homerun2-pr, token-ttl=3600)
dagger call -m argocd create-vault-k8s-auth \
  --cluster-name homerun2-dev \
  --kubeconfig-source-file secrets/kubeconfigs/homerun2-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --progress plain
```

```bash
# CREATE — multiple policies, custom TTL
dagger call -m argocd create-vault-k8s-auth \
  --cluster-name homerun2-dev \
  --kubeconfig-source-file secrets/kubeconfigs/homerun2-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --token-policies "read-homerun2-pr,read-shared-config" \
  --token-ttl 7200 \
  --progress plain
```

```bash
# CREATE — different cluster + per-cluster policy + non-default SA
dagger call -m argocd create-vault-k8s-auth \
  --cluster-name foo-dev \
  --kubeconfig-source-file secrets/kubeconfigs/foo-dev.yaml \
  --vault-env-file secrets/envs/vault-infra-labul.enc.yaml \
  --sops-key env:SOPS_AGE_KEY \
  --auth-name eso \
  --namespace external-secrets \
  --token-policies read-foo-pr \
  --progress plain
```

### Resulting layout — what ESO references

After the function returns, ESO's `ClusterSecretStore` on the target
cluster wires up like this:

```yaml
spec:
  provider:
    vault:
      server: https://vault.infra.sthings-vsphere.labul.sva.de
      path: homerun2-pr               # the KV mount your policy reads
      version: v2
      auth:
        kubernetes:
          mountPath: homerun2-dev-eso # <cluster-name>-<auth-name>
          role: eso                   # <auth-name>
          serviceAccountRef:
            name: eso                 # SA created by this function
            namespace: external-secrets
```

### Verify

```bash
# Cluster side
kubectl -n external-secrets get sa eso
kubectl -n external-secrets get secret eso -o jsonpath='{.data.token}' \
  | base64 -d | head -c 40; echo …
kubectl get clusterrolebinding eso

# Vault side
VAULT_ADDR=https://vault.infra.sthings-vsphere.labul.sva.de \
  vault read auth/homerun2-dev-eso/role/eso
```

### Idempotency / re-runs

Safe to re-run. The auth-mount call short-circuits on "path is already
in use"; the config + role calls are upsert-style. The SA-token Secret
isn't rotated by this function — kubelet keeps the same value across
runs unless you delete the Secret manually.

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

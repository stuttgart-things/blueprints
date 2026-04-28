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
  --operator-version "0.42.1" \
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
  --operator-version "0.42.1" \
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
  --state-values "version=0.42.1" \
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

## Moved out of this module

| Old call | New call |
|---|---|
| `dagger call -m flux validate-age-key-pair` | `dagger call -m secrets validate-age-key-pair` |
| `dagger call -m flux flux-encrypt-secrets --secret-content $X` | `dagger call -m secrets encrypt-string --plaintext $X` |

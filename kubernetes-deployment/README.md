# KUBERNETES-DEPLOYMENT

```bash
# RENDERS HELMFILE
dagger call -m kubernetes-deployment \
deploy-helmfile \
--operation template \
--helmfile-ref "git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl" \
--progress plain
```

```bash
# APPLY BY SOURCE URL
dagger call -m kubernetes-deployment apply-manifests \
  --source-url "https://gist.githubusercontent.com/matthewpalmer/33016359f49c88acc12e86eda232f14a/raw/240e535a5e493b907ce441e4cabafdb35547d87d/config-map.yaml,https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/refs/heads/master/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml" \
  --kube-config file:///home/sthings/.kube/xplane \
  --progress plain -vv
```

```bash
# APPLY MULTIPLE CRDS AT ONCE
dagger call -m kubernetes-deployment install-custom-resource-definitions \
--kustomize-sources "https://github.com/stuttgart-things/helm/infra/crds/cilium,https://github.com/stuttgart-things/helm/infra/crds/cert-manager" \
--kube-config file:///home/sthings/.kube/xplane \
--progress plain
```

```bash
# VALIDATE AGE KEY PAIR (standalone — fails fast on mismatch)
dagger call -m kubernetes-deployment validate-age-key-pair \
  --sops-age-key env:SOPS_AGE_KEY \
  --age-public-key env:AGE_PUB \
  --progress plain
```

```bash
# FLUX BOOTSTRAP - FULL LIFECYCLE (validate keys, render, deploy operator, apply config, apply secrets, verify, wait)
dagger call -m kubernetes-deployment flux-bootstrap \
  --config-parameters "name=flux-system,namespace=flux-system,version=2.4.0,gitUrl=https://github.com/my-org/fleet,gitRef=main,gitPath=clusters/prod" \
  --kube-config file:///home/sthings/.kube/cluster \
  --src ./helmfile \
  --render-secrets \
  --git-username env:GIT_USERNAME \
  --git-password env:GIT_PASSWORD \
  --sops-age-key env:SOPS_AGE_KEY \
  --age-public-key env:AGE_PUB \
  --progress plain
```

```bash
# FLUX BOOTSTRAP - RENDER + ENCRYPT + COMMIT TO GIT (no cluster deploy)
dagger call -m kubernetes-deployment flux-bootstrap \
  --config-parameters "name=flux-system,namespace=flux-system,version=2.4.0,gitUrl=https://github.com/my-org/fleet,gitRef=main,gitPath=clusters/staging" \
  --kube-config file:///home/sthings/.kube/cluster \
  --render-secrets \
  --git-username env:GIT_USERNAME \
  --git-password env:GIT_PASSWORD \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypt-secrets \
  --age-public-key env:AGE_PUBLIC_KEY \
  --commit-to-git \
  --repository "my-org/fleet" \
  --git-token env:GITHUB_TOKEN \
  --destination-path "clusters/staging/" \
  --deploy-operator=false \
  --wait-for-reconciliation=false \
  --progress plain
```

```bash
# FLUX BOOTSTRAP - DEPLOY OPERATOR ONLY (skip rendering and git)
dagger call -m kubernetes-deployment flux-bootstrap \
  --config-parameters "name=flux-system,namespace=flux-system" \
  --kube-config file:///home/sthings/.kube/cluster \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@cicd/flux-operator.yaml.gotmpl" \
  --apply-secrets=false \
  --commit-to-git=false \
  --wait-for-reconciliation=false \
  --progress plain
```

```bash
# ONLY CREATE SECRETS ON CLUSTER
dagger call -m kubernetes-deployment flux-bootstrap \
  --kube-config file:///home/sthings/.kube/vre2.yaml \
  --deploy-operator=false \
  --commit-to-git=false \
  --config-parameters "name=flux,namespace=flux-system,version=2.4.0,gitUrl=https://github.com/stuttgart-things/stuttgart-things,gitRef=refs/heads/main,gitPath=clusters/labul/vsphere/vre2" \
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
# INDIVIDUAL PHASE FUNCTIONS (each callable standalone via dagger call)

# Render config only
dagger call -m kubernetes-deployment flux-render-config \
  --config-parameters "name=flux-system,namespace=flux-system,version=2.4.0" \
  --progress plain

# Encrypt secrets
dagger call -m kubernetes-deployment flux-encrypt-secrets \
  --secret-content "$(cat secrets.yaml)" \
  --age-public-key env:AGE_PUB \
  --progress plain

# Apply config to cluster
dagger call -m kubernetes-deployment flux-apply-config \
  --config-content "$(cat config.yaml)" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Apply secrets to cluster
dagger call -m kubernetes-deployment flux-apply-secrets \
  --secret-content "$(cat secrets.yaml)" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Verify secrets exist in cluster
dagger call -m kubernetes-deployment flux-verify-secrets \
  --secret-content "$(cat secrets.yaml)" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain

# Deploy operator only
dagger call -m kubernetes-deployment flux-deploy-operator \
  --kube-config file:///home/sthings/.kube/cluster \
  --src ./helmfile \
  --progress plain

# Wait for reconciliation
dagger call -m kubernetes-deployment flux-wait-for-reconciliation \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain
```

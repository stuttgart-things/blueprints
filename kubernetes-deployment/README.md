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
# FLUX BOOTSTRAP - FULL LIFECYCLE (render, apply secrets, deploy operator, wait)
dagger call -m kubernetes-deployment flux-bootstrap \
  --config-parameters "name=flux-system,namespace=flux-system,version=2.4.0,gitUrl=https://github.com/my-org/fleet,gitRef=main,gitPath=clusters/prod" \
  --kube-config file:///home/sthings/.kube/cluster \
  --src ./helmfile \
  --render-secrets \
  --git-username env:GIT_USERNAME \
  --git-password env:GIT_PASSWORD \
  --sops-age-key env:SOPS_AGE_KEY \
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
  --src ./helmfile \
  --apply-secrets=false \
  --commit-to-git=false \
  --progress plain
```

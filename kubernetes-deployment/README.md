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
# DEPLOY KCL - RENDER + APPLY W/ OCI SOURCE + INLINE PARAMETERS
dagger call -m kubernetes-deployment deploy-kcl \
  --oci-source ghcr.io/stuttgart-things/kcl-ansible \
  --parameters 'pipelineRunName=run-ansible-test-6,namespace=tekton-ci' \
  --kube-config file:///home/sthings/.kube/movie-scripts \
  --progress plain
```

```bash
# DEPLOY KCL - RENDER + APPLY W/ OCI SOURCE + PARAMETERS FILE
dagger call -m kubernetes-deployment deploy-kcl \
  --oci-source ghcr.io/stuttgart-things/kcl-ansible \
  --parameters-file ./params.yaml \
  --kube-config file:///home/sthings/.kube/movie-scripts \
  --progress plain
```

```bash
# DEPLOY KCL - RENDER + APPLY W/ LOCAL SOURCE
dagger call -m kubernetes-deployment deploy-kcl \
  --source ./kcl-module \
  --parameters-file ./profile.yaml \
  --kube-config file:///home/sthings/.kube/movie-scripts \
  --progress plain
```

```bash
# DEPLOY KCL - RENDER + APPLY W/ OCI SOURCE + CUSTOM NAMESPACE
dagger call -m kubernetes-deployment deploy-kcl \
  --oci-source ghcr.io/stuttgart-things/kcl-flux-instance \
  --parameters 'name=flux-system,namespace=flux-system' \
  --kube-config file:///home/sthings/.kube/movie-scripts \
  --namespace flux-system \
  --progress plain
```

```bash
# DEPLOY KCL - DELETE RESOURCES
dagger call -m kubernetes-deployment deploy-kcl \
  --oci-source ghcr.io/stuttgart-things/kcl-ansible \
  --parameters 'pipelineRunName=run-ansible-test-6,namespace=tekton-ci' \
  --kube-config file:///home/sthings/.kube/movie-scripts \
  --operation delete \
  --progress plain
```

> **SOPS-encrypted Kubernetes Secret generation moved.** `create-sops-secret`
> and `create-sops-secret-string` now live in the
> [`secrets`](../secrets/README.md) module as `create-kubernetes-secret`
> and `create-kubernetes-secret-string` (flags unchanged).

> **Flux workflows moved.** Bootstrap, render, apply, verify, destroy, and
> AGE-key validation now live in the dedicated [`flux`](../flux/README.md)
> module. The redundant `flux-` prefix has been dropped — `dagger call -m
> kubernetes-deployment flux-bootstrap` is now `dagger call -m flux
> bootstrap`, `flux-destroy` → `destroy`, etc. AGE-key validation moved to
> `dagger call -m secrets validate-age-key-pair`.

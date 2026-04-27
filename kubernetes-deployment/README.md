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

```bash
# CREATE SOPS-ENCRYPTED KUBERNETES SECRET FROM NAME/NAMESPACE/KEY-VALUES
# (default: exports encrypted manifest as file)
dagger call -m kubernetes-deployment create-sops-secret \
  --name my-secret \
  --namespace default \
  --key-values "user=admin,password=s3cret" \
  --age-public-key env:AGE_PUB \
  export --path ./secret.enc.yaml

# Print encrypted manifest to stdout (no file written) via `contents`
dagger call -m kubernetes-deployment create-sops-secret \
  --name my-secret \
  --namespace default \
  --key-values "user=admin,password=s3cret" \
  --age-public-key env:AGE_PUB \
  contents

# Same, but return the encrypted manifest as a string directly
dagger call -m kubernetes-deployment create-sops-secret-string \
  --name my-secret \
  --namespace default \
  --key-values "user=admin,password=s3cret" \
  --age-public-key env:AGE_PUB
```

> **Flux workflows moved.** Bootstrap, render, apply, verify, destroy, and
> AGE-key validation now live in the dedicated [`flux`](../flux/README.md)
> module. Replace `dagger call -m kubernetes-deployment flux-*` with
> `dagger call -m flux flux-*` (function names and flags unchanged).

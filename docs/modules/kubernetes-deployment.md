# Kubernetes Deployment Module

Render Helmfiles, apply manifests and install CRDs.

## Features

- Render Helmfiles from Git sources
- Apply Kubernetes manifests from URLs
- Install Custom Resource Definitions (CRDs) via Kustomize

## Usage

### Render Helmfile

```bash
dagger call -m kubernetes-deployment deploy-helmfile \
  --operation template \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl" \
  --progress plain
```

### Apply Manifests

Apply manifests from URLs:

```bash
dagger call -m kubernetes-deployment apply-manifests \
  --source-url "https://gist.githubusercontent.com/matthewpalmer/33016359f49c88acc12e86eda232f14a/raw/240e535a5e493b907ce441e4cabafdb35547d87d/config-map.yaml,https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/refs/heads/master/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml" \
  --kube-config file:///home/sthings/.kube/xplane \
  --progress plain -vv
```

### Install CRDs

Apply multiple CRDs at once using Kustomize:

```bash
dagger call -m kubernetes-deployment install-custom-resource-definitions \
  --kustomize-sources "https://github.com/stuttgart-things/helm/infra/crds/cilium,https://github.com/stuttgart-things/helm/infra/crds/cert-manager" \
  --kube-config file:///home/sthings/.kube/xplane \
  --progress plain
```

## Parameters

### deploy-helmfile

| Parameter | Description |
|-----------|-------------|
| `--operation` | Helmfile operation (e.g., `template`, `apply`) |
| `--helmfile-ref` | Git reference to Helmfile |

### apply-manifests

| Parameter | Description |
|-----------|-------------|
| `--source-url` | Comma-separated URLs to manifest files |
| `--kube-config` | Path to kubeconfig file |

### install-custom-resource-definitions

| Parameter | Description |
|-----------|-------------|
| `--kustomize-sources` | Comma-separated Kustomize source URLs |
| `--kube-config` | Path to kubeconfig file |

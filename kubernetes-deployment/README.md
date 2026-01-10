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
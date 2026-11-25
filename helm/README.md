# HELM

Helmfile-driven deployment workflows. Extracted from `kubernetes-deployment`
in #143 so the catch-all module shrinks to a thin orchestrator over
`kcl` + `kubectl`.

The module is named `helm-blueprint` (not `helm`): it wraps
`github.com/stuttgart-things/dagger/helm`, and Dagger rejects a module that
depends on another module with its own name. The directory and
`dagger call -m helm ...` stay the same; as a dependency it is
`dag.HelmBlueprint()`.

```bash
# RENDER HELMFILE (no cluster apply)
dagger call -m helm deploy-helmfile \
  --operation template \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl" \
  --progress plain
```

```bash
# APPLY HELMFILE TO A CLUSTER
dagger call -m helm deploy-helmfile \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain
```

```bash
# APPLY HELMFILE WITH VAULT CREDENTIALS + STATE-VALUES OVERRIDES
dagger call -m helm deploy-helmfile \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@cicd/flux-operator.yaml.gotmpl" \
  --kube-config file:///home/sthings/.kube/cluster \
  --vault-app-role-id env:VAULT_ROLE_ID \
  --vault-secret-id env:VAULT_SECRET_ID \
  --vault-url env:VAULT_ADDR \
  --vault-auth-method approle \
  --state-values "version=0.61.0" \
  --progress plain
```

```bash
# DESTROY HELMFILE RELEASE
dagger call -m helm deploy-helmfile \
  --operation destroy \
  --helmfile-ref "git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain
```

```bash
# DEPLOY MULTIPLE HELMFILE RELEASES IN SEQUENCE (comma-separated refs)
dagger call -m helm deploy-microservices \
  --helmfile-refs "git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl,git::https://github.com/stuttgart-things/helm.git@apps/cert-manager.yaml.gotmpl" \
  --kube-config file:///home/sthings/.kube/cluster \
  --progress plain
```

## Migrated from

| Old call | New call |
|---|---|
| `dagger call -m kubernetes-deployment deploy-helmfile` | `dagger call -m helm deploy-helmfile` |
| `dagger call -m kubernetes-deployment deploy-microservices` | `dagger call -m helm deploy-microservices` |

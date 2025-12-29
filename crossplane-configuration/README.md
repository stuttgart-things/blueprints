# CROSSPLANE-CONFIGURATION

## DEV

```bash
dagger call -m crossplane-configuration create \
--name openebs --variables "kind=openebs,apiGroup=resources.stuttgart-things.com,apiVersion=v1,claimKind=OpenEBS" \
--dependencies "xpkg.upbound.io/crossplane-contrib/provider-helm=>=v0.19.0" \
--progress plain -vv \
export --path=~/projects/crossplane/configurations/apps/openebs
```

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --variables 'kind=openebs,apiGroup=resources.stuttgart-things.com,apiVersion=v1,claimKind=OpenEBS,maintainer=patrick.hermann@sva.de,crossplaneVersion=2.13.0,functions=[{"Name":"function-patch-and-transform","ApiVersion":"pt.fn.crossplane.io/v1beta1","PackageURL":"xpkg.upbound.io/function-patch-and-transform","Version":"v0.1.0"},{"Name":"function-go-templating","ApiVersion":"gotemplating.fn.crossplane.io/v1beta1","PackageURL":"xpkg.upbound.io/function-go-templating","Version":"v0.1.0"}]' \
  --dependencies "xpkg.upbound.io/crossplane-contrib/provider-helm=>=v0.19.0" \
  --progress plain -vv export --path=~/projects/crossplane/configurations/apps/openebs
```

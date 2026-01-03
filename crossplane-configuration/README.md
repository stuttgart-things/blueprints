# Crossplane Configuration Module

A Dagger module for generating Crossplane configurations with flexible variable management through defaults, variable files, and CLI overrides.

## Overview

This module creates complete Crossplane package configurations including:
- **CompositeResourceDefinition (XRD)** - Defines the API for your custom claim
- **Composition** - Orchestrates how claims are composed
- **Configuration** - Package metadata and dependencies
- **Examples** - Sample claim and function definitions

## Features

<details>
<summary><b>Variable Priority System</b></summary>

Variables are merged in this priority order (highest to lowest):

1. **CLI `--variables` flag** (highest priority) - Runtime overrides
2. **`--variables-file` YAML** - Product/environment specific settings
3. **`--defaults-file` YAML** - Common defaults shared across products
4. **Built-in defaults** - Code defaults (lowest priority)

This allows you to:
- Keep common settings in `defaults.yaml`
- Override per-product in `openebs-variables.yaml`
- Override at runtime with `--variables` flag

</details>

<details>
<summary><b>Dependencies Management</b></summary>

Dependencies are defined in the YAML files as a list of providers:

```yaml
dependencies:
  - provider: xpkg.upbound.io/crossplane-contrib/provider-helm
    version: ">=v0.19.0"
  - provider: xpkg.upbound.io/crossplane-contrib/provider-kubernetes
    version: ">=v0.14.1"
```

These are rendered into the generated `crossplane.yaml` configuration file.

</details>

## Usage

### Add Cluster to crossplane

<summary><b>Click to expand</b></summary>

```bash
dagger call -m ./crossplane-configuration add-cluster \
--clusterName=pat4 \
--kubeconfig-cluster file:///home/sthings/.kube/kind-dev \
--kubeconfig-crossplane-cluster file:///home/sthings/.kube/xplane \
export --path=./output.yaml \
--progress plain -vv
```

</details>

### Render Kubeconfig Secret

<details>
<summary><b>Click to expand</b></summary>

Create a Kubernetes Secret manifest with an encoded kubeconfig for use with Crossplane:

```bash
dagger call -m crossplane-configuration render-kubeconfig-secret \
  --kubeconfig-file=file://~/.kube/config \
  --secret-name=kubeconfig-cicd \
  --secret-namespace=crossplane-system \
  --secret-key=config \
  export --path=./secret.yaml
```

**What this does:**
- Reads the kubeconfig file from the specified path
- Base64 encodes the kubeconfig content
- Renders a Kubernetes Secret manifest
- Outputs the rendered secret YAML

**Parameters:**
- `--kubeconfig-file` - Path to the kubeconfig file (supports `file://` prefix)
- `--secret-name` - Name for the Kubernetes Secret (e.g., `kubeconfig-cicd`)
- `--secret-namespace` - Namespace where the secret will be created (e.g., `crossplane-system`)
- `--secret-key` - Key name in the secret's data field (e.g., `config`, `sthings-cicd`)

**Example output (secret.yaml):**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kubeconfig-cicd
  namespace: crossplane-system
data:
  config: LS0tIGFwaVZlcnNpb246IHYxCmtpbmQ6IENvbmZpZwoK...
type: Opaque
```

You can then apply this secret to your Crossplane cluster:
```bash
kubectl apply -f secret.yaml
```

</details>

### Basic Usage with Defaults and Variables File

<details>
<summary><b>Click to expand</b></summary>

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --defaults-file ./defaults.yaml \
  --variables-file ./openebs-variables.yaml \
  export --path=./output/openebs
```

**What this does:**
- Loads common settings from `defaults.yaml`
- Overrides with product-specific settings from `openebs-variables.yaml`
- Generates Crossplane configuration in `./output/openebs`

</details>

### With CLI Variable Overrides

<details>
<summary><b>Click to expand</b></summary>

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --defaults-file ./defaults.yaml \
  --variables-file ./openebs-variables.yaml \
  --variables='crossplaneVersion=4.13.0' \
  export --path=./output/openebs
```

**What this does:**
- Loads defaults and variables file as above
- Overrides `crossplaneVersion` to `4.13.0` at runtime
- Useful for testing different versions or one-off changes

</details>

### With Complex Variable Overrides

<details>
<summary><b>Click to expand</b></summary>

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --defaults-file ./defaults.yaml \
  --variables-file ./openebs-variables.yaml \
  --variables='crossplaneVersion=4.13.0,functions=[{"Name":"function-patch-and-transform","ApiVersion":"pt.fn.crossplane.io/v1beta1","PackageURL":"xpkg.upbound.io/function-patch-and-transform","Version":"v0.1.0"}]' \
  export --path=./output/openebs
```

**What this does:**
- Overrides both simple (string) and complex (JSON array) values
- JSON values must be properly escaped and nested
- Powerful for dynamic configuration

</details>

## Configuration Files

### defaults.yaml

Common settings shared across all products.

<details>
<summary><b>Click to expand example</b></summary>

```bash
cat > defaults.yaml <<EOF
---
maintainer: patrick.hermann@sva.de
source: https://github.com/stuttgart-things
license: Apache-2.0
crossplaneVersion: "2.13.0"
claimNamespace: default
claimName: demo
xrdScope: Namespaced
xrdDeletePolicy: Foreground
dependencies:
  - provider: xpkg.upbound.io/crossplane-contrib/provider-helm
    version: ">=v0.19.0"
  - provider: xpkg.upbound.io/crossplane-contrib/provider-kubernetes
    version: ">=v0.14.1"
EOF
```

**Key fields:**
- `maintainer` - Author/owner email
- `source` - Git repository URL
- `license` - License type (Apache-2.0, MIT, etc.)
- `crossplaneVersion` - Minimum Crossplane version required
- `claimNamespace` - Default namespace for claims
- `xrdScope` - Either `Namespaced` or `Cluster`
- `xrdDeletePolicy` - How to handle deletion (`Foreground` or `Background`)
- `dependencies` - List of required Crossplane providers

</details>

### openebs-variables.yaml (Product-Specific)

Product or package-specific settings that override defaults.

<details>
<summary><b>Click to expand example</b></summary>

```bash
cat > openebs-variables.yaml <<EOF
---
kind: openebs
apiGroup: resources.stuttgart-things.com
apiVersion: v1
claimKind: OpenEBS
claimApiVersion: v1alpha1
xrdPlural: openebses
xrdSingular: openebs
compositionApiVersion: v1beta1
name: openebs
functions:
  - Name: function-patch-and-transform
    ApiVersion: pt.fn.crossplane.io/v1beta1
    PackageURL: xpkg.upbound.io/function-patch-and-transform
    Version: v0.1.0
  - Name: function-go-templating
    ApiVersion: gotemplating.fn.crossplane.io/v1beta1
    PackageURL: xpkg.upbound.io/function-go-templating
    Version: v0.1.0
EOF
```

**Key fields:**
- `kind` - The resource kind (e.g., `openebs`, `mysql`, `postgres`)
- `apiGroup` - Unique API group for your custom resources
- `apiVersion` - API version (typically `v1`)
- `claimKind` - The claim kind users will create (e.g., `OpenEBS`)
- `claimApiVersion` - Schema version for claims (typically `v1alpha1`)
- `xrdPlural` - Plural form of the kind (e.g., `openebses`)
- `xrdSingular` - Singular form (e.g., `openebs`)
- `compositionApiVersion` - Composition API version (typically `v1beta1`)
- `name` - Package name
- `functions` - List of Crossplane function packages to include

</details>

## Generated Files

<details>
<summary><b>Output Structure</b></summary>

```
openebs/
├── crossplane.yaml              # Package metadata & dependencies
├── apis/
│   ├── definition.yaml         # CompositeResourceDefinition (XRD)
│   └── composition.yaml        # Composition template
├── examples/
│   ├── claim.yaml             # Example claim resource
│   └── functions.yaml         # Function definitions
└── README.md                   # Documentation
```

**Files:**
- `crossplane.yaml` - Package configuration with maintainer info and dependencies
- `apis/definition.yaml` - Defines the custom resource API schema
- `apis/composition.yaml` - Orchestrates resource composition
- `examples/claim.yaml` - Template for creating resources
- `examples/functions.yaml` - Function package declarations
- `README.md` - Auto-generated documentation

</details>

## Examples

### Creating a PostgreSQL Configuration

<details>
<summary><b>Click to expand</b></summary>

Create `postgres-variables.yaml`:
```yaml
---
kind: postgres
apiGroup: database.stuttgart-things.com
apiVersion: v1
claimKind: PostgreSQL
claimApiVersion: v1alpha1
xrdPlural: postgresqls
xrdSingular: postgres
compositionApiVersion: v1beta1
name: postgres
functions:
  - Name: function-patch-and-transform
    ApiVersion: pt.fn.crossplane.io/v1beta1
    PackageURL: xpkg.upbound.io/function-patch-and-transform
    Version: v0.1.0
```

Then generate:
```bash
dagger call -m crossplane-configuration create \
  --name postgres \
  --defaults-file ./defaults.yaml \
  --variables-file ./postgres-variables.yaml \
  export --path=./output/postgres
```

</details>

### Overriding Dependencies at Runtime

<details>
<summary><b>Click to expand</b></summary>

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --defaults-file ./defaults.yaml \
  --variables-file ./openebs-variables.yaml \
  --variables='dependencies=[{"provider":"xpkg.upbound.io/crossplane-contrib/provider-aws","version":">=v1.0.0"}]' \
  export --path=./output/openebs-aws
```

This overrides the default Helm provider with AWS provider.

</details>

## Development

<details>
<summary><b>Testing Changes</b></summary>

When modifying the module:

```bash
# Test with verbose output
dagger call -m crossplane-configuration create \
  --name test \
  --defaults-file ./tests/crossplane-configuration/defaults.yaml \
  --variables-file ./tests/crossplane-configuration/openebs-variables.yaml \
  --progress plain -vv \
  export --path=./test-output
```

Check the generated files:
```bash
cat test-output/crossplane.yaml
cat test-output/apis/composition.yaml
```

</details>

## Notes

- All YAML files must be valid YAML (check with `yamllint`)
- JSON values in CLI variables must be properly escaped
- The module uses Go templating for all configurations
- Dependencies are mandatory - always define at least one provider

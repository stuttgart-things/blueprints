# Crossplane Configuration Module

A Dagger module for generating Crossplane configurations with flexible variable management through defaults, variable files, and CLI overrides.

## Overview

This module creates complete Crossplane package configurations including:
- **CompositeResourceDefinition (XRD)** - Defines the API for your custom resource
- **Composition** - Orchestrates how resources are composed via pipeline
- **Configuration** - Package metadata and dependencies
- **Examples** - Sample XR, function definitions, and provider config

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
# RENDER AND DEPLOY
dagger call -m ./crossplane-configuration add-cluster \
--clusterName=pat4 \
--kubeconfig-cluster file:///home/sthings/.kube/kind-dev \
--kubeconfig-crossplane-cluster file:///home/sthings/.kube/xplane \
--progress plain -vv
```

```bash
# JUST RENDER
dagger call -m ./crossplane-configuration add-cluster \
--clusterName=pat4 \
--deploy-to-cluster=false \
--kubeconfig-cluster file:///home/sthings/.kube/xplane \
export --path=/tmp/output.yaml \
--progress plain -vv
```

```bash
# RENDER WITH SOPS ENCRYPTION
# First, set the AGE public key:
export AGE_PUB="age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Then call with encryption enabled:
dagger call -m ./crossplane-configuration add-cluster \
--clusterName=pat4 \
--deploy-to-cluster=false \
--kubeconfig-cluster file:///home/sthings/.kube/xplane \
--encrypt-with-sops=true \
--age-public-key=env:AGE_PUB \
export --path=/tmp/output.yaml \
--progress plain -vv
```

```bash
# RENDER WITH SOPS ENCRYPTION AND CUSTOM SOPS CONFIG
# Use a custom .sops.yaml config file for encryption rules:
export SOPS_AGE_PUBLIC_KEY="age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

dagger call -m ./crossplane-configuration add-cluster \
--clusterName=pat4 \
--deploy-to-cluster=false \
--kubeconfig-cluster file:///home/sthings/.kube/xplane \
--encrypt-with-sops=true \
--age-public-key=env:SOPS_AGE_PUBLIC_KEY \
--sops-config=file://.sops.yaml \
export --path=/tmp/output.yaml \
--progress plain -vv
```

**SOPS Parameters:**
- `--encrypt-with-sops` - Enable SOPS encryption for the output (default: false)
- `--age-public-key` - AGE public key for encryption (required when encryption is enabled)
- `--sops-config` - Optional path to a custom `.sops.yaml` config file

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
  --variables='crossplaneVersion=4.13.0,functions=[{"Name":"crossplane-contrib-function-go-templating","ApiVersion":"pkg.crossplane.io/v1beta1","PackageURL":"xpkg.crossplane.io/crossplane-contrib/function-go-templating","Version":"v0.11.3"}]' \
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
xrdPlural: openebses
xrdSingular: openebs
name: openebs
functions:
  - Name: crossplane-contrib-function-go-templating
    ApiVersion: pkg.crossplane.io/v1beta1
    PackageURL: xpkg.crossplane.io/crossplane-contrib/function-go-templating
    Version: v0.11.3
  - Name: crossplane-contrib-function-auto-ready
    ApiVersion: pkg.crossplane.io/v1beta1
    PackageURL: xpkg.crossplane.io/crossplane-contrib/function-auto-ready
    Version: v0.6.0
EOF
```

**Key fields:**
- `kind` - The resource kind (e.g., `openebs`, `mysql`, `postgres`)
- `apiGroup` - Unique API group for your custom resources
- `xrdPlural` - Plural form of the kind (e.g., `openebses`)
- `xrdSingular` - Singular form (e.g., `openebs`)
- `name` - Package name
- `functions` - List of Crossplane function packages to include

</details>

## Generated Files

<details>
<summary><b>Output Structure</b></summary>

```
<name>/
├── crossplane.yaml              # Package metadata & dependencies
├── apis/
│   └── definition.yaml          # CompositeResourceDefinition (XRD)
├── compositions/
│   └── <name>.yaml              # Composition with pipeline
├── examples/
│   ├── <name>.yaml              # Example XR resource
│   ├── functions.yaml           # Function definitions
│   └── provider-config.yaml     # Helm ProviderConfig example
└── README.md                    # Documentation
```

**Files:**
- `crossplane.yaml` - Package configuration with maintainer info and dependencies
- `apis/definition.yaml` - Defines the custom resource API schema
- `compositions/<name>.yaml` - Composition with pipeline mode (go-templating + auto-ready)
- `examples/<name>.yaml` - Example XR for rendering with `crossplane render`
- `examples/functions.yaml` - Function package declarations
- `examples/provider-config.yaml` - Helm ProviderConfig with InjectedIdentity
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
xrdPlural: postgresqls
xrdSingular: postgres
name: postgres
functions:
  - Name: crossplane-contrib-function-go-templating
    ApiVersion: pkg.crossplane.io/v1beta1
    PackageURL: xpkg.crossplane.io/crossplane-contrib/function-go-templating
    Version: v0.11.3
  - Name: crossplane-contrib-function-auto-ready
    ApiVersion: pkg.crossplane.io/v1beta1
    PackageURL: xpkg.crossplane.io/crossplane-contrib/function-auto-ready
    Version: v0.6.0
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
cat test-output/compositions/test.yaml
cat test-output/examples/test.yaml
```

</details>

## Notes

- All YAML files must be valid YAML (check with `yamllint`)
- JSON values in CLI variables must be properly escaped
- The module uses Go templating for all configurations
- Dependencies are mandatory - always define at least one provider

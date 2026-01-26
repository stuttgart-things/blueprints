# Crossplane Configuration Module

A Dagger module for generating Crossplane configurations with flexible variable management through defaults, variable files, and CLI overrides.

## Overview

This module creates complete Crossplane package configurations including:

- **CompositeResourceDefinition (XRD)** - Defines the API for your custom claim
- **Composition** - Orchestrates how claims are composed
- **Configuration** - Package metadata and dependencies
- **Examples** - Sample claim and function definitions

## Features

### Variable Priority System

Variables are merged in this priority order (highest to lowest):

1. **CLI `--variables` flag** (highest priority) - Runtime overrides
2. **`--variables-file` YAML** - Product/environment specific settings
3. **`--defaults-file` YAML** - Common defaults shared across products
4. **Built-in defaults** - Code defaults (lowest priority)

### Dependencies Management

Dependencies are defined in YAML files as a list of providers:

```yaml
dependencies:
  - provider: xpkg.upbound.io/crossplane-contrib/provider-helm
    version: ">=v0.19.0"
  - provider: xpkg.upbound.io/crossplane-contrib/provider-kubernetes
    version: ">=v0.14.1"
```

## Usage

### Add Cluster to Crossplane

Render and deploy:

```bash
dagger call -m ./crossplane-configuration add-cluster \
  --clusterName=pat4 \
  --kubeconfig-cluster file:///home/sthings/.kube/kind-dev \
  --kubeconfig-crossplane-cluster file:///home/sthings/.kube/xplane \
  --progress plain -vv
```

Just render (without deploying):

```bash
dagger call -m ./crossplane-configuration add-cluster \
  --clusterName=pat4 \
  --deploy-to-cluster=false \
  --kubeconfig-crossplane-cluster file:///home/sthings/.kube/xplane \
  export --path=/tmp/output.yaml \
  --progress plain -vv
```

### Render Kubeconfig Secret

Create a Kubernetes Secret manifest with an encoded kubeconfig:

```bash
dagger call -m crossplane-configuration render-kubeconfig-secret \
  --kubeconfig-file=file://~/.kube/config \
  --secret-name=kubeconfig-cicd \
  --secret-namespace=crossplane-system \
  --secret-key=config \
  export --path=./secret.yaml
```

**Parameters:**

| Parameter | Description |
|-----------|-------------|
| `--kubeconfig-file` | Path to the kubeconfig file (supports `file://` prefix) |
| `--secret-name` | Name for the Kubernetes Secret |
| `--secret-namespace` | Namespace where the secret will be created |
| `--secret-key` | Key name in the secret's data field |

### Create Configuration

Basic usage with defaults and variables file:

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --defaults-file ./defaults.yaml \
  --variables-file ./openebs-variables.yaml \
  export --path=./output/openebs
```

With CLI variable overrides:

```bash
dagger call -m crossplane-configuration create \
  --name openebs \
  --defaults-file ./defaults.yaml \
  --variables-file ./openebs-variables.yaml \
  --variables='crossplaneVersion=4.13.0' \
  export --path=./output/openebs
```

## Configuration Files

### defaults.yaml Example

```yaml
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
```

### Product Variables Example

```yaml
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
```

## Generated Files

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

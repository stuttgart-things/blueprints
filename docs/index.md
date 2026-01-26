# Blueprints

**Blueprints** is a collection of modular, reusable Dagger pipelines for automating build, test, and deployment workflows in modern DevOps environments.

## Overview

These blueprints are designed for platform engineers, SREs, and developers who want to accelerate CI/CD, infrastructure automation, and code quality gates using [Dagger](https://dagger.io/).

## Available Modules

| Module | Description |
|--------|-------------|
| [Configuration](modules/configuration.md) | Render Meta/Docs, Flux-Kustomizations, vSphere-Vars and Ansible-Requirements |
| [Crossplane Configuration](modules/crossplane-configuration.md) | Generate XRD/Composition/Configuration with variable merging and cluster/secret helpers |
| [Go Microservice](modules/go-microservice.md) | Lint, test, security scan and build (ldflags/ko) for Go services |
| [Kubernetes Microservice](modules/kubernetes-microservice.md) | Build/stage/scan images, lint Dockerfiles, static analysis, Helmfile analysis (AI) |
| [Kubernetes Deployment](modules/kubernetes-deployment.md) | Render Helmfiles, apply manifests and install CRDs |
| [Repository Linting](modules/repository-linting.md) | Repository checks, aggregate findings, create issues and AI analysis |
| [VM](modules/vm.md) | Terraform + Ansible workflows with SOPS/Vault, profile-based local/remote execution |
| [VM Template](modules/vmtemplate.md) | Packer workflows with Vault/Git (build/tests), plus test VM via Terraform |
| [Presentations](modules/presentations.md) | Initialize presentation sites (Hugo), add content and serve locally |

## Getting Started

1. Clone this repository
2. Install [Dagger](https://dagger.io/) and required dependencies
3. Explore each module's documentation for usage instructions and examples

### Quick Example

Run AI-powered linting analysis:

```bash
dagger call -m repository-linting analyze-report \
  --report-file /tmp/all-findings.txt \
  export --path=/tmp/ai.txt
```

## License

Licensed under the Apache License, Version 2.0.

## Author

Patrick Hermann, stuttgart-things (2025)

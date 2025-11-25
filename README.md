# stuttgart-things/blueprints

**Blueprints** is a collection of modular, reusable Dagger pipelines for automating build, test, and deployment workflows in modern DevOps environments.

## Overview

These blueprints are designed for platform engineers, SREs, and developers who want to accelerate CI/CD, infrastructure automation, and code quality gates using Dagger.

### Available Modules

| Module | Description |
|--------|-------------|
| 💻 [VM Module](./vm/README.md) | Automate VM lifecycle with Terraform and Ansible, including Vault/SOPS integration. |
| 🖼️ [VM-Template Module](./vmtemplate/README.md) | Build and test VM templates using Packer, Vault secrets, and Git workflows. |
| 🚀 [Go Microservice](./go-microservice/README.md) | Run Go microservice CI pipelines: lint, test, coverage, and security scan. |
| ☸️ [Kubernetes Microservice](./kubernetes-microservice/README.md) | Build and stage Kubernetes container images, supporting insecure registries and platform targeting. |
| 📝 [Repository Linting](./repository-linting/README.md) | Validate and lint multiple technologies in a repository, merge findings, and analyze reports with AI. |
| 📝 [Configuration](./configuration/README.md) | This module provides functions for configuration management tasks including |

## Getting Started

1. Clone this repository.
2. Install [Dagger](https://dagger.io/) and required dependencies.
3. Explore each module's README for usage instructions and examples.

Example: Run AI-powered linting analysis

```sh
dagger call -m repository-linting analyze-report --report-file /tmp/all-findings.txt export --path=/tmp/ai.txt
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for details.

## Author

Patrick Hermann, stuttgart-things (2025)

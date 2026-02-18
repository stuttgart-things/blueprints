# stuttgart-things/blueprints

**Blueprints** is a collection of modular, reusable Dagger pipelines for automating build, test, and deployment workflows in modern DevOps environments.

## Overview

These blueprints are designed for platform engineers, SREs, and developers who want to accelerate CI/CD, infrastructure automation, and code quality gates using Dagger.

### Available Modules

| Module | Description |
|--------|-------------|
| 🧩 [Configuration](./configuration/README.md) | Render meta/docs, Flux kustomizations, vSphere vars and Ansible requirements. |
| 🧭 [Crossplane Configuration](./crossplane-configuration/README.md) | Generate XRD/Composition/Configuration with variable merging and cluster/secret helpers. |
| 🚀 [Go Microservice](./go-microservice/README.md) | Lint, test, security scan and build (ldflags/ko) for Go services. |
| ☸️ [Kubernetes Microservice](./kubernetes-microservice/README.md) | Build/stage/scan images, lint Dockerfiles, static stages, AI-powered Helmfile analysis. |
| 📦 [Kubernetes Deployment](./kubernetes-deployment/README.md) | Render Helmfiles, apply manifests and install CRDs. |
| 📝 [Repository Linting](./repository-linting/README.md) | Multi-tech repo validation (YAML/Markdown/Pre-commit/Secrets), AI analysis, GitHub issues. |
| 💻 [VM](./vm/README.md) | Terraform + Ansible workflows with SOPS/Vault, profile-driven local/remote execution. |
| 🖼️ [VM-Template](./vmtemplate/README.md) | Packer workflows with Vault/Git (build/test), plus test VM via Terraform. |
| 🎤 [Presentations](./presentations/README.md) | Initialize presentation sites (Hugo), add content and serve locally. |

## Getting Started

1. Clone this repository.
2. Install [Dagger](https://dagger.io/) and required dependencies.
3. Explore each module's README for usage instructions and examples.

Example: Run AI-powered linting analysis

```sh
dagger call -m repository-linting analyze-report --report-file /tmp/all-findings.txt export --path=/tmp/ai.txt
```

## Quick Examples

- Configuration (render Flux kustomization):
	- `dagger call -m configuration render-flux-kustomization --oci-source ghcr.io/stuttgart-things/kcl-flux-instance -vv --progress plain`
- Crossplane Configuration (add cluster):
	- `dagger call -m ./crossplane-configuration add-cluster --clusterName=demo --kubeconfig-crossplane-cluster file://~/.kube/xplane -vv`
- Go Microservice (build with ldflags):
	- `dagger call -m go-microservice run-build-stage --src tests/go-microservice/ldflags/ --ko-build=false --ldflags "main.Version=1.2.5; main.Commit=abc1234" -vv`
- Kubernetes Deployment (render Helmfile):
	- `dagger call -m kubernetes-deployment deploy-helmfile --operation template --helmfile-ref git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl`
- Kubernetes Microservice (build image):
	- `dagger call -m kubernetes-microservice bake-image --src tests/kubernetes-microservice --repository-name stuttgart-things/test --registry-url ttl.sh --tag 1.2.3 -vv`
- Repository Linting (validate):
	- `dagger call -m repository-linting validate-multiple-technologies --src tests/repository-linting/test-repo --enable-pre-commit=true --enable-secrets=true --fail-on any export --path /tmp/all-findings.txt`
- VM (bake locally with Terraform/Ansible):
	- `dagger call -m vm bake-local --terraform-dir ~/projects/terraform/vms/sthings-runner/ --operation apply -vv`
- VM-Template (Packer workflow):
	- `dagger call -m vmtemplate run-vsphere-workflow --git-repository ~/projects/stuttgart-things/stuttgart-things --git-workdir packer/builds/ubuntu24-labda-vsphere -vv`
- Presentations (initialize):
	- `dagger call -m presentations init --name backstage export --path=/tmp/presentation`

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for details.

## Author

Patrick Hermann, stuttgart-things (2025)

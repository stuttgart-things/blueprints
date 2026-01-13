# stuttgart-things/blueprints

**Blueprints** is a collection of modular, reusable Dagger pipelines for automating build, test, and deployment workflows in modern DevOps environments.

## Overview

These blueprints are designed for platform engineers, SREs, and developers who want to accelerate CI/CD, infrastructure automation, and code quality gates using Dagger.

### Available Modules

| Module | Description |
|--------|-------------|
| 🧩 [Configuration](./configuration/README.md) | Render Meta/Docs, Flux-Kustomizations, vSphere-Vars und Ansible-Requirements. |
| 🧭 [Crossplane Configuration](./crossplane-configuration/README.md) | Generiert XRD/Composition/Configuration inkl. Variablen-Merging und Cluster-/Secret-Helfer. |
| 🚀 [Go Microservice](./go-microservice/README.md) | Lint/Tests/Security-Scan und Build (ldflags/ko) für Go-Services. |
| ☸️ [Kubernetes Microservice](./kubernetes-microservice/README.md) | Images bauen/stagen/scannen, Dockerfile linten, Static-Stage, Helmfile-Analyse (AI). |
| 📦 [Kubernetes Deployment](./kubernetes-deployment/README.md) | Helmfile rendern, Manifeste anwenden und CRDs installieren. |
| 📝 [Repository Linting](./repository-linting/README.md) | Repo-Checks, Findings aggregieren, Issues erstellen und AI-Analyse. |
| 💻 [VM](./vm/README.md) | Terraform + Ansible Workflows, SOPS/Vault, Profile-gestützt lokal/remote. |
| 🖼️ [VM-Template](./vmtemplate/README.md) | Packer-Workflows mit Vault/Git (Build/Tests), plus Test-VM via Terraform. |
| 🎤 [Presentations](./presentations/README.md) | Präsentationsseiten (Hugo) initialisieren, Content hinzufügen und lokal serven. |

## Getting Started

1. Clone this repository.
2. Install [Dagger](https://dagger.io/) and required dependencies.
3. Explore each module's README for usage instructions and examples.

Example: Run AI-powered linting analysis

```sh
dagger call -m repository-linting analyze-report --report-file /tmp/all-findings.txt export --path=/tmp/ai.txt
```

## Quick Examples

- Configuration (Flux-Kustomization rendern):
	- `dagger call -m configuration render-flux-kustomization --oci-source ghcr.io/stuttgart-things/kcl-flux-instance -vv --progress plain`
- Crossplane Configuration (Cluster hinzufügen):
	- `dagger call -m ./crossplane-configuration add-cluster --clusterName=demo --kubeconfig-crossplane-cluster file://~/.kube/xplane -vv`
- Go Microservice (Build mit ldflags):
	- `dagger call -m go-microservice run-build-stage --src tests/go-microservice/ldflags/ --ko-build=false --ldflags "main.Version=1.2.5; main.Commit=abc1234" -vv`
- Kubernetes Deployment (Helmfile rendern):
	- `dagger call -m kubernetes-deployment deploy-helmfile --operation template --helmfile-ref git::https://github.com/stuttgart-things/helm.git@apps/nginx.yaml.gotmpl`
- Kubernetes Microservice (Image bauen):
	- `dagger call -m kubernetes-microservice bake-image --src tests/kubernetes-microservice --repository-name stuttgart-things/test --registry-url ttl.sh --tag 1.2.3 -vv`
- Repository Linting (validieren):
	- `dagger call -m repository-linting validate-multiple-technologies --src tests/repository-linting/test-repo export --path /tmp/all-findings.txt`
- VM (Bake lokal mit Terraform/Ansible):
	- `dagger call -m vm bake-local --terraform-dir ~/projects/terraform/vms/sthings-runner/ --operation apply -vv`
- VM-Template (Packer-Workflow):
	- `dagger call -m vmtemplate run-vsphere-workflow --git-repository ~/projects/stuttgart-things/stuttgart-things --git-workdir packer/builds/ubuntu24-labda-vsphere -vv`
- Presentations (initialisieren):
	- `dagger call -m presentations init --name backstage export --path=/tmp/presentation`

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for details.

## Author

Patrick Hermann, stuttgart-things (2025)

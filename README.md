# stuttgart-things/blueprints

collection of modular, reusable Dagger pipelines for automating build, test &amp; deployment workflows

### 🧩 Modular Dagger Pipelines

| Module                 | Link                                                      | Description                                                                                      |
|------------------------|-----------------------------------------------------------|--------------------------------------------------------------------------------------------------|
| **VM Module**          | [📘 vm/README.md](./vm/README.md)                         | Automates VM lifecycle with Terraform and Ansible, integrates Vault/SOPS.                       |
| **VM-Template Module** | [📘 vmtemplate/README.md](./vmtemplate/README.md)         | Builds and tests VM templates using Packer, Vault secrets, and Git SCM workflows.               |
| **Go Microservice**    | [📘 go-microservice/README.md](./go-microservice/README.md) | Executes a Go microservice CI pipeline with linting, testing, coverage analysis, and security scanning. |
| **Kubernetes Microservice** | [📘 kubernetes-microservice/README.md](./kubernetes-microservice/README.md) | Builds and stages Kubernetes container images with support for insecure registries and platform targeting. |


## LICENSE

<details><summary><b>APACHE 2.0</b></summary>

Copyright 2023 patrick hermann.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

</details>

```yaml
Author Information
------------------
Patrick Hermann, stuttgart-things 06/2025
```

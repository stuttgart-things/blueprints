# stuttgart-things/blueprints

collection of modular, reusable Dagger pipelines for automating build, test &amp; deployment workflows

## /VM

```mermaid
flowchart TD
    enc[terraform.tfvars.enc.json] --> A[SOPS Decrypt]
    A --> plain[terraform.tfvars.json]
    plain --> B[Terraform Apply]
    B --> infra[Infrastructure Created]
    infra --> C[Generate Ansible Inventory YAML]
    C --> out[inventory.yaml]
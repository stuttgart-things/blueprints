# stuttgart-things/blueprints/vm

```mermaid
flowchart TD
    enc[terraform.tfvars.enc.json] --> A[SOPS Decrypt]
    A --> plain[terraform.tfvars.json]
    plain --> B[Terraform Apply]
    B --> infra[Infrastructure Created]
    infra --> C[Generate Ansible Inventory YAML]
    C --> out[inventory.yaml]


<details><summary>BUILD NEW VM</summary>

```bash
dagger call -m vm bake \
--terraform-dir tests/vm/tf \
--encrypted-file tests/vm/terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
-vv --progress plain \
export --path=~/projects/terraform/vms/dagger/
```

```bash
export SSH_USER=sthings
export SSH_PASSWORD=<REPLACEME>

dagger call -m vm bake \
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--encrypted-file /home/sthings/projects/stuttgart-things/terraform/secrets/labda-terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-requirements-file tests/vm/requirements.yaml \
--ansible-user=env:SSH_USER \
--ansible-password=env:SSH_PASSWORD \
--ansible-parameters "send_to_homerun=false" \
--ansible-playbooks "sthings.baseos.setup" \
-vv --progress plain \
export --path=~/projects/terraform/vms/sthings-runner/
```

</details>

<details><summary>APPLY OVER EXISTING WORKSPACE/DIR</summary>

```bash
dagger call -m vm bake \
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--encrypted-file /home/sthings/projects/stuttgart-things/terraform/secrets/ labda-terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
-vv --progress plain \
export --path=~/projects/terraform/vms/sthings-runner/
```

</details>

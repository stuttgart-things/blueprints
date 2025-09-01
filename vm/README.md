# stuttgart-things/blueprints/vm

## 📊 Diagram: VM Provisioning Flow

```mermaid
flowchart TD
    enc[terraform.tfvars.enc.json] --> A[SOPS Decrypt]
    A --> plain[terraform.tfvars.json]
    plain --> B[Terraform Apply]
    B --> infra[Infrastructure Created]
    infra --> C[Generate Ansible Inventory YAML]
    C --> out[inventory.yaml]
```

## WORKFLOWS

<details><summary>BAKE LOCAL</summary>

```bash
# TERRAFORM SECRETS SOPS ENCRYPTED
export SSH_USER=sthings
export SSH_PASSWORD=<REPLACEME>

dagger call -m vm bake-local \
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

```bash
# TERRAFORM SECRETS FROM VAULT
export SSH_USER=sthings
export SSH_PASSWORD=<REPLACEME>

dagger call -m vm bake-local \
--terraform-dir ~/projects/terraform/vms/sthings-runner \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--variables "vault_addr=https://vault-vsphere.tiab.labda.sva.de:8200" \
--ansible-requirements-file tests/vm/requirements.yaml \
--ansible-playbooks "sthings.baseos.setup" \
--ansible-user=env:ANSIBLE_USER \
--ansible-password=env:ANSIBLE_PASSWORD \
--ansible-wait-timeout=90 \
--ansibleParameters="send_to_homerun=false" \
--progress plain -vv \
export --path=~/projects/terraform/vms/sthings-runner/
```

```bash
dagger call -m vm bake \
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--encrypted-file /home/sthings/projects/stuttgart-things/terraform/secrets/labda-terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-user=env:SSH_USER \
--ansible-password=env:SSH_PASSWORD \
--ansible-parameters "send_to_homerun=false" \
--ansible-playbooks "sthings.baseos.setup" \
-vv --progress plain \
export --path=~/projects/terraform/vms/sthings-runner/
```

</details>

<details><summary>DESTROY</summary>

```bash
dagger call -m vm bake-local \
--operation destroy
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--variables "vault_addr=https://vault-vsphere.example.com:8200" \
--ansible-requirements-file tests/vm/requirements.yaml \
--ansible-playbooks "sthings.baseos.setup" \
--ansible-user=env:ANSIBLE_USER \
--ansible-password=env:ANSIBLE_PASSWORD \
--progress plain -vv \
```

</details>


## FUNCTIONS

<details><summary>DECRYPT FILE w/ SOPS</summary>

```bash
dagger call -m vm \
decrypt-sops \
--sops-key=env:SOPS_AGE_KEY \
--encrypted-file tests/vm/terraform.tfvars.enc.json
```

</details>

<details><summary>EXECUTE TERRAFORM</summary>

```bash
# APPLY
dagger call -m vm \
execute-terraform \
--terraform-dir tests/vmtemplate/tftest \
--operation apply \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--variables "vault_addr=https://vault-vsphere.example.com:8200" \
--progress plain -vv \
export --path=/tmp/dagger/tests/terraform/
```

```bash
# DESTROY
dagger call -m vm \
execute-terraform \
--terraform-dir /tmp/dagger/tests/terraform/ \
--operation destroy \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--variables "vault_addr=https://vault-example.com:8200" \
--progress plain -vv
```

</details>

<details><summary>OUTPUT TERRAFORM RUN</summary>

```bash
dagger call -m vm \
output-terraform-run \
--terraform-dir=~/tmp/dagger/tests/terraform/ \
--progress plain -vv \
```

</details>

<details><summary>RUN ANSIBLE</summary>

```bash
dagger call -m vm \
execute-ansible \
--src . \
--playbooks tests/vm/ansible/vault-test.yaml \
--requirements tests/vm/ansible/requirements.yaml \
--inventory tests/vm/ansible/inventory \
--vaultAppRoleID env:VAULT_ROLE_ID \
--vaultSecretID env:VAULT_SECRET_ID \
--vaultURL env:VAULT_ADDR \
-vv --progress plain

```

</details>

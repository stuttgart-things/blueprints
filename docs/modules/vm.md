# VM Module

Terraform + Ansible workflows with SOPS/Vault integration, profile-based local and remote execution.

## Features

- Execute Terraform operations (apply, destroy)
- Run Ansible playbooks against hosts
- SOPS-encrypted secrets support
- HashiCorp Vault integration
- Profile-based workflow configuration
- T-shirt size VM configurations

## Usage

### Run Ansible

Basic Ansible execution without inventory:

```bash
dagger call -m vm execute-ansible \
  --playbooks "sthings.baseos.setup" \
  --hosts "10.31.103.58" \
  --ssh-user=env:SSH_USER \
  --ssh-password=env:SSH_PASSWORD \
  --progress plain -vv
```

With requirements file and multiple playbooks:

```bash
dagger call -m vm execute-ansible \
  --playbooks "sthings.baseos.setup,sthings.container.kind_xplane" \
  --hosts "10.31.103.27" \
  --ssh-user=env:SSH_USER \
  --ssh-password=env:SSH_PASSWORD \
  --requirements /tmp/requirements.yaml \
  --progress plain -vv
```

With Vault and inventory:

```bash
dagger call -m vm execute-ansible \
  --src . \
  --playbooks tests/vm/ansible/vault-test.yaml \
  --requirements tests/vm/ansible/requirements.yaml \
  --inventory tests/vm/ansible/inventory \
  --vaultAppRoleID env:VAULT_ROLE_ID \
  --vaultSecretID env:VAULT_SECRET_ID \
  --vaultURL env:VAULT_ADDR \
  -vv --progress plain
```

### Bake Local (Terraform + Ansible)

With SOPS-encrypted secrets:

```bash
export SSH_USER=sthings
export SSH_PASSWORD=<password>

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

With Vault secrets:

```bash
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
  --progress plain -vv \
  export --path=~/projects/terraform/vms/sthings-runner/
```

### Bake Local by Profile

Create a profile configuration:

```yaml
---
operation: apply
variables:
  - vault_addr=https://vault-vsphere.tiab.labda.sva.de:8200
ansiblePlaybooks:
  - "sthings.baseos.setup"
ansibleParameters: []
ansibleInventoryType: default
ansibleWaitTimeout: 30
ansibleRequirementsFile: ./requirements.yaml
encryptedFile: ""
```

Run with profile:

```bash
dagger call -m vm bake-local-by-profile \
  --src ./ \
  --profile vm.yaml \
  --vault-secret-id env:VAULT_SECRET_ID \
  --vault-role-id env:VAULT_ROLE_ID \
  --ansible-user env:ANSIBLE_USER \
  --ansible-password env:ANSIBLE_PASSWORD \
  --progress plain -vv \
  export --path ./
```

### Destroy VMs

```bash
dagger call -m vm bake-local \
  --operation destroy \
  --terraform-dir ~/projects/terraform/vms/sthings-runner/ \
  --vault-secret-id env:VAULT_SECRET_ID \
  --vault-role-id env:VAULT_ROLE_ID \
  --variables "vault_addr=https://vault-vsphere.example.com:8200" \
  --ansible-requirements-file tests/vm/requirements.yaml \
  --ansible-playbooks "sthings.baseos.setup" \
  --ansible-user=env:ANSIBLE_USER \
  --ansible-password=env:ANSIBLE_PASSWORD \
  --progress plain -vv
```

### Execute Terraform

Apply:

```bash
dagger call -m vm execute-terraform \
  --terraform-dir tests/vmtemplate/tftest \
  --operation apply \
  --vault-secret-id env:VAULT_SECRET_ID \
  --vault-role-id env:VAULT_ROLE_ID \
  --variables "vault_addr=https://vault-vsphere.example.com:8200" \
  --progress plain -vv \
  export --path=/tmp/dagger/tests/terraform/
```

Destroy:

```bash
dagger call -m vm execute-terraform \
  --terraform-dir /tmp/dagger/tests/terraform/ \
  --operation destroy \
  --vault-secret-id env:VAULT_SECRET_ID \
  --vault-role-id env:VAULT_ROLE_ID \
  --variables "vault_addr=https://vault-example.com:8200" \
  --progress plain -vv
```

### Decrypt SOPS File

```bash
dagger call -m vm decrypt-sops \
  --sops-key=env:SOPS_AGE_KEY \
  --encrypted-file tests/vm/terraform.tfvars.enc.json
```

### Get Terraform Output

```bash
dagger call -m vm output-terraform-run \
  --terraform-dir=~/tmp/dagger/tests/terraform/ \
  --progress plain -vv
```

### Get T-Shirt Size

```bash
dagger call -m vm tshirt-size \
  --config-file=tests/vm/config/vm-tshirt-sizes.yaml \
  --size=medium \
  -vv --progress plain
```

## Parameters

### execute-ansible

| Parameter | Description |
|-----------|-------------|
| `--playbooks` | Comma-separated playbook names or paths |
| `--hosts` | Target hosts |
| `--ssh-user` | SSH username |
| `--ssh-password` | SSH password |
| `--requirements` | Path to requirements file |
| `--inventory` | Path to inventory file |
| `--inventoryType` | Inventory type (default, cluster) |
| `--parameters` | Additional Ansible parameters |

### bake-local

| Parameter | Description |
|-----------|-------------|
| `--terraform-dir` | Terraform directory |
| `--operation` | Terraform operation (apply, destroy) |
| `--encrypted-file` | SOPS-encrypted file path |
| `--sops-key` | SOPS AGE key |
| `--vault-secret-id` | Vault secret ID |
| `--vault-role-id` | Vault role ID |
| `--variables` | Additional variables |
| `--ansible-*` | Ansible configuration parameters |

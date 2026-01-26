# VM Template Module

Packer workflows with Vault/Git integration for building VM templates, plus test VM creation via Terraform.

## Features

- Run Packer builds for VM templates
- vSphere workflow integration
- Vault authentication support
- Git-based configuration sources
- Test VM provisioning via Terraform

## Usage

### vSphere Workflow (Git Source)

Run a complete vSphere Packer workflow from a Git repository:

```bash
export VAULT_TOKEN=<your-token>
export VAULT_ROLE_ID=<your-role-id>
export VAULT_SECRET_ID=<your-secret-id>

dagger call -m vmtemplate run-vsphere-workflow \
  --git-repository ~/projects/stuttgart-things/stuttgart-things \
  --git-ref main \
  --git-token env:GITHUB_TOKEN \
  --git-workdir packer/builds/ubuntu24-labda-vsphere \
  --packer-config ubuntu24-base-os.pkr.hcl \
  --packer-version 1.13.1 \
  --vault-addr https://vault-vsphere.tiab.labda.sva.de:8200 \
  --vault-token env:VAULT_TOKEN \
  --vault-role-id env:VAULT_ROLE_ID \
  --vault-secret-id env:VAULT_SECRET_ID \
  --progress plain -vv 2>&1 | tee /tmp/packer-log.txt
```

### Run Packer Build

Basic Packer build for testing:

```bash
dagger call -m vmtemplate bake \
  --packer-config-dir tests/vmtemplate/hello \
  --packer-config hello.pkr.hcl \
  --packer-version 1.13.1 \
  --progress plain -vv
```

### Create Test VM

Provision a test VM using Terraform:

```bash
export VAULT_ROLE_ID=<your-role-id>
export VAULT_SECRET_ID=<your-secret-id>

dagger call -m vmtemplate create-test-vm \
  --terraform-dir tests/vmtemplate/tfvaulttest \
  --vault-secret-id env:VAULT_SECRET_ID \
  --vault-role-id env:VAULT_ROLE_ID \
  --variables "vault_addr=https://vault-example.com:8200" \
  --operation apply \
  -vv --progress plain \
  export --path=~/tmp/dagger/tests/terraform/
```

## Parameters

### run-vsphere-workflow

| Parameter | Description |
|-----------|-------------|
| `--git-repository` | Path to Git repository |
| `--git-ref` | Git branch or tag |
| `--git-token` | GitHub token |
| `--git-workdir` | Working directory within the repo |
| `--packer-config` | Packer configuration file name |
| `--packer-version` | Packer version to use |
| `--vault-addr` | Vault server address |
| `--vault-token` | Vault token |
| `--vault-role-id` | Vault AppRole role ID |
| `--vault-secret-id` | Vault AppRole secret ID |

### bake

| Parameter | Description |
|-----------|-------------|
| `--packer-config-dir` | Directory containing Packer config |
| `--packer-config` | Packer configuration file name |
| `--packer-version` | Packer version to use |

### create-test-vm

| Parameter | Description |
|-----------|-------------|
| `--terraform-dir` | Terraform directory |
| `--operation` | Terraform operation (apply, destroy) |
| `--vault-secret-id` | Vault secret ID |
| `--vault-role-id` | Vault role ID |
| `--variables` | Additional Terraform variables |

## Vault Integration

The module supports HashiCorp Vault for secure credential management:

- **AppRole Authentication**: Use `--vault-role-id` and `--vault-secret-id`
- **Token Authentication**: Use `--vault-token` directly
- **Vault Address**: Specify with `--vault-addr`

## Test Data

Example test configurations can be found in:

- `tests/vmtemplate/hello/` - Basic Packer test
- `tests/vmtemplate/tfvaulttest/` - Terraform with Vault test

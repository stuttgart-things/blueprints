# stuttgart-things/blueprints/configuration

<details><summary>RENDER-FLUX-KUSTOMIZATION</summary>

```bash
dagger call -m configuration \
render-flux-kustomization \
--oci-source ghcr.io/stuttgart-things/kcl-flux-instance \
 -vv --progress plain
```

```bash
dagger call -m configuration \
render-flux-kustomization \
--oci-source ghcr.io/stuttgart-things/kcl-flux-instance \
--token=env:GITHUB_TOKEN \
--branch-name=test \
--create-branch=true \
--commit-changes=true \
--file-name=flux-instance \
--repository stuttgart-things/dagger \
-vv --progress plain
```

</details>

<details><summary>CREATE VSPHERE CONFIG</summary>

```bash
# EXAMPLE WITH ALL PARAMETERS SPECIFIED
dagger call -m configuration vsphere-vm \
--src ./ \
--config-parameters "name=demo-infra1,count=4,ram=8192,template=sthings-u24,disk=64,cpu=8,firmware=bios,vm_folder=stuttgart-things/testing,datacenter=/LabUL,datastore=/LabUL/datastore/UL-ESX-SAS-02,resourcePool=/LabUL/host/Cluster-V6.5/Resources,network=/LabUL/network/LAB-10.31.103,useVault=false,vaultSecretPath=vsphere-labul" \
--token=env:GITHUB_TOKEN \
--repository "stuttgart-things/blueprints" \
--branch-name test \
--commit-message "Add vSphere VM configuration for demo-infra1 in LabUL" \
--destination-folder "demo-infra1-LabUL" \
--destination-base-path "./" \
--author-name "John Doe" \
--author-email "john.doe@example.com" \
--pull-request-title "Add vSphere VM configuration for demo-infra1 in LabUL" \
--pull-request-body "This PR adds the rendered vSphere VM configuration for demo-infra1 in datacenter LabUL." \
--create-branch=true \
--commit-config=true \
--create-pull-request=true \
export --path=./demo-infra1

# MINIMAL EXAMPLE
dagger call -m configuration vsphere-vm \
--src ./ \
--config-parameters "name=demo-infra1,count=4,ram=8192,template=sthings-u24,disk=64,cpu=8,firmware=bios,vm_folder=stuttgart-things/testing,datacenter=/LabUL,datastore=/LabUL/datastore/UL-ESX-SAS-02,resourcePool=/LabUL/host/Cluster-V6.5/Resources,network=/LabUL/network/LAB-10.31.103,useVault=false,vaultSecretPath=vsphere-labul" \
--token=env:GITHUB_TOKEN \
--repository "stuttgart-things/blueprints" \
--create-branch=false \
--commit-config=false \
--create-pull-request=false \
export --path=./demo-infra1
```

</details>

<details><summary>RENDER ANSIBLE REQUIREMENTS FILE</summary>

```bash
dagger call -m configuration create-ansible-requirement-files \
--template-paths https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl \
--data-file https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml \
export --path /tmp/ansible-output \
-vv --progress plain
```

</details>

<details><summary>RENDER META INFORMATION</summary>

```bash
# RENDER README
dagger call -m configuration render-metadata \
--src ./tests/configuration \
--template-path README.md.tmpl \
--data-files vm-ansible.yaml,other-vars.yaml \
export --path /tmp/readme-output \
-vv --progress plain
```

```bash
# RENDER EXECUTIONFILE
dagger call -m configuration render-metadata \
--src tests/vm \
--template-path osaka-profile.yaml.tmpl \
--data-files osaka-profile-vars.yaml \
export --path /tmp/execution-output \
-vv --progress plain
```

</details>

<details><summary>GET TSHIRT SIZE</summary>

```bash
dagger call -m configuration vsphere-vm \
--src "./" \
--template-paths tests/configuration/vm.tf.tpl \ --config-parameters "name=bla,count=2" \
export --path=/tmp/vm/
```

</details>

<details><summary>CREATE SECRETS FILE</summary>

Decrypts a SOPS-encrypted data file with an AGE key, renders a Go template against the decrypted values, and optionally re-encrypts the rendered output with SOPS.

### Render and re-encrypt (default)

```bash
dagger call -m configuration create-secrets-file \
  --age-key env:SOPS_AGE_KEY \
  --age-recipient cmd:"echo age19vgzvmpt9tdlcsu8rzaacj397yz8gguz38nsmuy6eeelt5vjsyms542xtm" \
  --encrypted-data-file tests/vm/terraform.tfvars.enc.json \
  --template-file tests/configuration/vault-secret.json.tmpl \
  --file-extension json \
  --progress plain -vv \
  export --path /tmp/secrets/vault-secret.enc.json
```

### Render plaintext (skip re-encryption)

```bash
dagger call -m configuration create-secrets-file \
  --age-key env:SOPS_AGE_KEY \
  --encrypted-data-file tests/vm/terraform.tfvars.enc.json \
  --template-file tests/configuration/vault-secret.json.tmpl \
  --encrypt=false \
  --progress plain -vv \
  export --path /tmp/secrets/vault-secret.json
```

### With a custom .sops.yaml

```bash
dagger call -m configuration create-secrets-file \
  --age-key env:SOPS_AGE_KEY \
  --age-recipient env:SOPS_AGE_RECIPIENT \
  --encrypted-data-file ./secrets/vault-infra-labul.enc.yaml \
  --template-file ./templates/vault-secret.json.tmpl \
  --sops-config ./.sops.yaml \
  --file-extension json \
  export --path /tmp/secrets/vault-secret.enc.json
```

### Parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `--age-key` | Secret | yes | Private AGE key (`AGE-SECRET-KEY-...`) used to decrypt the input |
| `--age-recipient` | Secret | when `encrypt=true` | Public AGE recipient (`age1...`) used to re-encrypt the output |
| `--encrypted-data-file` | File | yes | SOPS-encrypted YAML/JSON data file |
| `--template-file` | File | yes | Go template rendered against the decrypted values |
| `--file-extension` | string | no | Output extension for SOPS re-encryption (default `json`) |
| `--sops-config` | File | no | Optional `.sops.yaml` used for decrypt and encrypt |
| `--encrypt` | bool | no | Re-encrypt rendered output (default `true`); `false` returns plaintext |

Template keys reference fields from the decrypted data via Go template syntax (`{{ .myKey }}`). The template uses strict mode — missing keys fail the render.

</details>

<details><summary>TERRAFORM APPLY</summary>

### Basic apply with SOPS decryption and Kubernetes backend

```bash
dagger call -m configuration terraform-apply \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --progress plain -vv
```

### Apply with terraform output --json

```bash
dagger call -m configuration terraform-apply \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --export-tf-output \
  file --path output.json contents \
  --progress plain -vv
```

### Apply with AWS/S3 backend

```bash
dagger call -m configuration terraform-apply \
  --terraform-dir /path/to/terraform/configs \
  --aws-access-key-id env:AWS_ACCESS_KEY_ID \
  --aws-secret-access-key env:AWS_SECRET_ACCESS_KEY \
  --progress plain -vv
```

### Apply with Vault credentials

```bash
dagger call -m configuration terraform-apply \
  --terraform-dir /path/to/terraform/configs \
  --vault-token env:VAULT_TOKEN \
  --progress plain -vv
```

### Apply with Kubernetes secret lookup (e.g. retrieve Vault token from cluster)

```bash
dagger call -m configuration terraform-apply \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --kube-secret-name "vault-root-token" \
  --kube-secret-namespace "vault" \
  --kube-secret-jsonpath ".data.root_token" \
  --kube-secret-tf-var "vault_token" \
  --progress plain -vv
```

### Terraform destroy

```bash
dagger call -m configuration terraform-apply \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --operation destroy \
  --progress plain -vv
```

### Parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `--terraform-dir` | Directory | yes | Directory containing terraform configurations |
| `--sops-age-key` | Secret | no | AGE key for SOPS decryption |
| `--encrypted-files` | string | no | Comma-separated SOPS-encrypted file paths to decrypt |
| `--operation` | string | no | Terraform operation: `apply` (default), `destroy`, `init` |
| `--variables` | string | no | Comma-separated terraform variables (e.g. `name=patrick,food=schnitzel`) |
| `--kube-config` | Secret | no | Kubeconfig for Kubernetes state backend |
| `--kube-config-path` | string | no | Container path for kubeconfig (must match `config_path` in backend.tf) |
| `--encrypted-kube-config` | File | no | SOPS-encrypted kubeconfig file |
| `--kube-secret-name` | string | no | Kubernetes secret name to read |
| `--kube-secret-namespace` | string | no | Kubernetes namespace for the secret |
| `--kube-secret-jsonpath` | string | no | JSONPath to extract from the Kubernetes secret |
| `--kube-secret-tf-var` | string | no | Terraform variable name to set from the extracted secret value |
| `--aws-access-key-id` | Secret | no | AWS access key for S3/MinIO backend |
| `--aws-secret-access-key` | Secret | no | AWS secret access key for S3/MinIO backend |
| `--vault-role-id` | Secret | no | Vault role ID |
| `--vault-secret-id` | Secret | no | Vault secret ID |
| `--vault-token` | Secret | no | Vault token |
| `--export-tf-output` | bool | no | Run `terraform output --json` after apply, writes `output.json` |

### SOPS env var handling

SOPS-encrypted JSON files containing `VAULT_TOKEN`, `VAULT_ADDR`, or `VAULT_SKIP_VERIFY` are automatically extracted as container environment variables instead of being written as terraform variable files. Remaining keys are written to `terraform.tfvars.json` as usual.

</details>

<details><summary>TERRAFORM OUTPUT</summary>

### Retrieve terraform outputs from an existing state

```bash
dagger call -m configuration terraform-output \
  --terraform-dir /path/to/terraform/state-dir \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --progress plain -vv
```

### With AWS/S3 backend

```bash
dagger call -m configuration terraform-output \
  --terraform-dir /path/to/terraform/state-dir \
  --aws-access-key-id env:AWS_ACCESS_KEY_ID \
  --aws-secret-access-key env:AWS_SECRET_ACCESS_KEY \
  --progress plain -vv
```

</details>

## CREATE LOCAL CONFIG

```bash
dagger call -m configuration vsphere-vm \
--src ./ \
--config-parameters "name=demo3,count=4,ram=8192,template=sthings-u24,disk=64,cpu=8,firmware=bios,vm_folder=stuttgart-things/testing,datacenter=/LabUL,datastore=/LabUL/datastore/UL-ESX-SAS-02,resourcePool=/LabUL/host/Cluster-V6.5/Resources,network=/LabUL/network/LAB-10.31.103,useVault=false,vaultSecretPath=vsphere-labul" \
--create-branch=false \
--commit-config=false \
--create-pull-request=false \
export --path=/tmp/demo3
```

## CREATE ANSIBLE REQUIREMENTS

```bash
dagger call -m configuration create-ansible-requirement-files \
--template-paths https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl \
--data-file https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml \
export --path /tmp/demo3 \
-vv --progress plain
```

## RENDER README

```bash
dagger call -m configuration render-vm-readme \
--src ./tests/configuration \
--template-path README.md.tmpl \
--data-files vm-ansible.yaml,other-vars.yaml \
--config-parameters="vm=demo3,profile=baseos" \
export --path /tmp/demo3 \
-vv --progress plain
```

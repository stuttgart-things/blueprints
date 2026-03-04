# stuttgart-things/blueprints/vm

## FUNCTIONS

<details><summary>RUN ANSIBLE</summary>

```bash
# JUST RUN ANSIBLE w/o src, inventory file or requirements file
dagger call -m vm execute-ansible \
--playbooks "sthings.baseos.setup" \
--hosts "10.31.103.58" \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--progress plain -vv
```

```bash
# MULTIPLE PLAYS
dagger call -m vm execute-ansible \
--playbooks "sthings.baseos.setup,sthings.container.kind_xplane" \
--hosts "10.31.103.27" \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements /tmp/requirements.yaml \
--progress plain -vv
```

```bash
# PROVIDE PLAY FROM SRC
dagger call -m vm execute-ansible \
--src "." \
--playbooks test-play.yaml \
--playbooks "sthings.baseos.setup,sthings.container.kind_xplane" \
--hosts "10.31.103.27" \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements /tmp/requirements.yaml \
--progress plain -vv
```


```bash
# PROVIDE PLAY FROM SRC + VARS FILE
cat <<'EOF' > ./vars.yaml
execute_baseos: false
install_ansible: true
install_binaries: false
EOF

cat <<'EOF' > ./test-play.yaml
---
- name: Base setup
  ansible.builtin.import_playbook: sthings.baseos.setup
  when: execute_baseos | default(true) | bool

- name: Install binaries
  ansible.builtin.import_playbook: sthings.baseos.binaries
  when: install_binaries | default(true) | bool

- name: Install ansible
  ansible.builtin.import_playbook: sthings.baseos.ansible
  when: install_ansible | default(true) | bool
EOF

dagger call -m vm execute-ansible \
--src "." \
--playbooks test-play.yaml \
--hosts "10.31.103.27" \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements /tmp/requirements.yaml \
--progress plain -vv
```

</details>

<details><summary>RUN ANSIBLE + EXPORT FILES</summary>

```bash
# Execute Ansible playbook and export files from the container
dagger call -m vm execute-ansible-with-export \
--playbooks "sthings.rke.k3s" \
--hosts 10.31.103.22 \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements ./requirements.yaml \
--parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml" \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml" \
--progress plain -vv \
export --path=/tmp/exported/
```

</details>

<details><summary>ENCRYPT FILE w/ SOPS</summary>

```bash
# Encrypt a plaintext file with SOPS using an AGE public key
dagger call -m vm encrypt-file \
--age-public-key=env:AGE_PUBLIC_KEY \
--plaintext-file /tmp/k3s.yaml \
--file-extension yaml
```

</details>

<details><summary>COMMIT TO GIT</summary>

```bash
# Commit a directory of files to a GitHub repository branch
dagger call -m vm commit-to-git \
--source-dir /tmp/encrypted/ \
--repository "stuttgart-things/k8s-configs" \
--branch-name main \
--commit-message "Add encrypted kubeconfig" \
--destination-path "clusters/k3s/" \
--git-token=env:GITHUB_TOKEN
```

```bash
# Commit to a new branch and open a PR
dagger call -m vm commit-to-git \
--source-dir /tmp/encrypted/ \
--repository "stuttgart-things/k8s-configs" \
--branch-name main \
--create-branch "feat/add-kubeconfig" \
--create-pr \
--pr-title "Add encrypted kubeconfig" \
--commit-message "Add encrypted kubeconfig" \
--destination-path "clusters/k3s/" \
--git-token=env:GITHUB_TOKEN
```

</details>

<details><summary>RUN ANSIBLE + ENCRYPT + COMMIT</summary>

```bash
# Full pipeline: execute Ansible, encrypt exported files, commit to Git
dagger call -m vm execute-ansible-encrypt-and-commit \
--playbooks "sthings.rke.k3s" \
--hosts 10.31.103.22 \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements ./requirements.yaml \
--parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml prepare_rancher_ha_nodes=true" \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml" \
--age-public-key=env:AGE_PUBLIC_KEY \
--git-repository "stuttgart-things/k8s-configs" \
--git-branch main \
--git-commit-message "Add encrypted kubeconfig for k3s cluster" \
--git-destination-path "clusters/k3s/" \
--git-token=env:GITHUB_TOKEN \
--progress plain -vv
```

```bash
# Full pipeline with branch creation + PR
dagger call -m vm execute-ansible-encrypt-and-commit \
--playbooks "sthings.rke.k3s" \
--hosts 10.31.103.22 \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements ./requirements.yaml \
--parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml prepare_rancher_ha_nodes=true" \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml" \
--age-public-key=env:AGE_PUBLIC_KEY \
--git-repository "stuttgart-things/k8s-configs" \
--git-branch main \
--git-create-branch "feat/add-k3s-kubeconfig" \
--git-create-pr \
--git-pr-title "Add encrypted kubeconfig for k3s cluster" \
--git-commit-message "Add encrypted kubeconfig for k3s cluster" \
--git-destination-path "clusters/k3s/" \
--git-token=env:GITHUB_TOKEN \
--progress plain -vv
```

</details>

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
# SOPS ENCRYPTED w/ AUTO SSH CREDS
# When the SOPS-encrypted tfvars file contains "vm_ssh_user" and
# "vm_ssh_password", Ansible SSH credentials are extracted automatically.
# No --ansible-user / --ansible-password flags needed.

dagger call -m vm bake-local \
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--encrypted-file /home/sthings/projects/stuttgart-things/terraform/secrets/labda-terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-requirements-file tests/vm/requirements.yaml \
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
# Bake + S3 STATE
dagger call -m vm bake \
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--encrypted-file /home/sthings/projects/stuttgart-things/terraform/secrets/labda-terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-user=env:SSH_USER \
--ansible-password=env:SSH_PASSWORD \
--ansible-parameters "send_to_homerun=false" \
--ansible-playbooks "sthings.baseos.setup" \
--awsAccessKeyID env:AWS_ACCESS_KEY_ID \
--awsSecretAccessKey env:AWS_SECRET_ACCESS_KEY \
-vv --progress plain \
export --path=~/projects/terraform/vms/sthings-runner/
```

</details>

<details><summary>BAKE LOCAL BY PROFILE</summary>

```bash
cat <<EOF >> vm.yaml
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
EOF
```

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

```bash
# SOPS ENCRYPTED w/ AUTO SSH CREDS
# If the profile references a SOPS-encrypted tfvars file that contains
# "vm_ssh_user" and "vm_ssh_password", --ansible-user / --ansible-password
# can be omitted — credentials are extracted from the decrypted content.

cat <<EOF >> vm-sops.yaml
---
operation: apply
ansiblePlaybooks:
  - "sthings.baseos.setup"
ansibleParameters: []
ansibleInventoryType: default
ansibleWaitTimeout: 30
ansibleRequirementsFile: ./requirements.yaml
encryptedFile: ./terraform.tfvars.enc.json
EOF
```

```bash
dagger call -m vm bake-local-by-profile \
--src ./ \
--profile vm-sops.yaml \
--sops-key env:SOPS_AGE_KEY \
--awsAccessKeyID env:AWS_ACCESS_KEY_ID \
--awsSecretAccessKey env:AWS_SECRET_ACCESS_KEY \
--progress plain -vv \
export --path ./
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
# EXAMPLE 1

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

```bash
# EXAMPLE 2

dagger call -m github.com/stuttgart-things/blueprints/vm@v1.34.0 execute-ansible \
--playbooks "sthings.rke.k3s" \
--hosts "192.168.1.40" \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--parameters="install_k3s=true k3s_state=present k3s_k8s_version=1.34.2 k3s_release_kind=k3s1 cluster_setup=singlenode install_cillium=true deploy_helm_charts=true install_helm_diff=true cilium_lbrange_start_ip=192.168.1.80 cilium_lbrange_stop_ip=192.168.1.80 ingress_service_type=ClusterIP" \
--requirements requirements.yaml \
--inventoryType="cluster" \
--progress plain -vv
```

```bash
# EXAMPLE 3 - PDNS w/ VAULT

dagger call -m vm \
execute-ansible \
--playbooks sthings.baseos.pdns \
--requirements requirements.yaml \
--parameters "ip_address=10.31.102.8 hostname=dev-infra-pre pdns_url=https://pdns-vsphere.labul.sva.de:8443 entry_zone=sthings-vsphere.labul.sva.de." \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--vault-url env:VAULT_ADDR \
--progress plain -vv
```

</details>

<details><summary>GET TSHIRT SIZE</summary>

```bash
dagger call -m vm tshirt-size \
--config-file=tests/vm/config/vm-tshirt-sizes.yaml \
--size=medium \
-vv --progress plain
```

</details>

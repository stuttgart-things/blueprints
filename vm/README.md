# stuttgart-things/blueprints/vm

## OUTPUTS

After a successful `apply` operation, `BakeLocal` / `BakeLocalByProfile` write the following files into the returned directory:

- `inventory.yaml` — generated Ansible inventory (default or cluster layout)
- `outputs.json` — flat JSON object with stage outputs for downstream consumers (e.g. chained Dapr workflows). Current shape:

  ```json
  { "vm_ips": ["10.x.x.x", "10.x.x.y"] }
  ```

  `vm_ips` is always a list (single-VM apply yields a one-element list). The file is omitted for non-`apply` operations.

`BakeHarvester` writes the same `outputs.json` (so downstream stages do not care which provisioner ran), plus:

- `harvester-vm.yaml` — the rendered PVC + cloud-init Secret + VirtualMachine, kept so a failed run can be inspected or re-applied by hand
- `inventory.ini` — the `[all]` inventory Ansible ran with

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
# Use --export-target-names to rename the file in the target repository
dagger call -m vm execute-ansible-encrypt-and-commit \
--playbooks "sthings.rke.k3s" \
--hosts 10.31.103.22 \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements ./requirements.yaml \
--parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml prepare_rancher_ha_nodes=true" \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml" \
--export-target-names "prod-kubeconfig.yaml" \
--age-public-key=env:AGE_PUBLIC_KEY \
--git-repository "stuttgart-things/k8s-configs" \
--git-branch main \
--git-commit-message "Add encrypted kubeconfig for k3s cluster" \
--git-destination-path "clusters/k3s/" \
--git-token=env:GITHUB_TOKEN \
--progress plain -vv
```

```bash
# Full pipeline with multiple exported files and custom target names
dagger call -m vm execute-ansible-encrypt-and-commit \
--playbooks "sthings.rke.k3s" \
--hosts 10.31.103.22 \
--ssh-user=env:SSH_USER \
--ssh-password=env:SSH_PASSWORD \
--requirements ./requirements.yaml \
--parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml prepare_rancher_ha_nodes=true" \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml,/tmp/cluster-token.txt" \
--export-target-names "prod-kubeconfig.yaml,prod-token.txt" \
--age-public-key=env:AGE_PUBLIC_KEY \
--git-repository "stuttgart-things/k8s-configs" \
--git-branch main \
--git-commit-message "Add encrypted kubeconfig and token for k3s cluster" \
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
--export-target-names "prod-kubeconfig.yaml" \
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

<details><summary>BAKE HARVESTER (VM bootstrap without a control plane)</summary>

The Crossplane-free counterpart to `BakeLocal`, and the reason it exists: the first VM on a
Harvester cluster has to be created before there is any control plane to create it with —
typically the VM that then runs the Crossplane management cluster provisioning everything after it.

Same shape as `BakeLocal` (provision, read the machine's address back, hand it to Ansible), but the
provisioning step is the Kubernetes API instead of OpenTofu:

1. render PVC + cloud-init Secret + KubeVirt VirtualMachine from the
   [`harvester-vm`](https://github.com/stuttgart-things/kcl/tree/main/kubernetes/harvester-vm) KCL module
2. `kubectl apply` them against Harvester
3. poll the VirtualMachineInstance until it is Running **and** the in-guest QEMU guest agent has reported an IP
4. run Ansible against that IP

```yaml
# params.yaml — KCL parameters for the harvester-vm module.
# vmName and namespace are NOT set here: BakeHarvester forces them from its own
# flags so the manifests and the VMI it polls for cannot drift apart.
enablePvc: true
enableCloudConfig: true
enableVm: true

# Root disk. pvcName is optional — it defaults to "<vm-name>-disk-0".
# NOTE: the module assembles the storage class as
# "<storageClass>-<imageId>", so pass "longhorn" — not "harvester-longhorn".
pvcName: bootstrap-disk-0
imageNamespace: default
imageId: image-t9w92
storage: 60Gi
storageClass: longhorn
volumeMode: Block
# ReadWriteOnce for a single-VM boot disk: RWX on a Block boot volume invites a
# second instance attaching the same disk.
accessModes: '["ReadWriteOnce"]'

# cloud-init. The module builds the cloud-config from these and always installs
# and starts qemu-guest-agent — which is what makes step 3 above terminate.
secretName: bootstrap-cloud-init # pragma: allowlist secret
cloudInitUsername: sthings
cloudInitPassword: <REPLACEME> # pragma: allowlist secret
cloudInitSshKey: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC...

# VM
cpuCores: 8
cpuSockets: 1
cpuThreads: 1
memory: 16Gi
networkNamespace: default
networkName: vms
runStrategy: RerunOnFailure
```

```bash
# FULL BOOTSTRAP: render -> apply -> wait for IP -> ansible
dagger call -m vm bake-harvester \
--kube-config file://~/.kube/harvester \
--vm-name bootstrap-xplane \
--namespace vms \
--kcl-parameters-file ./params.yaml \
--ansible-playbooks "sthings.baseos.setup,sthings.container.kind_xplane" \
--ansible-user env:ANSIBLE_USER \
--ansible-password env:ANSIBLE_PASSWORD \
--progress plain -vv \
export --path /tmp/bootstrap
```

```bash
# NO ANSIBLE: leave --ansible-playbooks off to stop once the VM has an IP
dagger call -m vm bake-harvester \
--kube-config file://~/.kube/harvester \
--vm-name bootstrap-xplane \
--namespace vms \
--kcl-parameters-file ./params.yaml \
--progress plain -vv \
export --path /tmp/bootstrap
```

```bash
# SOPS-ENCRYPTED PARAMETERS (decrypted in-memory; the plaintext never becomes
# an operation argument)
dagger call -m vm bake-harvester \
--kube-config file://~/.kube/harvester \
--vm-name bootstrap-xplane \
--namespace vms \
--encrypted-file ./params.enc.yaml \
--sops-key env:SOPS_AGE_KEY \
--ansible-playbooks "sthings.baseos.setup" \
--ansible-user env:ANSIBLE_USER \
--ansible-password env:ANSIBLE_PASSWORD \
--progress plain -vv
```

```bash
# DRY RUN: render the three manifests without touching a cluster.
# vmName is what the PVC and Secret names are derived from, so pass it here
# too — it is the --vm-name that bake-harvester would force in.
dagger call -m vm render-harvester-vm \
--kcl-parameters-file ./params.yaml \
--kcl-parameters "vmName=bootstrap-xplane,namespace=vms" \
--progress plain

# WAIT ONLY: for a VM created some other way
dagger call -m vm wait-for-vm-ip \
--kube-config file://~/.kube/harvester \
--vm-name bootstrap-xplane \
--namespace vms \
--progress plain
```

Worth knowing:

- **The IP comes from the guest agent.** `qemu-guest-agent` must be installed and running in the VM, or the wait runs its full `--wait-timeout` (default 900s) against a VM that is perfectly healthy. The module's generated cloud-config installs it; a hand-supplied `userdata` must do the same.
- **Ansible login.** The `sthings-*` golden images carry the user and password auth that `ANSIBLE_USER` / `ANSIBLE_PASSWORD` expect. A vanilla cloud image disables password auth and trusts only the cloud-init key, so Ansible cannot log in (`Permission denied (publickey)`).
- **Resource names default to the VM.** `pvcName` and `secretName` are derived as `<vm-name>-disk-0` and `<vm-name>-cloud-init` when they are set in neither `--kcl-parameters-file` nor `--kcl-parameters`. The KCL module's own fallbacks are the fixed strings `dev2-disk-0` and `dev4`, which two bootstrap runs in the same namespace would silently share — including the block boot disk. Set them explicitly to attach a restored disk. `render-harvester-vm` derives them the same way, so a dry run shows the names `bake-harvester` would apply — but it can only do so if the parameters name the VM, since it has no `--vm-name` of its own.
- **Keep credentials in the parameters file.** `--kcl-parameters-file` is mounted into the render container; `--kcl-parameters` values become operation arguments and are echoed verbatim by `dagger --progress plain`.
- **One-shot, not managed.** There is no reconcile loop and no drift correction — updating or deleting the VM afterwards is `kubectl`'s job (or Crossplane's, once the cluster this VM bootstraps is up). Note also that the KCL module omits KubeVirt's LiveUpdate hotplug fields (`cpu.maxSockets`, `cpu.model`, `memory.guest`, `memory.maxGuest`), which is harmless for a single apply but will show up as `RestartRequired` drift if the VM is later handed to the `harvester-vm` Crossplane Configuration.
- **The target namespace is created** (idempotently) unless `--skip-namespace` is passed, because `kubectl apply` does not create it and a missing namespace is the most common way a bootstrap run dies on line one.

</details>

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

```bash
# BAKE LOCAL + ANSIBLE EXPORT + SOPS ENCRYPT
# Runs Terraform + Ansible, exports files from the Ansible container,
# encrypts them with SOPS, and includes them in the result under encrypted-exports/
dagger call -m vm bake-local \
--terraform-dir ~/projects/terraform/vms/k3s-cluster/ \
--encrypted-file ./terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-playbooks "sthings.rke.k3s" \
--ansible-parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml" \
--ansible-requirements-file ./requirements.yaml \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml" \
--export-target-names "kubeconfig.yaml" \
--age-public-key=env:AGE_PUBLIC_KEY \
--sops-file-extension yaml \
-vv --progress plain \
export --path=./output/
```

```bash
# BAKE LOCAL + MULTIPLE EXPORTS + SOPS ENCRYPT
dagger call -m vm bake-local \
--terraform-dir ~/projects/terraform/vms/k3s-cluster/ \
--encrypted-file ./terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-playbooks "sthings.rke.k3s" \
--ansible-parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml" \
--ansible-requirements-file ./requirements.yaml \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml,/tmp/cluster-token.txt" \
--export-target-names "kubeconfig.yaml,cluster-token.txt" \
--age-public-key=env:AGE_PUBLIC_KEY \
--sops-config ./.sops.yaml \
-vv --progress plain \
export --path=./output/
```

```bash
# BAKE LOCAL + EXPORT + SOPS ENCRYPT AT ROOT LEVEL (no subdirectory)
# Use --export-destination-path="./" to place encrypted files at root
dagger call -m vm bake-local \
--terraform-dir ~/projects/terraform/vms/k3s-cluster/ \
--encrypted-file ./terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
--ansible-playbooks "sthings.rke.k3s" \
--ansible-parameters "k3s_k8s_version=1.35.1 k3s_release_kind=k3s1 cluster_setup=singlenode fetched_kubeconfig_path=/tmp/k3s.yaml" \
--ansible-requirements-file ./requirements.yaml \
--inventory-type cluster \
--export-paths "/tmp/k3s.yaml" \
--export-target-names "kubeconfig.yaml" \
--age-public-key=env:AGE_PUBLIC_KEY \
--export-destination-path "./" \
-vv --progress plain \
export --path=./output/
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

```bash
# PROFILE w/ ANSIBLE EXPORT + SOPS ENCRYPT
# Exported files are encrypted with SOPS and placed under encrypted-exports/

cat <<EOF >> vm-export.yaml
---
operation: apply
variables:
  - vault_addr=https://vault-vsphere.tiab.labda.sva.de:8200
ansiblePlaybooks:
  - "sthings.rke.k3s"
ansibleParameters:
  - "k3s_k8s_version=1.35.1"
  - "k3s_release_kind=k3s1"
  - "cluster_setup=singlenode"
  - "fetched_kubeconfig_path=/tmp/k3s.yaml"
ansibleInventoryType: default
ansibleWaitTimeout: 60
ansibleRequirementsFile: ./requirements.yaml
encryptedFile: ""
exportPaths:
  - "/tmp/k3s.yaml"
exportTargetNames:
  - "kubeconfig.yaml"
sopsFileExtension: yaml
EOF
```

```bash
dagger call -m vm bake-local-by-profile \
--src ./ \
--profile vm-export.yaml \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--ansible-user env:ANSIBLE_USER \
--ansible-password env:ANSIBLE_PASSWORD \
--age-public-key env:AGE_PUBLIC_KEY \
--progress plain -vv \
export --path ./
```

```bash
# PROFILE w/ MULTIPLE EXPORTS + SOPS CONFIG FILE

cat <<EOF >> vm-multi-export.yaml
---
operation: apply
ansiblePlaybooks:
  - "sthings.rke.k3s"
ansibleParameters:
  - "k3s_k8s_version=1.35.1"
  - "k3s_release_kind=k3s1"
  - "cluster_setup=singlenode"
  - "fetched_kubeconfig_path=/tmp/k3s.yaml"
ansibleInventoryType: default
ansibleWaitTimeout: 60
ansibleRequirementsFile: ./requirements.yaml
encryptedFile: ./terraform.tfvars.enc.json
exportPaths:
  - "/tmp/k3s.yaml"
  - "/tmp/cluster-token.txt"
exportTargetNames:
  - "kubeconfig.yaml"
  - "cluster-token.txt"
sopsFileExtension: yaml
exportDestinationPath: "./"  # place encrypted files at root (no subdirectory)
EOF
```

```bash
dagger call -m vm bake-local-by-profile \
--src ./ \
--profile vm-multi-export.yaml \
--sops-key env:SOPS_AGE_KEY \
--age-public-key env:AGE_PUBLIC_KEY \
--sops-config ./.sops.yaml \
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

> SOPS decrypt/encrypt now lives in the [`secrets`](../secrets/README.md) module:
>
> ```bash
> dagger call -m secrets decrypt \
>   --sops-key env:SOPS_AGE_KEY \
>   --encrypted-file tests/vm/terraform.tfvars.enc.json
> ```

</details>

<details><summary>EXECUTE TERRAFORM</summary>

`execute-terraform` is the canonical Terraform entry point — it absorbed the
rich `terraform-apply` previously hosted in the `configuration` module
(see #143). All SOPS, Vault, AWS-S3 backend, Kubernetes-state-backend and
`--kube-secret-*` flags are available here.

```bash
# APPLY (Vault credentials)
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

```bash
# APPLY with SOPS-decrypted tfvars + Kubernetes state backend
dagger call -m vm execute-terraform \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --progress plain -vv
```

```bash
# APPLY with AWS/S3 backend
dagger call -m vm execute-terraform \
  --terraform-dir /path/to/terraform/configs \
  --aws-access-key-id env:AWS_ACCESS_KEY_ID \
  --aws-secret-access-key env:AWS_SECRET_ACCESS_KEY \
  --progress plain -vv
```

```bash
# APPLY with Kubernetes-secret lookup (e.g. inject Vault root token as a tfvar)
dagger call -m vm execute-terraform \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-secret-name "vault-root-token" \
  --kube-secret-namespace "vault" \
  --kube-secret-jsonpath ".data.root_token" \
  --kube-secret-tf-var "vault_token" \
  --progress plain -vv
```

```bash
# APPLY then write terraform output --json into the result dir
dagger call -m vm execute-terraform \
  --terraform-dir /path/to/terraform/configs \
  --sops-age-key env:SOPS_AGE_KEY \
  --encrypted-files "terraform.tfvars.sops.json" \
  --kube-config file:///path/to/.kube/my-cluster \
  --export-tf-output \
  file --path output.json contents \
  --progress plain -vv
```

#### SOPS env-var handling

SOPS-encrypted JSON files containing `VAULT_TOKEN`, `VAULT_ADDR`, or
`VAULT_SKIP_VERIFY` are extracted as container environment variables instead
of being written as terraform variable files. Remaining keys are written to
`terraform.tfvars.json` as usual.

</details>

<details><summary>OUTPUT TERRAFORM RUN</summary>

```bash
# Local / file backend
dagger call -m vm \
output-terraform-run \
--terraform-dir=~/tmp/dagger/tests/terraform/ \
--progress plain -vv
```

```bash
# AWS/S3 backend
dagger call -m vm output-terraform-run \
  --terraform-dir /path/to/terraform/state-dir \
  --aws-access-key-id env:AWS_ACCESS_KEY_ID \
  --aws-secret-access-key env:AWS_SECRET_ACCESS_KEY \
  --progress plain -vv
```

```bash
# Kubernetes state backend
dagger call -m vm output-terraform-run \
  --terraform-dir /path/to/terraform/state-dir \
  --kube-config file:///path/to/.kube/my-cluster \
  --kube-config-path "/path/to/.kube/my-cluster" \
  --progress plain -vv
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

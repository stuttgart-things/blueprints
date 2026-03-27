# stuttgart-things/blueprints/vmtemplate

## ARCHITECTURE

```
packer/templates/                        # shared templates (reusable across builds)
├── ubuntu/                              # Ubuntu OS family
│   ├── vsphere-base-os.pkr.hcl.tmpl      # packer config (autoinstall, apt)
│   ├── user-data.tmpl                     # cloud-init autoinstall template
│   └── requirements.yaml.tmpl            # ansible requirements template
├── rocky/                               # Rocky/Fedora OS family
│   ├── vsphere-base-os.pkr.hcl.tmpl      # packer config (kickstart, dnf)
│   └── user-data.tmpl                     # kickstart template
└── test-vm/                             # test VM (shared across OS families)
    ├── test-vm.tf.tmpl                    # test VM terraform template
    └── state.tf.tmpl                      # terraform state backend template

packer/environments/                       # shared environment configs
├── labul-vsphere-v2.yaml                    # LabUL infra (datacenter, network, vault, test-vm)
└── labda-vsphere-v2.yaml                    # LabDA infra

packer/builds/<os>-<env>-vsphere-base-os/   # per-build config
├── build-vars.yaml                          # OS-specific values (iso, ssh, collections)
├── base-os.yaml                             # ansible playbook (static, keeps Jinja2 syntax)
└── meta-data                                # cloud-init meta-data (static)
```

**Variable priority (last wins):**
1. `environments/*.yaml` — infra layer (datacenter, network, vault, s3)
2. `build-vars.yaml` — OS layer (iso, ssh user, collections)
3. `--overrides "key=val"` — CLI overrides (highest priority)

**Workflow:**
1. `RenderBuildConfig` merges variable files and renders all templates
2. `Bake` runs packer build with rendered config
3. (optional) Test VM validation via `vm` module
4. (optional) Golden image promotion via govc

## WORKFLOWS

<details><summary>BAKE ONLY (RENDER + BUILD)</summary>

```bash
export VAULT_TOKEN=<REPLACEME>
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>

dagger call -m /home/sthings/projects/blueprints/vmtemplate \
run-vsphere-workflow \
--packer-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/ubuntu \
--packer-templates "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl,requirements.yaml.tmpl" \
--build-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-base-os \
--env-dir /home/sthings/projects/stuttgart-things/packer/environments \
--variables-files "labul-vsphere-v2.yaml,build-vars.yaml" \
--packer-config vsphere-base-os.pkr.hcl \
--packer-version 1.13.1 \
--vault-addr $(echo $VAULT_ADDR) \
--vault-token env:VAULT_TOKEN \
--vault-role-id env:VAULT_ROLE_ID \
--vault-secret-id env:VAULT_SECRET_ID \
--progress plain -vv -vv 2>&1 |tee /tmp/packer-log.txt
```

</details>

<details><summary>BAKE + TEST VM VALIDATION</summary>

```bash
export VAULT_TOKEN=<REPLACEME>
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>

dagger call -m /home/sthings/projects/blueprints/vmtemplate \
run-vsphere-workflow \
--packer-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/ubuntu \
--packer-templates "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl,requirements.yaml.tmpl" \
--test-vm-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/test-vm \
--test-vm-templates "test-vm.tf.tmpl,state.tf.tmpl" \
--build-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-base-os \
--env-dir /home/sthings/projects/stuttgart-things/packer/environments \
--variables-files "labul-vsphere-v2.yaml,build-vars.yaml" \
--packer-config vsphere-base-os.pkr.hcl \
--packer-version 1.13.1 \
--vault-addr $(echo $VAULT_ADDR) \
--vault-token env:VAULT_TOKEN \
--vault-role-id env:VAULT_ROLE_ID \
--vault-secret-id env:VAULT_SECRET_ID \
--test-vm \
--test-playbooks test-baseos.yaml \
--ssh-user env:SSH_USER \
--ssh-password env:SSH_PASSWORD \
--ansible-wait-timeout 60 \
--progress plain -vv -vv 2>&1 |tee /tmp/packer-log.txt
```

</details>

<details><summary>FULL WORKFLOW (RENDER + BAKE + TEST + PROMOTE)</summary>

```bash
export VAULT_TOKEN=<REPLACEME>
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>
export VCENTER_URL=<REPLACEME>
export VCENTER_USERNAME=<REPLACEME>
export VCENTER_PASSWORD=<REPLACEME>

dagger call -m /home/sthings/projects/blueprints/vmtemplate \
run-vsphere-workflow \
--packer-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/ubuntu \
--packer-templates "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl,requirements.yaml.tmpl" \
--test-vm-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/test-vm \
--test-vm-templates "test-vm.tf.tmpl,state.tf.tmpl" \
--build-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-base-os \
--env-dir /home/sthings/projects/stuttgart-things/packer/environments \
--variables-files "labul-vsphere-v2.yaml,build-vars.yaml" \
--packer-config vsphere-base-os.pkr.hcl \
--packer-version 1.13.1 \
--vault-addr $(echo $VAULT_ADDR) \
--vault-token env:VAULT_TOKEN \
--vault-role-id env:VAULT_ROLE_ID \
--vault-secret-id env:VAULT_SECRET_ID \
--test-vm \
--test-playbooks test-baseos.yaml \
--ssh-user env:SSH_USER \
--ssh-password env:SSH_PASSWORD \
--ansible-wait-timeout 60 \
--promote-template \
--golden-template-name ubuntu25-base \
--golden-template-folder "/LabUL/vm/golden" \
--vcenter env:VCENTER_URL \
--vcenter-username env:VCENTER_USERNAME \
--vcenter-password env:VCENTER_PASSWORD \
--progress plain -vv -vv 2>&1 |tee /tmp/packer-log.txt
```

</details>

<details><summary>RENDER ONLY (DRY RUN)</summary>

```bash
dagger call -m /home/sthings/projects/blueprints/vmtemplate \
render-build-config \
--templates-dir /home/sthings/projects/stuttgart-things/packer/templates/ubuntu \
--templates "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl,requirements.yaml.tmpl" \
--build-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-base-os \
--env-dir /home/sthings/projects/stuttgart-things/packer/environments \
--variables-files "labul-vsphere-v2.yaml,build-vars.yaml" \
--overrides "cpus=16,ram=32768" \
export --path /tmp/rendered-packer/
```

</details>

<details><summary>RENDER AND COMMIT TO GIT</summary>

```bash
dagger call -m /home/sthings/projects/blueprints/vmtemplate \
render-and-commit \
--packer-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/ubuntu \
--packer-templates "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl,requirements.yaml.tmpl" \
--test-vm-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/test-vm \
--test-vm-templates "test-vm.tf.tmpl,state.tf.tmpl" \
--build-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-base-os \
--env-dir /home/sthings/projects/stuttgart-things/packer/environments \
--variables-files "labul-vsphere-v2.yaml,build-vars.yaml" \
--repository stuttgart-things/stuttgart-things \
--token env:GITHUB_TOKEN \
--create-branch \
--branch-name "feat/rendered-ubuntu25-labul-config" \
--commit-config \
--packer-destination-path "packer/builds/ubuntu25-labul-vsphere-base-os" \
--test-vm-destination-path "packer/builds/ubuntu25-labul-vsphere-base-os/test-vm" \
--create-pull-request \
--progress plain -vv
```

</details>

<details><summary>RENDER ONLY (NO GIT)</summary>

```bash
dagger call -m /home/sthings/projects/blueprints/vmtemplate \
render-and-commit \
--packer-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/ubuntu \
--packer-templates "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl,requirements.yaml.tmpl" \
--test-vm-templates-dir /home/sthings/projects/stuttgart-things/packer/templates/test-vm \
--test-vm-templates "test-vm.tf.tmpl,state.tf.tmpl" \
--build-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-base-os \
--env-dir /home/sthings/projects/stuttgart-things/packer/environments \
--variables-files "labul-vsphere-v2.yaml,build-vars.yaml" \
export --path /tmp/rendered/
```

</details>

<details><summary>RUN PACKER TEST CODE (HELLO WORLD)</summary>

```bash
dagger call -m vmtemplate \
bake \
--packer-config-dir tests/vmtemplate/hello \
--packer-config hello.pkr.hcl \
--packer-version 1.13.1 \
--progress plain -vv
```

</details>

## PARAMETERS

### render-build-config

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--templates-dir` | `Directory` | yes | - | Directory containing template files (.tmpl) |
| `--templates` | `string` | yes | - | Comma-separated list of template files to render |
| `--build-dir` | `Directory` | yes | - | Directory containing build-specific variables and static files |
| `--variables-files` | `string` | yes | - | Comma-separated YAML files to merge (last wins) |
| `--env-dir` | `Directory` | no | - | Additional directory with shared variable files |
| `--overrides` | `string` | no | - | Comma-separated `key=value` overrides (highest priority) |

### render-and-commit

#### Rendering

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--packer-templates-dir` | `Directory` | yes | - | Directory containing packer template files |
| `--packer-templates` | `string` | yes | - | Comma-separated packer templates to render |
| `--test-vm-templates-dir` | `Directory` | no | - | Directory containing test VM template files |
| `--test-vm-templates` | `string` | no | - | Comma-separated test VM templates to render |
| `--build-dir` | `Directory` | yes | - | Directory with build-specific vars and static files |
| `--env-dir` | `Directory` | no | - | Additional directory with shared variable files |
| `--variables-files` | `string` | yes | - | Comma-separated YAML files to merge (last wins) |
| `--overrides` | `string` | no | - | Comma-separated `key=value` overrides (highest priority) |

#### Git Operations (all optional)

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--repository` | `string` | no | - | GitHub repository (e.g., `stuttgart-things/stuttgart-things`) |
| `--token` | `Secret` | no | - | GitHub authentication token |
| `--branch-name` | `string` | no | `rendered-packer-config` | Branch name |
| `--base-branch` | `string` | no | `main` | Base branch to create from |
| `--create-branch` | `bool` | no | `false` | Create a new branch |
| `--commit-config` | `bool` | no | `false` | Commit rendered files |
| `--packer-destination-path` | `string` | no | - | Destination path for packer files in repo |
| `--test-vm-destination-path` | `string` | no | - | Destination path for test VM files in repo |
| `--create-pull-request` | `bool` | no | `false` | Create a PR after committing |
| `--pull-request-title` | `string` | no | auto-generated | PR title |
| `--pull-request-body` | `string` | no | auto-generated | PR body |
| `--commit-message` | `string` | no | auto-generated | Commit message |

### run-vsphere-workflow

#### Rendering

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--packer-templates-dir` | `Directory` | yes | - | Directory containing packer template files |
| `--packer-templates` | `string` | yes | - | Comma-separated packer templates to render |
| `--test-vm-templates-dir` | `Directory` | no | - | Directory containing test VM template files |
| `--test-vm-templates` | `string` | no | - | Comma-separated test VM templates to render |
| `--build-dir` | `Directory` | yes | - | Directory with build-specific vars and static files |
| `--env-dir` | `Directory` | no | - | Additional directory with shared variable files |
| `--variables-files` | `string` | yes | - | Comma-separated YAML files to merge (last wins) |
| `--overrides` | `string` | no | - | Comma-separated `key=value` overrides (highest priority) |

#### Packer Build

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--packer-config` | `string` | yes | - | Packer configuration file name (after rendering) |
| `--packer-version` | `string` | no | `1.13.1` | Packer version |
| `--arch` | `string` | no | `linux_amd64` | Packer arch |
| `--init-only` | `bool` | no | `false` | Only init packer without build |
| `--vault-addr` | `string` | no | - | Vault address |
| `--vault-role-id` | `Secret` | no | - | Vault AppRole role ID |
| `--vault-secret-id` | `Secret` | no | - | Vault AppRole secret ID |
| `--vault-token` | `Secret` | no | - | Vault token |

#### Test VM Validation (optional)

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--test-vm` | `bool` | no | `false` | Enable test VM creation and validation |
| `--test-playbooks` | `string` | no | - | Comma-separated Ansible playbook paths |
| `--test-requirements` | `File` | no | - | Ansible requirements file for test playbooks |
| `--ansible-wait-timeout` | `int` | no | `30` | Seconds to wait before running Ansible |
| `--ansible-inventory-type` | `string` | no | `simple` | Ansible inventory type (`simple` or `cluster`) |
| `--ansible-parameters` | `string` | no | - | Ansible parameters (e.g., `key1=value1,key2=value2`) |
| `--ssh-user` | `Secret` | no | - | SSH user for test VM |
| `--ssh-password` | `Secret` | no | - | SSH password for test VM |

#### Golden Image Promotion (optional)

| Parameter | Type | Required | Default | Description |
|---|---|---|---|---|
| `--promote-template` | `bool` | no | `false` | Enable golden image promotion |
| `--golden-template-name` | `string` | no | - | Target name for the golden template |
| `--golden-template-folder` | `string` | no | - | vCenter folder to move the golden template to |
| `--vcenter` | `Secret` | no | - | vCenter URL for govc operations |
| `--vcenter-username` | `Secret` | no | - | vCenter username |
| `--vcenter-password` | `Secret` | no | - | vCenter password |

## ADDING A NEW BUILD

To add a new OS/environment build:

1. Create `packer/builds/<name>/build-vars.yaml` with OS-specific values (iso, ssh, collections)
2. Add static files: `base-os.yaml`, `meta-data`
3. Point at the matching environment file (`environments/labul-vsphere-v2.yaml`) and OS template dir (`templates/ubuntu/`)

The shared templates and environment files stay the same — only `build-vars.yaml` changes per build.

## TESTING

Run the render test locally (no secrets required):

```bash
cd /home/sthings/projects/blueprints
task test-vmtemplate-render
```

This renders test templates using layered variables (env + build + overrides) and exports to `/tmp/vmtemplate-render-test/`.

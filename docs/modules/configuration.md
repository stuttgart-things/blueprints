# Configuration Module

Render Meta/Docs, Flux-Kustomizations, vSphere-Vars and Ansible-Requirements.

## Features

- Render Flux Kustomizations from OCI sources
- Create vSphere VM configurations with optional Git integration
- Generate Ansible requirements files from templates
- Render metadata and README files from templates

## Usage

### Render Flux Kustomization

Basic rendering:

```bash
dagger call -m configuration render-flux-kustomization \
  --oci-source ghcr.io/stuttgart-things/kcl-flux-instance \
  -vv --progress plain
```

With Git integration (create branch and commit):

```bash
dagger call -m configuration render-flux-kustomization \
  --oci-source ghcr.io/stuttgart-things/kcl-flux-instance \
  --token=env:GITHUB_TOKEN \
  --branch-name=test \
  --create-branch=true \
  --commit-changes=true \
  --file-name=flux-instance \
  --repository stuttgart-things/dagger \
  -vv --progress plain
```

### Create vSphere VM Configuration

Full example with all parameters:

```bash
dagger call -m configuration vsphere-vm \
  --src ./ \
  --config-parameters "name=demo-infra1,count=4,ram=8192,template=sthings-u24,disk=64,cpu=8,firmware=bios,vm_folder=stuttgart-things/testing,datacenter=/LabUL,datastore=/LabUL/datastore/UL-ESX-SAS-02,resourcePool=/LabUL/host/Cluster-V6.5/Resources,network=/LabUL/network/LAB-10.31.103,useVault=false,vaultSecretPath=vsphere-labul" \
  --token=env:GITHUB_TOKEN \
  --repository "stuttgart-things/blueprints" \
  --branch-name test \
  --commit-message "Add vSphere VM configuration" \
  --destination-folder "demo-infra1-LabUL" \
  --create-branch=true \
  --commit-config=true \
  --create-pull-request=true \
  export --path=./demo-infra1
```

Minimal local example:

```bash
dagger call -m configuration vsphere-vm \
  --src ./ \
  --config-parameters "name=demo3,count=4,ram=8192,template=sthings-u24,disk=64,cpu=8,firmware=bios,vm_folder=stuttgart-things/testing,datacenter=/LabUL,datastore=/LabUL/datastore/UL-ESX-SAS-02,resourcePool=/LabUL/host/Cluster-V6.5/Resources,network=/LabUL/network/LAB-10.31.103,useVault=false,vaultSecretPath=vsphere-labul" \
  --create-branch=false \
  --commit-config=false \
  --create-pull-request=false \
  export --path=/tmp/demo3
```

### Create Ansible Requirements

```bash
dagger call -m configuration create-ansible-requirement-files \
  --template-paths https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl \
  --data-file https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml \
  export --path /tmp/ansible-output \
  -vv --progress plain
```

### Render Metadata

Render README from template:

```bash
dagger call -m configuration render-metadata \
  --src ./tests/configuration \
  --template-path README.md.tmpl \
  --data-files vm-ansible.yaml,other-vars.yaml \
  export --path /tmp/readme-output \
  -vv --progress plain
```

Render execution file:

```bash
dagger call -m configuration render-metadata \
  --src tests/vm \
  --template-path osaka-profile.yaml.tmpl \
  --data-files osaka-profile-vars.yaml \
  export --path /tmp/execution-output \
  -vv --progress plain
```

### Render VM README

```bash
dagger call -m configuration render-vm-readme \
  --src ./tests/configuration \
  --template-path README.md.tmpl \
  --data-files vm-ansible.yaml,other-vars.yaml \
  --config-parameters="vm=demo3,profile=baseos" \
  export --path /tmp/demo3 \
  -vv --progress plain
```

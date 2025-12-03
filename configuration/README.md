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

# stuttgart-things/blueprints/configuration

<details><summary>CREATE VSPHERE CONFIG</summary>

```bash
# Example with all parameters specified
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

# Minimal example (auto-generates branch name, commit message, destination folder, PR title/body)
dagger call -m configuration vsphere-vm \
--src ./ \
--config-parameters "name=demo-infra1,count=4,ram=8192,template=sthings-u24,disk=64,cpu=8,firmware=bios,vm_folder=stuttgart-things/testing,datacenter=/LabUL,datastore=/LabUL/datastore/UL-ESX-SAS-02,resourcePool=/LabUL/host/Cluster-V6.5/Resources,network=/LabUL/network/LAB-10.31.103,useVault=false,vaultSecretPath=vsphere-labul" \
--token=env:GITHUB_TOKEN \
--repository "stuttgart-things/blueprints" \
--create-branch=true \
--commit-config=true \
--create-pull-request=true \
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

<details><summary>RENDER VM-README</summary>

```bash
dagger call -m configuration render-vm-readme \
--src ./tests/configuration \
--template-path README.md.tmpl \
--data-files vm-ansible.yaml,other-vars.yaml \
export --path /tmp/readme-output \
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

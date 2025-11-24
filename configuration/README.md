# stuttgart-things/blueprints/configuration

<details><summary>GET TSHIRT SIZE</summary>

```bash
dagger call -m configuration vsphere-vm \
--src "./" \
--template-paths tests/configuration/vm.tf.tpl \ --config-parameters "name=bla,count=2" \
export --path=/tmp/vm/
```

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

<details><summary>GET TSHIRT SIZE</summary>

```bash
dagger call -m vm tshirt-size \
--config-file=tests/vm/config/vm-tshirt-sizes.yaml \
--size=medium \
-vv --progress plain
```

</details>

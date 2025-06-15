# stuttgart-things/blueprints/vm

<details><summary>BUILD NEW VM</summary>

```bash
dagger call -m vm bake \
--terraform-dir tests/vm/tf \
--encrypted-file tests/vm/terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
-vv --progress plain \
export --path=~/projects/terraform/vms/dagger/
```

</details>

<details><summary>APPLY OVER EXISTING WORKSPACE/DIR</summary>

```bash
dagger call -m vm bake \
--terraform-dir ~/projects/terraform/vms/sthings-runner/ \
--encrypted-file /home/sthings/projects/stuttgart-things/terraform/secrets/ labda-terraform.tfvars.enc.json \
--operation apply \
--sops-key=env:SOPS_AGE_KEY \
-vv --progress plain \
export --path=~/projects/terraform/vms/sthings-runner/
```

</details>
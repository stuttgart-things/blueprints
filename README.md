# stuttgart-things/blueprints

collection of modular, reusable Dagger pipelines for automating build, test &amp; deployment workflows

```bash

dagger call -m vm bake --terraform-dir /home/sthings/projects/dagger/tests/terraform --operation apply --sops-key=env:SOPS_AGE_KEY --encrypted-file /home/sthings/projects/stuttgart-things/terraform/builds/labda-dagger-vm/terraform.tfvars.enc.json -vv --progress plain export --path=~/projects/terraform/vms/dagger/


```
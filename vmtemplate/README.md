# stuttgart-things/blueprints/vmtemplate

## WORKFLOWS

<details><summary>EXAMPLE VSPHERE WORKFLOW (GIT-SOURCE)</summary>

```bash
export VAULT_TOKEN=<REPLACEME>
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>

dagger call -m vmtemplate run-vsphere-workflow \
--git-repository ~/projects/stuttgart-things/stuttgart-things \
--git-ref main \
--git-token env:GITHUB_TOKEN \
--git-workdir packer/builds/ubuntu24-labda-vsphere \
--packer-config ubuntu24-base-os.pkr.hcl \
--packer-version 1.13.1 \
--vault-addr https://vault-vsphere.tiab.labda.sva.de:8200 \
--vault-token env:VAULT_TOKEN \
--vault-role-id env:VAULT_ROLE_ID \
--vault-secret-id env:VAULT_SECRET_ID \
--progress plain -vv -vv 2>&1 |tee /tmp/packer-log-local.txt
```

</details>

<details><summary>EXAMPLE VSPHERE WORKFLOW (GIT)</summary>

```bash
export VAULT_TOKEN=<REPLACEME>
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>

dagger call -m vmtemplate \
run-vsphere-workflow \
--git-repository ~/projects/stuttgart-things/stuttgart-things \
--git-ref main \
--git-token env:GITHUB_TOKEN \
--git-workdir packer/builds/ubuntu24-labda-vsphere \
--packer-config ubuntu24-base-os.pkr.hcl \
--packer-version 1.13.1 \
--vault-addr https://vault-vsphere.tiab.labda.sva.de:8200 \
--vault-token env:VAULT_TOKEN \
--vault-role-id env:VAULT_ROLE_ID \
--vault-secret-id env:VAULT_SECRET_ID \
--progress plain -vv -vv 2>&1 |tee /tmp/packer-log-local.txt
```

</details>

## MODULES

<details><summary>EXAMPLE VSPHERE WORKFLOW (LOCAL)</summary>

```bash
export VAULT_TOKEN=<REPLACEME>
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>

# # github.com/stuttgart-things/blueprints/go-microservice@v1.11.1

dagger call -m /home/sthings/projects/blueprints/go-microservice \
run-vsphere-workflow \
--packer-config-dir /home/sthings/projects/stuttgart-things/packer/builds/ubuntu25-labul-vsphere-baseos \
--packer-config ubuntu25-base-os.pkr.hcl \
--packer-version 1.13.1 \
--vault-addr $(echo $VAULT_ADDR) \
--vault-token env:VAULT_TOKEN \
--vault-role-id env:VAULT_ROLE_ID \
--vault-secret-id env:VAULT_SECRET_ID \
--progress plain -vv -vv 2>&1 |tee /tmp/packer-log-local.txt
```

</details>

<details><summary>RUN PACKER TEST CODE (TEMPLATE)</summary>

```bash
dagger call -m vmtemplate \
bake \
--packer-config-dir tests/vmtemplate/hello \
--packer-config hello.pkr.hcl \
--packer-version 1.13.1 \
--progress plain -vv
```

</details>

<details><summary>RUN TERRAFOMR TEST CODE (VM)</summary>

```bash
export VAULT_ROLE_ID=<REPLACEME>
export VAULT_SECRET_ID=<REPLACEME>

dagger call -m vmtemplate \
create-test-vm \
--terraform-dir tests/vmtemplate/tfvaulttest \
--vault-secret-id env:VAULT_SECRET_ID \
--vault-role-id env:VAULT_ROLE_ID \
--variables "vault_addr=https://vault-example.com:8200" \
--operation apply \
-vv --progress plain \
export --path=~/tmp/dagger/tests/terraform/
```

</details>

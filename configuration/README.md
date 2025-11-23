# stuttgart-things/blueprints/configuration

<details><summary>GET TSHIRT SIZE</summary>

```bash
dagger call -m configuration vsphere-vm \
--src "./" \
--template-paths tests/configuration/vm.tf.tpl \ --config-parameters "name=bla,count=2" \
export --path=/tmp/vm/
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

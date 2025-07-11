

```bash
dagger call -m kubernetes-microservice \
bake-image \
--src tests/k8s-microservice \
--repository-name stuttgart-things/test \
--registry-url ttl.sh \
--tag 1.2.3 \
-vv --progress plain
```
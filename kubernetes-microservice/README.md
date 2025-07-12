# stuttgart-things/blueprints/kubernetes-microservice

```bash
dagger call -m kubernetes-microservice \
bake-image \
--src tests/k8s-microservice \
--repository-name stuttgart-things/test \
--registry-url ttl.sh \
--tag 1.2.3 \
-vv --progress plain
```

```bash
dagger call -m kubernetes-microservice stage-image \
--target-username robot$sthings+backstage \
--target-password env:REG_PASSWORD \
--source redis:latest \
--target registry.example.com/sthings/redis:1.2.3 \ --target-registry registry.example.com \
--insecure=true \
--platform linux/amd64 \
-vv --progress plain
```

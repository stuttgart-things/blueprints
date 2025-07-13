# stuttgart-things/blueprints/kubernetes-microservice

```bash
# BAKE IMAGE w/o AUTH
dagger call -m kubernetes-microservice \
bake-image \
--src tests/kubernetes-microservice \
--repository-name stuttgart-things/test \
--registry-url ttl.sh \
--tag 1.2.3 \
-vv --progress plain
```

```bash
# BAKE IMAGE w/o AUTH + CONTEXT DIR .
dagger call -m kubernetes-microservice bake-image \
--src .devcontainer \
--repository-name stuttgart-things/backstage-dev \
--registry-url ttl.sh \
--tag 1.2.3 \
--with-directories . \
-vv --progress plain
```

```bash
# STAGE IMAGE TO REGISTRY
dagger call -m kubernetes-microservice stage-image \
--target-username robot$sthings+backstage \
--target-password env:REG_PASSWORD \
--source redis:latest \
--target registry.example.com/sthings/redis:1.2.3 \
--target-registry registry.example.com \
--insecure=true \
--platform linux/amd64 \
-vv --progress plain
```

```bash
# STAGE IMAGE w/o AUTH
dagger call -m kubernetes-microservice \
scan-image \
--imageRef nginx \
-vv --progress plain
```

```bash
# LINT DOCKERFILE
dagger call -m kubernetes-microservice \
lint-dockerfile \
--src . \
--dockerfile tests/kubernetes-microservice/Dockerfile \
-vv --progress plain
```

```bash
# RUN STATIC WORKFLOW STAGE
dagger call -m kubernetes-microservice \
run-static-stage \
--src . \
--path-to-dockerfile tests/kubernetes-microservice \
--progress plain -vv \
export --path=/tmp/static.json
```

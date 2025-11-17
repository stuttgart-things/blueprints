# stuttgart-things/blueprints/kubernetes-microservice

```bash
# TALKS TO KUBERNETES
dagger call -m kubernetes-microservice config \
--prompt-scope "show me all ingress resources in the entire cluster" \
--kube-config file://~/.kube/demo-infra \
--model="gemini-2.5-flash" \
--progress plain
```

```bash
# TALKS TO KUBERNETES
dagger call -m kubernetes-microservice config \
--prompt-scope "in the namespace kube-system and in the namespace crossplane-system - how many pods are running in each namespace?" \
--kube-config file://~/.kube/demo-infra \
--model="gemini-2.5-flash" \
--progress plain \
export --path=/tmp/cluster-pods.txt
```

```bash
# RENDERS HELMFILE
dagger call -m kubernetes-microservice deploy-helmfile \
--operation template \
--src ../dagger/tests/helm/ \
--progress plain
```

```bash
# READS GIVEN HELMFILE; TALKS TO CLUSTER; GIVES POSSIBLE VALUES LIKE STORAGECLASS OR INGRESS-DOMAIN
dagger call -m kubernetes-microservice analyze-helmfile \
--src ../dagger/tests/helm/ \
--kube-config file://~/.kube/demo-infra  \
--model="gemini-2.5-flash" \
--progress plain \
export --path=/tmp/argocd.txt
```

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

# Kubernetes Microservice Module

Build, stage, and scan container images. Lint Dockerfiles, run static analysis, and perform AI-powered Helmfile analysis.

## Features

- Build container images from Dockerfiles
- Stage images between registries
- Scan images for vulnerabilities
- Lint Dockerfiles
- AI-powered Kubernetes cluster queries
- AI-powered Helmfile analysis

## Usage

### AI Kubernetes Queries

Query cluster resources using AI:

```bash
dagger call -m kubernetes-microservice config \
  --prompt-scope "show me all ingress resources in the entire cluster" \
  --kube-config file://~/.kube/demo-infra \
  --model="gemini-2.5-flash" \
  --progress plain
```

Query pod counts across namespaces:

```bash
dagger call -m kubernetes-microservice config \
  --prompt-scope "in the namespace kube-system and in the namespace crossplane-system - how many pods are running in each namespace?" \
  --kube-config file://~/.kube/demo-infra \
  --model="gemini-2.5-flash" \
  --progress plain \
  export --path=/tmp/cluster-pods.txt
```

### Analyze Helmfile

AI-powered Helmfile analysis with cluster context:

```bash
dagger call -m kubernetes-microservice analyze-helmfile \
  --src ../dagger/tests/helm/ \
  --kube-config file://~/.kube/demo-infra \
  --model="gemini-2.5-flash" \
  --progress plain \
  export --path=/tmp/argocd.txt
```

### Build Image

Build image without authentication:

```bash
dagger call -m kubernetes-microservice bake-image \
  --src tests/kubernetes-microservice \
  --repository-name stuttgart-things/test \
  --registry-url ttl.sh \
  --tag 1.2.3 \
  -vv --progress plain
```

Build with additional context directories:

```bash
dagger call -m kubernetes-microservice bake-image \
  --src .devcontainer \
  --repository-name stuttgart-things/backstage-dev \
  --registry-url ttl.sh \
  --tag 1.2.3 \
  --with-directories . \
  -vv --progress plain
```

### Stage Image

Stage image to another registry:

```bash
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

### Scan Image

Scan image for vulnerabilities:

```bash
dagger call -m kubernetes-microservice scan-image \
  --imageRef nginx \
  -vv --progress plain
```

### Lint Dockerfile

```bash
dagger call -m kubernetes-microservice lint-dockerfile \
  --src . \
  --dockerfile tests/kubernetes-microservice/Dockerfile \
  -vv --progress plain
```

### Run Static Stage

Complete static analysis workflow:

```bash
dagger call -m kubernetes-microservice run-static-stage \
  --src . \
  --path-to-dockerfile tests/kubernetes-microservice \
  --progress plain -vv \
  export --path=/tmp/static.json
```

## Parameters

### bake-image

| Parameter | Description |
|-----------|-------------|
| `--src` | Source directory with Dockerfile |
| `--repository-name` | Image repository name |
| `--registry-url` | Container registry URL |
| `--tag` | Image tag |
| `--with-directories` | Additional context directories |

### stage-image

| Parameter | Description |
|-----------|-------------|
| `--source` | Source image reference |
| `--target` | Target image reference |
| `--target-registry` | Target registry URL |
| `--target-username` | Registry username |
| `--target-password` | Registry password |
| `--insecure` | Allow insecure registries |
| `--platform` | Target platform |

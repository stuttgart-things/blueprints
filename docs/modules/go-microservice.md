# Go Microservice Module

Lint, test, security scan and build (ldflags/ko) for Go services.

## Features

- Static code analysis with linting
- Security scanning
- Unit testing
- Binary building with ldflags support
- Container image building with ko

## Usage

### Static Stage

Run all static checks:

```bash
dagger call -m go-microservice run-static-stage \
  --src tests/go-microservice/ldflags/ \
  --progress plain -vv \
  export --path=/tmp/report.json
```

Skip tests and security scan:

```bash
dagger call -m go-microservice run-static-stage \
  --src tests/go-microservice/ldflags/ \
  --lintCanFail=true \
  --security-scan=false \
  --test=false \
  --progress plain -vv \
  export --path=/tmp/report.json
```

Check the report:

```bash
cat /tmp/report.json
```

### Build Stage

Build binary only (no ko build):

```bash
dagger call -m go-microservice run-build-stage \
  --src tests/go-microservice/ldflags/ \
  --ko-build=false \
  --bin-name demo \
  --progress plain -vv \
  export --path=/tmp/microservices/ldflags-test/
```

Build binary with ldflags:

```bash
dagger call -m go-microservice run-build-stage \
  --src tests/go-microservice/ldflags/ \
  --ko-build=false \
  --bin-name demo \
  --ldflags "main.Version=1.2.5; main.Commit=abc1234; main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --progress plain -vv \
  export --path=/tmp/microservices/ldflags-test/
```

Build with ldflags and push to GHCR via ko:

```bash
dagger call -m go-microservice run-build-stage \
  --src tests/go-microservice/ldflags/ \
  --ko-build=true \
  --bin-name demo \
  --ldflags "main.Version=1.2.5; main.Commit=abc1234; main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --token=env:GITHUB_TOKEN \
  --ko-push=true \
  --ko-repo ghcr.io/stuttgart-things/test \
  --progress plain -vv \
  export --path=/tmp/microservices/ldflags-test/
```

Check the build report:

```bash
cat /tmp/microservices/ldflags-test/build-report.txt
```

## Parameters

### Static Stage

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--src` | Source directory | Required |
| `--lintCanFail` | Allow lint failures | `false` |
| `--security-scan` | Run security scan | `true` |
| `--test` | Run tests | `true` |

### Build Stage

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--src` | Source directory | Required |
| `--ko-build` | Build with ko | `false` |
| `--bin-name` | Binary name | Required |
| `--ldflags` | Linker flags (semicolon-separated) | - |
| `--ko-push` | Push to registry | `false` |
| `--ko-repo` | Ko repository URL | - |
| `--token` | Registry token | - |

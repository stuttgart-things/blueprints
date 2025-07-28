# stuttgart-things/blueprints/go-microservice

## STATIC STAGE

```bash
# DO ALL STATIC CHECKS
dagger call -m go-microservice \
run-static-stage \
--src tests/go-microservice/ldflags/ \
--progress plain -vv \
export --path=/tmp/report.json

# DO NOT TEST AND NO SECURITY SCAN
dagger call -m go-microservice \
run-static-stage \
--src tests/go-microservice/ldflags/ \
--lintCanFail=true \
--security-scan=false \
--test=false \
--progress plain -vv \
export --path=/tmp/report.json

# CHECK CHECK REPORT
cat /tmp/report.json
```

## BUILD STAGE

```bash
# JUST BUILD BINARY (NO KO BUILD)
dagger call -m go-microservice \
run-build-stage \
--src tests/go-microservice/ldflags/ \
--ko-build=false \
--bin-name demo \
--progress plain -vv \
export --path=/tmp/microservices/ldflags-test/

# JUST BUILD BINARY w/ LDFLAGS (NO KO BUILD)
dagger call -m go-microservice \
run-build-stage \
--src tests/go-microservice/ldflags/ \
--ko-build=false \
--bin-name demo \
--ldflags "main.Version=1.2.5; main.Commit=abc1234; main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
--progress plain -vv \
export --path=/tmp/microservices/ldflags-test/

# BUILD BINARY w/ LDFLAGS + KO BUILD TO GHCR
dagger call -m go-microservice \
run-build-stage \
--src tests/go-microservice/ldflags/ \
--ko-build=true \
--bin-name demo \
--ldflags "main.Version=1.2.5; main.Commit=abc1234; main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
--token=env:GITHUB_TOKEN \
--ko-push=true \
--ko-repo ghcr.io/stuttgart-things/test \
--progress plain -vv \
export --path=/tmp/microservices/ldflags-test/

# CHECK BUILD REPORT
cat /tmp/microservices/ldflags-test/build-report.txt
```

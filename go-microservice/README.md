# stuttgart-things/blueprints/go-microservice

## STATIC STAGE
```bash
dagger call -m go-microservice \
run-static-stage \
--src tests/go/calculator/ \
--lintCanFail=true \
export --path=/tmp/report.json
```

## BUILD STAGE

```bash
# BUILD w/ LDFLAGS
dagger call -m go-microservice \
run-build-stage \
--src tests/go-microservice/ldflags/ \
--ldflags "-X main.Version=1.2.3 -X main.Commit=abc1234 -X main.BuildTime=2025-07-05T13:45:00Z" \
--go-version 1.24.1 \
export --path=/tmp/build
```

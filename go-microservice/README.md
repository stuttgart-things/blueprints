# stuttgart-things/blueprints/go-microservice

```bash
dagger call -m go-microservice \
run-static-stage \
--src tests/go/calculator/ \
--lintCanFail=true export \
--path=/tmp/report.json
```

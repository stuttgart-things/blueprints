# stuttgart-things/blueprints/presentations

```bash
dagger call -m presentations init \
--name backstage \
export --path=/tmp/presentation \
--progress plain
```

```bash
dagger call -m presentations add-content \
--src tests/presentations \
--presentationFile tests/presentations/presentation.yaml \
export --path=tests/presentations/example-site/home \
--progress plain
```

```bash
dagger call -m presentations serve \
--config=tests/presentations/example-site/hugo.toml \
--content=tests/presentations/example-site \
--port=4146 \
up --progress plain
```

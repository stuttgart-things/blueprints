# Presentations Module

Initialize presentation sites (Hugo), add content from configuration, and serve locally.

## Features

- Initialize new Hugo presentation sites
- Add content from YAML configuration files
- Serve presentations locally for preview

## Usage

### Initialize Presentation

Create a new presentation site:

```bash
dagger call -m presentations init \
  --name backstage \
  export --path=/tmp/presentation \
  --progress plain
```

### Add Content

Add slides from a YAML configuration:

```bash
dagger call -m presentations add-content \
  --src tests/presentations \
  --presentationFile tests/presentations/presentation.yaml \
  export --path=tests/presentations/example-site/home \
  --progress plain
```

### Serve Locally

Start local development server:

```bash
dagger call -m presentations serve \
  --config=tests/presentations/example-site/hugo.toml \
  --content=tests/presentations/example-site \
  --port=4146 \
  up --progress plain
```

## Configuration Format

### Presentation YAML

```yaml
---
slides:
  agenda:
    desc: timing and topics
    duration: 5
    order: 00
    file: agenda.md
    background-color: "#E32020FF"
    transition: "fade"
  intro:
    desc: Internal Developer Platforms - Overview
    order: 01
    background-color: "#2e167aff"
    file: https://raw.githubusercontent.com/stuttgart-things/docs/refs/heads/main/slides/idp-intro.md
```

### Slide Content Example

```markdown
# Workshop Agenda
## Platform Engineering with Backstage
### `Insert Today's Date`

---
## Morning Session: Foundations & Orientation
### (9:00 AM - 12:00 PM)

| Time | Session | Duration |
|------|---------|----------|
| 9:00 | **Introduction & Context** | 30 min |
| 9:30 | **Internal Developer Platforms - Overview** | 60 min |
| 10:30 | *Coffee Break* | 15 min |
```

## Parameters

### init

| Parameter | Description |
|-----------|-------------|
| `--name` | Presentation name |

### add-content

| Parameter | Description |
|-----------|-------------|
| `--src` | Source directory |
| `--presentationFile` | Path to presentation YAML configuration |

### serve

| Parameter | Description |
|-----------|-------------|
| `--config` | Path to Hugo config file |
| `--content` | Content directory |
| `--port` | Port to serve on |

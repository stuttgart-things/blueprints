# stuttgart-things/blueprints/presentations

<details>
  <summary>EXAMPLE CONFIGURATION</summary>

```bash
cat <<EOF > agenda.md
# Workshop Agenda
## Platform Engineering with Backstage
### `Insert Today's Date`

---
## Morning Session: Foundations & Orientation
### (9:00 AM - 12:00 PM)

| Time | Session | Duration |
|------|---------|----------|
| 9:00 | **Introduction & Context** | 30 min |
| | • Welcome, target vision, expectation alignment | |
| 9:30 | **Internal Developer Platforms – Overview** | 60 min |
| | • Motivation, definitions (IDP, GitOps, Self-Service, IaC) | |
| | • Showcases & success factors | |
| 10:30 | *Coffee Break* | 15 min |

| 10:45 | **IaC & GitOps at Atruvia** | 45 min |
| | • Current state: Terraform, GitLab, YAML | |
| | • Discussion: Strengths, gaps, pain points | |
| 11:30 | **Backstage Deep Dive I** | 30 min |
| | • Core features (Catalog, TechDocs, Scaffolder, IDP features) | |
| | • Focus: IaC component provisioning | |
| | • Demo: Terraform + GitLab via Backstage | |

---
EOF
```

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
    desc: Internal Developer Platforms – Overview
    order: 01
    background-color: "#2e167aff"
    file: https://raw.githubusercontent.com/stuttgart-things/docs/refs/heads/main/slides/idp-intro.md
```

</details>


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

+++
weight = 30
+++

{{< slide id=infra background-color="#e7b8ddff" type="slide" transition="zoom" transition-speed="fast" >}}

{{% section %}}


# /Backstage

- A framework developed by Spotify
- Manage software ecosystem in one place
- Centralized dashboard for software tools, services, documentation, and infrastructure
- Self-service capabilities for development teams
- Reduce cognitive load, standardize tools, and speed up onboarding
- Backstage brings modularity to the frontend. Allowing the backend services to be easily integrated, discovered, and consumed

---


### /THE Backstage Platform

**Backstage** is an open platform for building Internal Developer Portals, originally created by Spotify.

<img src="https://backstage-spotify-com.spotifycdn.com/_next/static/media/twitter-summary-default.e17fd878.png" alt="Backstage Logo" style="width: 30%; margin: 1rem 0;" />

- 🧩 **Plugin-based architecture** — tailor it to your platform
- 📚 **Software Catalog** — track ownership, metadata & lifecycle
- 🚀 **Scaffolder** — bootstrap new services with templates
- 📊 **TechDocs** — docs-as-code, surfaced directly in the UI
- 🔌 **Ecosystem integrations** — GitHub, Argo CD, Kubernetes

---

### Backstage Concept

- Core: Base functionalities built by open-source project
- App: An instance deployed (Customized & Glues core with plugins
- Plugins: Extends core functionalities


---

### / GITLAB AUTHENTICATION

<img src="images/gl-auth.png" alt="Alt Text" width="7000" style="border: none; box-shadow: none;" />

- No need to manage a separate user database for Backstage
- Fine-grained access control based on GitLab org structure

---

### /SOFTWARE CATALOG

- Central place to manage all your software
- Supports components like services, libraries, APIs, resources
- YAML-based definitions (`catalog-info.yaml`)


---

### / SOFTWARE TEMPLATES


---

### /SOFTWARE TEMPLATE OVERVIEW


---

### /SOFTWARE TEMPLATE DIALOG (CROSSPLANE RESOURCE)


---

### /TEMPLATE DEFINITION


- Presents input parameters to the user (via Backstage UI)
- Uses those parameters to generate code from a template repo

---

### /NAMESPACE TEMPLATE


- Namespace Creation w/ Crossplane
- Nunjucks is a templating language created by Mozilla

---

### /CATALOG TEMPLATE


- 🔁 From input → templated code → Git repo → deployment = self-service, production-ready infrastructure.

---

### /MERGE REQUEST ACTION


- Backstage can create a merge/pull request
— allowing teams to review changes before merging

---

### /Crossplane Lifecycle Process


---


### /SOFTWARE TEMPLATES


---

### /KRO DB-CLAIM


- Kro will render a Kubernetes Deployment, Secret, Service, and PVC based on this claim

---

### / SOFTWARE TEMPLATES


---

### / SOFTWARE TEMPLATES - CUSTOMER GROUPS


---

### / SOFTWARE TEMPLATES - CUSTOMER GROUPS


---


### /SOFTWARE TEMPLATES - CUSTOMER GROUPS


---

### CI/CD GITLAB PIPELINES



---

### /KUBERNETES


- Detailed view of Kubernetes Clusters


---

### /TECHDOCS



---

### /TECHDOCS

- TechDocs View in Backstage



{{% /section %}}

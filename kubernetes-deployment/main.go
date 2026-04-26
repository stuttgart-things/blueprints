// KubernetesDeployment module bundles Helm/Helmfile rendering and apply,
// raw manifest application, KCL-based deployments, CRD installation, Flux
// bootstrap/render/apply/destroy workflows and SOPS-secret helpers used to
// roll out workloads against a Kubernetes cluster. See README.md for usage.

package main

type KubernetesDeployment struct{}

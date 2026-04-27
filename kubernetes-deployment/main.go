// KubernetesDeployment module bundles Helm/Helmfile rendering and apply,
// raw manifest application, KCL-based deployments, CRD installation, and
// SOPS-secret helpers used to roll out workloads against a Kubernetes
// cluster. Flux bootstrap/render/apply/destroy lives in the dedicated
// `flux` module. See README.md for usage.

package main

type KubernetesDeployment struct{}

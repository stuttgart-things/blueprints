// KubernetesDeployment is a thin orchestrator over `kcl` (manifest
// rendering) and `kubectl` (apply): KCL-based deployments, raw manifest
// application, and CRD installation. Helmfile workflows live in the
// dedicated `helm` module; Flux bootstrap/render/apply/destroy lives in
// the dedicated `flux` module; SOPS / k8s Secret manifest workflows live
// in the dedicated `secrets` module. See README.md for usage.

package main

type KubernetesDeployment struct{}

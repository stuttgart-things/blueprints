// Flux module bundles bootstrap, render, apply, and destroy workflows for
// Flux CD on Kubernetes, including KCL-based config rendering, SOPS secret
// encryption, Git commit of rendered manifests, Helmfile-driven operator
// install, and reconciliation waiting via the Flux CLI.

package main

type Flux struct{}

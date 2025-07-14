// Vm module provides a comprehensive workflow to manage virtual machine lifecycle
// and configuration using Terraform and Ansible, integrated with secure secret
// management via Vault and SOPS.
//
// This generated module was created with dagger init as a starting point for VM-related
// operations. It demonstrates key DevOps tasks such as decrypting secrets, applying
// Terraform infrastructure changes, generating dynamic Ansible inventories, and
// executing Ansible playbooks to configure VMs. The module is designed to be flexible
// and extensible to support your infrastructure automation needs.
//
// The primary function Bake orchestrates this workflow, accepting Terraform directories,
// encrypted files, Vault credentials, and Ansible parameters as inputs. It optionally
// decrypts SOPS-encrypted configuration files before applying Terraform operations,
// then parses Terraform outputs to generate inventory files for Ansible. It supports
// multiple inventory types and allows you to specify Ansible playbooks and credentials.
//
// This module can be invoked from the Dagger CLI or programmatically via the SDK,
// making it suitable for integrating into CI/CD pipelines, GitOps workflows, or
// custom operator/controller logic.
//
// Future enhancements planned include:
// - Rendering manifests or configs to branches/PRs for GitOps-style deployments
// - Seamless integration with SOPS for secret management and decryption
// - Advanced Terraform execution and output parsing features
// - Enhanced Ansible inventory generation and execution customization
// - VM testing and validation steps post-provisioning
// - Automated merge requests/PR handling post-deployment
//
// This documentation serves both as a high-level overview and a detailed guide
// to the module’s capabilities and intended use cases.

package main

import (
	"dagger/vm/internal/dagger"
)

var (
	workingDir *dagger.Directory
	workDir    = "/src"
)

type Vm struct {
	BaseImage string
}

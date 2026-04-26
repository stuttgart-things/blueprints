package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
	"path/filepath"
	"strings"
)

// ExecuteAnsibleWithExport runs Ansible playbooks and exports specified files from the container.
// Same parameters as ExecuteAnsible plus exportPaths (comma-separated file paths to extract).
func (m *Vm) ExecuteAnsibleWithExport(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	playbooks string,
	// Comma-separated list of file paths to export from the Ansible container
	exportPaths string,
	// +optional
	requirements *dagger.File,
	// +optional
	inventory *dagger.File,
	// Comma-separated list of hosts (e.g., "192.168.1.10,192.168.1.11")
	// Used to generate inventory if inventory file is not provided
	// +optional
	hosts string,
	// +optional
	parameters string,
	// Path to a YAML file containing parameters (lower priority)
	// +optional
	parametersFile *dagger.File,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// +optional
	sshUser *dagger.Secret,
	// +optional
	sshPassword *dagger.Secret,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	requirementsTemplate string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	requirementsData string,
	// Inventory type: "simple" (default [all] group) or "cluster" (master/worker groups)
	// +optional
	// +default="simple"
	inventoryType string,
) (*dagger.Directory, error) {

	prep, err := m.prepareAnsibleExecution(ctx, src, requirements, inventory, hosts, parameters, parametersFile, requirementsTemplate, requirementsData, inventoryType)
	if err != nil {
		return nil, err
	}

	// EXECUTE ANSIBLE AND EXPORT FILES
	exportDir := dag.Ansible().ExecuteAndExport(
		playbooks,
		exportPaths,
		dagger.AnsibleExecuteAndExportOpts{
			Src:            prep.src,
			Inventory:      prep.inventory,
			Parameters:     prep.parameters,
			VaultAppRoleID: vaultAppRoleID,
			VaultSecretID:  vaultSecretID,
			VaultURL:       vaultURL,
			Requirements:   prep.requirements,
			SSHUser:        sshUser,
			SSHPassword:    sshPassword,
		})

	return exportDir, nil
}

// ExecuteAnsibleEncryptAndCommit runs Ansible playbooks, extracts files from the container,
// encrypts them with SOPS, and commits the encrypted files to a Git repository.
func (m *Vm) ExecuteAnsibleEncryptAndCommit(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	playbooks string,
	// Comma-separated list of file paths to export from the Ansible container
	exportPaths string,
	// +optional
	requirements *dagger.File,
	// +optional
	inventory *dagger.File,
	// Comma-separated list of hosts (e.g., "192.168.1.10,192.168.1.11")
	// +optional
	hosts string,
	// +optional
	parameters string,
	// +optional
	parametersFile *dagger.File,
	// +optional
	vaultAppRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// +optional
	sshUser *dagger.Secret,
	// +optional
	sshPassword *dagger.Secret,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	requirementsTemplate string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	requirementsData string,
	// Inventory type: "simple" (default [all] group) or "cluster" (master/worker groups)
	// +optional
	// +default="simple"
	inventoryType string,
	// AGE public key for SOPS encryption
	agePublicKey *dagger.Secret,
	// File extension for SOPS encryption (e.g., "yaml", "json")
	// +optional
	// +default="yaml"
	sopsFileExtension string,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
	// Git repository in "owner/repo" format
	gitRepository string,
	// Git branch name
	// +optional
	// +default="main"
	gitBranch string,
	// Git commit message
	// +optional
	// +default="Add encrypted files from Ansible execution"
	gitCommitMessage string,
	// Destination path within the git repository
	// +optional
	// +default="/"
	gitDestinationPath string,
	// GitHub token for authentication
	gitToken *dagger.Secret,
	// If non-empty, create this branch (from gitBranch as base) and commit there instead
	// +optional
	gitCreateBranch string,
	// If true (and gitCreateBranch set), open a PR from the new branch back to gitBranch
	// +optional
	gitCreatePr bool,
	// PR title (defaults to gitCommitMessage if empty)
	// +optional
	gitPrTitle string,
	// Comma-separated list of target filenames for exported files (maps 1:1 to exportPaths)
	// If not set, original filenames are used
	// +optional
	exportTargetNames string,
) (string, error) {

	// PHASE 1: Execute Ansible and export files
	exportDir, err := m.ExecuteAnsibleWithExport(
		ctx, src, playbooks, exportPaths, requirements, inventory, hosts, parameters, parametersFile,
		vaultAppRoleID, vaultSecretID, vaultURL, sshUser, sshPassword,
		requirementsTemplate, requirementsData, inventoryType,
	)
	if err != nil {
		return "", fmt.Errorf("ansible execution failed: %w", err)
	}

	// PHASE 2: Encrypt each exported file with SOPS
	entries, err := exportDir.Entries(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list exported files: %w", err)
	}

	// Build rename map from exportTargetNames if provided
	renameMap := make(map[string]string)
	if exportTargetNames != "" {
		targetNames := strings.Split(exportTargetNames, ",")
		exportPathsList := strings.Split(exportPaths, ",")
		if len(targetNames) != len(exportPathsList) {
			return "", fmt.Errorf("exportTargetNames count (%d) must match exportPaths count (%d)", len(targetNames), len(exportPathsList))
		}
		for i, ep := range exportPathsList {
			baseName := filepath.Base(strings.TrimSpace(ep))
			renameMap[baseName] = strings.TrimSpace(targetNames[i])
		}
	}

	encryptedDir := dag.Directory()
	for _, entry := range entries {
		plaintextFile := exportDir.File(entry)

		encryptedContent, err := m.EncryptFile(ctx, agePublicKey, plaintextFile, sopsFileExtension, sopsConfig)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt file %s: %w", entry, err)
		}

		targetName := entry
		if mapped, ok := renameMap[entry]; ok {
			targetName = mapped
		}

		encryptedDir = encryptedDir.WithNewFile(targetName, encryptedContent)
	}

	// PHASE 3: Commit encrypted files to Git
	result, err := m.CommitToGit(ctx, encryptedDir, gitRepository, gitBranch, gitCommitMessage, gitDestinationPath, gitToken, gitCreateBranch, gitCreatePr, gitPrTitle)
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	return result, nil
}

package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func (m *Vm) BakeLocal(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	// +default="apply"
	operation string,
	// +optional
	// e.g., "cpu=4,ram=4096,storage=100"
	variables string,
	// +optional
	encryptedFile *dagger.File,
	// +optional
	sopsKey *dagger.Secret,
	// +optional
	awsAccessKeyID *dagger.Secret,
	// +optional
	awsSecretAccessKey *dagger.Secret,
	// +optional
	vaultRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// vaultToken
	// +optional
	vaultToken *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// +optional
	ansiblePlaybooks string,
	// +optional
	ansibleRequirementsFile *dagger.File,
	// +optional
	ansibleUser *dagger.Secret,
	// +optional
	ansiblePassword *dagger.Secret,
	// Extra environment for the Ansible container, as a secret in dotenv format
	// (NAME=value per line), for playbooks using lookup('env', ...).
	// +optional
	envSecrets *dagger.Secret,
	// +optional
	ansibleParameters string,
	// +optional
	// +default="default"
	ansibleInventoryType string,
	// +optional
	// +default=30
	ansibleWaitTimeout int,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	requirementsTemplate string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	requirementsData string,
	// +optional
	// +default=3
	terraformMaxRetries int,
	// +optional
	// +default=10
	terraformRetryDelay int,
	// Inventory type: "simple" (default [all] group) or "cluster" (master/worker groups)
	// +optional
	// +default="simple"
	inventoryType string,
	// Comma-separated list of file paths to export from the Ansible container
	// +optional
	exportPaths string,
	// AGE public key for SOPS encryption of exported files
	// +optional
	agePublicKey *dagger.Secret,
	// File extension for SOPS encryption (e.g., "yaml", "json")
	// +optional
	// +default="yaml"
	sopsFileExtension string,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
	// Comma-separated list of target filenames for exported files (maps 1:1 to exportPaths)
	// If not set, original filenames are used
	// +optional
	exportTargetNames string,
	// Destination path for encrypted exports within the result directory
	// Use "./" to place files at the root level (no subdirectory)
	// +optional
	// +default="encrypted-exports"
	exportDestinationPath string,
) (*dagger.Directory, error) {
	workDir := "/src"

	// INIT WORKING CONTAINER
	ctr, err := m.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}
	ctr = ctr.WithDirectory(workDir, terraformDir).WithWorkdir(workDir)

	// Inject AWS creds for S3-compatible backend
	if awsAccessKeyID != nil { // pragma: allowlist secret
		ctr = ctr.WithSecretVariable("AWS_ACCESS_KEY_ID", awsAccessKeyID)
	}
	if awsSecretAccessKey != nil { // pragma: allowlist secret
		ctr = ctr.WithSecretVariable("AWS_SECRET_ACCESS_KEY", awsSecretAccessKey)
	}

	// OPTIONAL SOPS DECRYPTION
	if encryptedFile != nil {
		decryptedContent, err := dag.Secrets().Decrypt(ctx, sopsKey, encryptedFile)
		if err != nil {
			return nil, fmt.Errorf("decrypting sops file failed: %w", err)
		}
		// Mounted-and-copied rather than WithNewFile: the plaintext must not
		// become an operation argument, or it lands in the build log.
		ctr = withSecretFile(
			ctr,
			fmt.Sprintf("%s/terraform.tfvars.json", workDir),
			decryptedContent,
			"tfvars-json")

		// Extract Ansible SSH creds from SOPS-decrypted content (CLI flags take precedence)
		if ansibleUser == nil || ansiblePassword == nil { // pragma: allowlist secret
			var tfvars map[string]interface{}
			if err := json.Unmarshal([]byte(decryptedContent), &tfvars); err == nil {
				if ansibleUser == nil {
					if u, ok := tfvars["vm_ssh_user"].(string); ok && u != "" {
						ansibleUser = dag.SetSecret("ansible-user", u)
					}
				}
				if ansiblePassword == nil { // pragma: allowlist secret
					if p, ok := tfvars["vm_ssh_password"].(string); ok && p != "" {
						ansiblePassword = dag.SetSecret("ansible-password", p)
					}
				}
			}
		}
	}

	// RUN TERRAFORM WITH RETRY LOGIC
	var terraformDirResult *dagger.Directory
	var terraformErr error

	maxRetries := terraformMaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := time.Duration(terraformRetryDelay) * time.Second
	if retryDelay <= 0 {
		retryDelay = 10 * time.Second
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		terraformDirResult, terraformErr = m.
			ExecuteTerraform(
				ctx,
				ctr.Directory(workDir),
				operation,
				variables,
				awsAccessKeyID,
				awsSecretAccessKey,
				vaultRoleID,
				vaultSecretID,
				vaultToken,
				nil,   // sopsAgeKey
				"",    // encryptedFiles
				nil,   // kubeConfig
				"",    // kubeConfigPath
				nil,   // encryptedKubeConfig
				"",    // kubeSecretName
				"",    // kubeSecretNamespace
				"",    // kubeSecretJsonpath
				"",    // kubeSecretTfVar
				false, // exportTfOutput
			)

		if terraformErr == nil {
			break
		}

		if attempt < maxRetries {
			fmt.Printf("Terraform attempt %d/%d failed: %v. Retrying in %v...\n", attempt, maxRetries, terraformErr, retryDelay)
			time.Sleep(retryDelay)
		}
	}

	if terraformErr != nil {
		return nil, fmt.Errorf("running terraform failed after %d attempts: %w", maxRetries, terraformErr)
	}

	// IF OPERATION IS NOT APPLY, RETURN EARLY
	if operation != "apply" {
		return terraformDirResult, nil
	}

	// GET TERRAFORM OUTPUT (WITH AWS CREDS FOR REMOTE BACKEND)
	tfOutput, err := m.
		OutputTerraformRunWithCreds(
			ctx,
			terraformDirResult,
			awsAccessKeyID,
			awsSecretAccessKey,
		)
	if err != nil {
		return nil, fmt.Errorf("getting terraform output failed: %w", err)
	}

	// GENERATE ANSIBLE INVENTORY
	var inventory string
	switch ansibleInventoryType {
	case "default":
		inventory, err = CreateDefaultAnsibleInventory(tfOutput)
	case "cluster":
		inventory, err = CreateClusterAnsibleInventory(tfOutput)
	default:
		err = fmt.Errorf("unsupported inventory type: %s", ansibleInventoryType)
	}
	if err != nil {
		return nil, fmt.Errorf("creating inventory failed: %w", err)
	}

	// WRITE INVENTORY TO terraformDirResult
	terraformDirResult = terraformDirResult.WithNewFile("inventory.yaml", inventory)

	// WRITE STAGE OUTPUTS FOR DOWNSTREAM CONSUMERS (e.g. Dapr workflow chain).
	// Consumers download outputs.json as a GitHub Actions artifact named "stage-outputs".
	ips, ipsErr := ParseIPsFromTfOutput(tfOutput)
	if ipsErr != nil {
		// Don't fail the apply — IPs are also reflected in inventory.yaml.
		ips = nil
	}
	stageOutputs := map[string]any{
		"vm_ips": ips,
	}
	stageOutputsBytes, err := json.Marshal(stageOutputs)
	if err != nil {
		return nil, fmt.Errorf("marshal stage outputs: %w", err)
	}
	terraformDirResult = terraformDirResult.WithNewFile("outputs.json", string(stageOutputsBytes))

	// SLEEP BEFORE ANSIBLE (GIVE MACHINES TIME TO BE READY)
	time.Sleep(time.Duration(ansibleWaitTimeout) * time.Second)

	// RUN ANSIBLE (with or without export)
	if exportPaths != "" {
		// EXECUTE ANSIBLE WITH EXPORT
		exportDir, err := m.
			ExecuteAnsibleWithExport(
				ctx,
				terraformDirResult,
				ansiblePlaybooks,
				exportPaths,
				ansibleRequirementsFile,
				terraformDirResult.File("inventory.yaml"),
				"",
				ansibleParameters,
				nil,
				vaultRoleID,
				vaultSecretID,
				vaultURL,
				ansibleUser,
				ansiblePassword,
				envSecrets,
				requirementsTemplate,
				requirementsData,
				inventoryType,
			)
		if err != nil {
			return nil, fmt.Errorf("ansible execution with export failed: %w", err)
		}

		// ENCRYPT EXPORTED FILES WITH SOPS
		if agePublicKey != nil {
			entries, err := exportDir.Entries(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list exported files: %w", err)
			}

			// Build rename map from exportTargetNames if provided
			renameMap := make(map[string]string)
			if exportTargetNames != "" {
				targetNames := strings.Split(exportTargetNames, ",")
				exportPathsList := strings.Split(exportPaths, ",")
				if len(targetNames) != len(exportPathsList) {
					return nil, fmt.Errorf("exportTargetNames count (%d) must match exportPaths count (%d)", len(targetNames), len(exportPathsList))
				}
				for i, ep := range exportPathsList {
					baseName := filepath.Base(strings.TrimSpace(ep))
					renameMap[baseName] = strings.TrimSpace(targetNames[i])
				}
			}

			encryptedDir := dag.Directory()
			for _, entry := range entries {
				plaintextFile := exportDir.File(entry)

				encryptedContent, err := dag.Secrets().EncryptFile(ctx, agePublicKey, plaintextFile, dagger.SecretsEncryptFileOpts{FileExtension: sopsFileExtension, SopsConfig: sopsConfig})
				if err != nil {
					return nil, fmt.Errorf("failed to encrypt file %s: %w", entry, err)
				}

				targetName := entry
				if mapped, ok := renameMap[entry]; ok {
					targetName = mapped
				}

				encryptedDir = encryptedDir.WithNewFile(targetName, encryptedContent)
			}

			// Merge encrypted files into the result directory
			if exportDestinationPath == "./" || exportDestinationPath == "." {
				terraformDirResult = terraformDirResult.WithDirectory("/", encryptedDir)
			} else {
				terraformDirResult = terraformDirResult.WithDirectory(exportDestinationPath, encryptedDir)
			}
		}
	} else {
		// STANDARD ANSIBLE EXECUTION (no exports)
		ansibleSuccess, err := m.
			ExecuteAnsible(
				ctx,
				terraformDirResult,
				ansiblePlaybooks,
				ansibleRequirementsFile,
				terraformDirResult.File("inventory.yaml"),
				"",
				ansibleParameters,
				nil,
				vaultRoleID,
				vaultSecretID,
				vaultURL,
				ansibleUser,
				ansiblePassword,
				envSecrets,
				requirementsTemplate,
				requirementsData,
				inventoryType,
			)

		if err != nil {
			return nil, fmt.Errorf("running ansible failed: %w", err)
		}

		if !ansibleSuccess {
			return nil, fmt.Errorf("ansible execution failed")
		}
	}

	// RETURN UPDATED WORKDIR WITH INVENTORY
	return terraformDirResult, nil
}

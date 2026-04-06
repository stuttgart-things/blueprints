package main

import (
	"context"
	"dagger/configuration/internal/dagger"
	"encoding/json"
	"fmt"
	"strings"
)

// TerraformApply decrypts SOPS-encrypted files, optionally retrieves a Kubernetes
// secret (e.g. VAULT_TOKEN), and runs terraform init + apply.
// Returns the terraform working directory after execution.
func (v *Configuration) TerraformApply(
	ctx context.Context,
	// Directory containing terraform configurations
	terraformDir *dagger.Directory,
	// AGE key for SOPS decryption
	// +optional
	sopsAgeKey *dagger.Secret,
	// Comma-separated list of SOPS-encrypted file paths to decrypt (e.g. "terraform.tfvars.sops.json,secrets.sops.yaml")
	// +optional
	encryptedFiles string,
	// Terraform operation to execute
	// +optional
	// +default="apply"
	operation string,
	// Comma-separated terraform variables (e.g. "name=patrick,food=schnitzel")
	// +optional
	variables string,
	// Kubeconfig secret for Kubernetes state backend access (plaintext)
	// +optional
	kubeConfig *dagger.Secret,
	// Path to mount the kubeconfig inside the container (must match backend config_path in backend.tf)
	// +optional
	// +default="/root/.kube/config"
	kubeConfigPath string,
	// SOPS-encrypted kubeconfig file (e.g. secrets/kubeconfigs/infra-sthings.yaml); decrypted with sopsAgeKey and used for kubectl
	// +optional
	encryptedKubeConfig *dagger.File,
	// Kubernetes secret name to read (e.g. "vault-root-token")
	// +optional
	kubeSecretName string,
	// Kubernetes namespace for the secret
	// +optional
	kubeSecretNamespace string,
	// JSONPath expression to extract from the Kubernetes secret (e.g. ".data.root_token")
	// +optional
	kubeSecretJsonpath string,
	// Terraform variable name to set from the Kubernetes secret value (e.g. "vault_token" becomes -var vault_token=<value>)
	// +optional
	kubeSecretTfVar string,
	// Additional environment variables as comma-separated key=value pairs (e.g. "VAULT_ADDR=https://vault.example.com,VAULT_SKIP_VERIFY=true")
	// +optional
	envVars string,
	// AWS access key ID for S3/MinIO backend
	// +optional
	awsAccessKeyID *dagger.Secret,
	// AWS secret access key for S3/MinIO backend
	// +optional
	awsSecretAccessKey *dagger.Secret,
	// Vault role ID secret
	// +optional
	vaultRoleID *dagger.Secret,
	// Vault secret ID secret
	// +optional
	vaultSecretID *dagger.Secret,
	// Vault token secret
	// +optional
	vaultToken *dagger.Secret,
	// Run terraform output --json after apply and write result to output.json in the returned directory
	// +optional
	exportTfOutput bool,
) (*dagger.Directory, error) {

	tfDir := terraformDir

	// DECRYPT SOPS-ENCRYPTED FILES
	// Known env var keys that should be set as container environment variables
	// rather than written as terraform variable files
	envVarKeys := map[string]bool{
		"VAULT_TOKEN":       true,
		"VAULT_ADDR":        true,
		"VAULT_SKIP_VERIFY": true,
	}
	decryptedEnvVars := map[string]string{}

	if sopsAgeKey != nil && encryptedFiles != "" {
		files := strings.Split(encryptedFiles, ",")
		for _, filePath := range files {
			filePath = strings.TrimSpace(filePath)
			if filePath == "" {
				continue
			}

			encFile := tfDir.File(filePath)

			// Decrypt returns *dagger.File (lazy)
			decryptedFile := dag.Sops().Decrypt(sopsAgeKey, encFile)

			decryptedContent, err := decryptedFile.Contents(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt %s: %w", filePath, err)
			}

			// Check if the decrypted JSON contains env var keys
			// If so, extract them as env vars and only write remaining keys as tfvars
			if strings.HasSuffix(filePath, ".json") {
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(decryptedContent), &parsed); err == nil {
					remaining := map[string]interface{}{}
					for k, v := range parsed {
						if envVarKeys[k] {
							decryptedEnvVars[k] = fmt.Sprintf("%v", v)
						} else {
							remaining[k] = v
						}
					}

					// Only write remaining non-env-var keys as tfvars
					if len(remaining) > 0 {
						outputName := strings.Replace(filePath, ".sops", "", 1)
						remainingJSON, err := json.MarshalIndent(remaining, "", "\t")
						if err != nil {
							return nil, fmt.Errorf("failed to marshal remaining vars from %s: %w", filePath, err)
						}
						tfDir = tfDir.WithNewFile(outputName, string(remainingJSON))
					}
					continue
				}
			}

			// For non-JSON files or unparseable JSON, write as-is
			outputName := strings.Replace(filePath, ".sops", "", 1)
			tfDir = tfDir.WithNewFile(outputName, decryptedContent)
		}
	}

	// DECRYPT KUBECONFIG IF ENCRYPTED
	if encryptedKubeConfig != nil && sopsAgeKey != nil {
		decryptedKubeFile := dag.Sops().Decrypt(sopsAgeKey, encryptedKubeConfig)

		kubeContent, err := decryptedKubeFile.Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt kubeconfig: %w", err)
		}

		kubeConfig = dag.SetSecret("kubeconfig", kubeContent)
	}

	// RETRIEVE KUBERNETES SECRET (e.g. VAULT_TOKEN from cluster)
	var kubeSecretValue string
	if kubeConfig != nil && kubeSecretName != "" && kubeSecretJsonpath != "" {
		resourceKind := fmt.Sprintf("secret %s -o jsonpath='{%s}'", kubeSecretName, kubeSecretJsonpath)

		output, err := dag.Kubernetes().Command(ctx, dagger.KubernetesCommandOpts{
			Operation:         "get",
			ResourceKind:      resourceKind,
			Namespace:         kubeSecretNamespace,
			KubeConfig:        kubeConfig,
			AdditionalCommand: "base64 -d",
		})

		if err != nil {
			return nil, fmt.Errorf("failed to get kubernetes secret %s: %w", kubeSecretName, err)
		}

		kubeSecretValue = strings.TrimSpace(output)
	}

	// BUILD TERRAFORM EXECUTE OPTIONS
	execOpts := dagger.TerraformExecuteOpts{
		Operation: operation,
		Variables: variables,
	}

	if awsAccessKeyID != nil {
		execOpts.AwsAccessKeyID = awsAccessKeyID
	}
	if awsSecretAccessKey != nil {
		execOpts.AwsSecretAccessKey = awsSecretAccessKey
	}
	if vaultRoleID != nil {
		execOpts.VaultRoleID = vaultRoleID
	}
	if vaultSecretID != nil {
		execOpts.VaultSecretID = vaultSecretID
	}
	if vaultToken != nil {
		execOpts.VaultToken = vaultToken
	}

	// Pass decrypted env vars to Execute
	if token, ok := decryptedEnvVars["VAULT_TOKEN"]; ok && vaultToken == nil {
		execOpts.VaultToken = dag.SetSecret("vault-token", token)
	}
	if addr, ok := decryptedEnvVars["VAULT_ADDR"]; ok {
		execOpts.VaultAddr = addr
	}

	if kubeConfig != nil {
		execOpts.KubeConfig = kubeConfig
		execOpts.KubeConfigPath = kubeConfigPath
	}
	if exportTfOutput {
		execOpts.ExportTfOutput = true
	}

	// Pass kubernetes secret value as terraform variable
	if kubeSecretValue != "" && kubeSecretTfVar != "" {
		tfVar := fmt.Sprintf("%s=%s", kubeSecretTfVar, kubeSecretValue)
		if execOpts.Variables != "" {
			execOpts.Variables = execOpts.Variables + "," + tfVar
		} else {
			execOpts.Variables = tfVar
		}
	}

	// Execute returns *dagger.Directory (lazy), sync to trigger execution
	resultDir := dag.Terraform().Execute(tfDir, execOpts)

	_, err := resultDir.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("terraform %s failed: %w", operation, err)
	}

	return resultDir, nil
}

// TerraformOutput retrieves terraform outputs as JSON from an already-applied
// terraform directory (returned by TerraformApply).
func (v *Configuration) TerraformOutput(
	ctx context.Context,
	// Directory containing terraform state (output of TerraformApply)
	terraformDir *dagger.Directory,
	// AWS access key ID for S3/MinIO backend
	// +optional
	awsAccessKeyID *dagger.Secret,
	// AWS secret access key for S3/MinIO backend
	// +optional
	awsSecretAccessKey *dagger.Secret,
	// Kubeconfig secret for Kubernetes backend access
	// +optional
	kubeConfig *dagger.Secret,
	// Path to mount the kubeconfig inside the container (must match backend config_path in backend.tf)
	// +optional
	// +default="/root/.kube/config"
	kubeConfigPath string,
) (string, error) {

	opts := dagger.TerraformOutputOpts{}
	if awsAccessKeyID != nil {
		opts.AwsAccessKeyID = awsAccessKeyID
	}
	if awsSecretAccessKey != nil {
		opts.AwsSecretAccessKey = awsSecretAccessKey
	}
	if kubeConfig != nil {
		opts.KubeConfig = kubeConfig
		opts.KubeConfigPath = kubeConfigPath
	}

	output, err := dag.Terraform().Output(ctx, terraformDir, opts)
	if err != nil {
		return "", fmt.Errorf("terraform output failed: %w", err)
	}

	return output, nil
}

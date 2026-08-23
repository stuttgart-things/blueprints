package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"encoding/json"
	"fmt"
	"strings"
)

// ExecuteTerraform runs terraform with optional SOPS-encrypted file decryption,
// optional Kubernetes-secret retrieval (e.g. VAULT_TOKEN injected as a tfvar),
// optional kubeconfig backend support, and AWS/Vault credentials. Returns the
// terraform working directory after execution. This is the canonical Terraform
// execution entry point — the configuration module previously hosted a
// duplicate (`TerraformApply`) which has been removed.
func (m *Vm) ExecuteTerraform(
	ctx context.Context,
	// Directory containing terraform configurations
	terraformDir *dagger.Directory,
	// Terraform operation to execute
	// +optional
	// +default="apply"
	operation string,
	// Comma-separated terraform variables (e.g. "name=patrick,food=schnitzel")
	// +optional
	variables string,
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
	// AGE key for SOPS decryption of encryptedFiles / encryptedKubeConfig
	// +optional
	sopsAgeKey *dagger.Secret,
	// Comma-separated list of SOPS-encrypted file paths under terraformDir to decrypt
	// (e.g. "terraform.tfvars.sops.json,secrets.sops.yaml")
	// +optional
	encryptedFiles string,
	// Kubeconfig secret for Kubernetes state backend access (plaintext)
	// +optional
	kubeConfig *dagger.Secret,
	// Path to mount the kubeconfig inside the container
	// (must match backend config_path in backend.tf)
	// +optional
	// +default="/root/.kube/config"
	kubeConfigPath string,
	// SOPS-encrypted kubeconfig file; decrypted with sopsAgeKey and used for kubectl
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
	// Terraform variable name to set from the Kubernetes secret value
	// (e.g. "vault_token" becomes -var vault_token=<value>)
	// +optional
	kubeSecretTfVar string,
	// Run terraform output --json after apply and write result to output.json
	// +optional
	exportTfOutput bool,
) (*dagger.Directory, error) {

	tfDir := terraformDir

	// Known env var keys that should flow into the container environment
	// rather than be written as terraform variable files.
	envVarKeys := map[string]bool{
		"VAULT_TOKEN":       true,
		"VAULT_ADDR":        true,
		"VAULT_SKIP_VERIFY": true,
	}
	decryptedEnvVars := map[string]string{}

	// DECRYPT SOPS-ENCRYPTED FILES
	if sopsAgeKey != nil && encryptedFiles != "" {
		files := strings.Split(encryptedFiles, ",")
		for _, filePath := range files {
			filePath = strings.TrimSpace(filePath)
			if filePath == "" {
				continue
			}

			encFile := tfDir.File(filePath)
			decryptedContent, err := dag.Secrets().Decrypt(ctx, sopsAgeKey, encFile)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt %s: %w", filePath, err)
			}

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

					if len(remaining) > 0 {
						outputName := strings.Replace(filePath, ".sops", "", 1)
						remainingJSON, err := json.MarshalIndent(remaining, "", "\t")
						if err != nil {
							return nil, fmt.Errorf("failed to marshal remaining vars from %s: %w", filePath, err)
						}
						tfDir, err = m.directoryWithSecretFile(
							ctx, tfDir, outputName, string(remainingJSON), "tfvars-remaining")
						if err != nil {
							return nil, fmt.Errorf("writing decrypted vars from %s: %w", filePath, err)
						}
					}
					continue
				}
			}

			outputName := strings.Replace(filePath, ".sops", "", 1)
			tfDir, err = m.directoryWithSecretFile(
				ctx, tfDir, outputName, decryptedContent, "tfvars-decrypted")
			if err != nil {
				return nil, fmt.Errorf("writing decrypted %s: %w", filePath, err)
			}
		}
	}

	// DECRYPT KUBECONFIG IF ENCRYPTED
	if encryptedKubeConfig != nil && sopsAgeKey != nil {
		kubeContent, err := dag.Secrets().Decrypt(ctx, sopsAgeKey, encryptedKubeConfig)
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
	if awsSecretAccessKey != nil { // pragma: allowlist secret
		execOpts.AwsSecretAccessKey = awsSecretAccessKey // pragma: allowlist secret
	}
	if vaultRoleID != nil {
		execOpts.VaultRoleID = vaultRoleID
	}
	if vaultSecretID != nil { // pragma: allowlist secret
		execOpts.VaultSecretID = vaultSecretID // pragma: allowlist secret
	}
	if vaultToken != nil {
		execOpts.VaultToken = vaultToken
	}

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

	if kubeSecretValue != "" && kubeSecretTfVar != "" {
		tfVar := fmt.Sprintf("%s=%s", kubeSecretTfVar, kubeSecretValue)
		if execOpts.Variables != "" {
			execOpts.Variables = execOpts.Variables + "," + tfVar
		} else {
			execOpts.Variables = tfVar
		}
	}

	resultDir := dag.Terraform().Execute(tfDir, execOpts)

	if _, err := resultDir.Sync(ctx); err != nil {
		return nil, fmt.Errorf("terraform %s failed: %w", operation, err)
	}

	return resultDir, nil
}

// OutputTerraformRun runs `terraform output --json` against an already-applied
// terraform directory. Supports AWS S3/MinIO and Kubernetes state backends.
func (m *Vm) OutputTerraformRun(
	ctx context.Context,
	// Directory containing terraform state (output of ExecuteTerraform)
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
	// Path to mount the kubeconfig inside the container
	// (must match backend config_path in backend.tf)
	// +optional
	// +default="/root/.kube/config"
	kubeConfigPath string,
) (string, error) {

	opts := dagger.TerraformOutputOpts{}
	if awsAccessKeyID != nil {
		opts.AwsAccessKeyID = awsAccessKeyID
	}
	if awsSecretAccessKey != nil { // pragma: allowlist secret
		opts.AwsSecretAccessKey = awsSecretAccessKey // pragma: allowlist secret
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

// OutputTerraformRunWithCreds is a back-compat alias for OutputTerraformRun
// limited to the AWS credentials path.
func (m *Vm) OutputTerraformRunWithCreds(
	ctx context.Context,
	terraformDir *dagger.Directory,
	// +optional
	awsAccessKeyID *dagger.Secret,
	// +optional
	awsSecretAccessKey *dagger.Secret,
) (string, error) {
	return m.OutputTerraformRun(ctx, terraformDir, awsAccessKeyID, awsSecretAccessKey, nil, "")
}

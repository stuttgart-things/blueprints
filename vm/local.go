package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func (v *Vm) BakeLocal(
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
) (*dagger.Directory, error) {
	workDir := "/src"

	// INIT WORKING CONTAINER
	ctr, err := v.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}
	ctr = ctr.WithDirectory(workDir, terraformDir).WithWorkdir(workDir)

	// OPTIONAL SOPS DECRYPTION
	if encryptedFile != nil {
		decryptedContent, err := v.
			DecryptSops(
				ctx,
				sopsKey,
				encryptedFile,
			)
		if err != nil {
			return nil, fmt.Errorf("decrypting sops file failed: %w", err)
		}
		ctr = ctr.
			WithNewFile(
				fmt.Sprintf("%s/terraform.tfvars.json", workDir),
				decryptedContent)
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
		terraformDirResult, terraformErr = v.
			ExecuteTerraform(
				ctx,
				ctr.Directory(workDir),
				operation,
				variables,
				vaultRoleID,
				vaultSecretID,
				vaultToken,
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

	// GET TERRAFORM OUTPUT
	tfOutput, err := v.
		OutputTerraformRun(
			ctx,
			terraformDirResult,
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

	// SLEEP BEFORE ANSIBLE (GIVE MACHINES TIME TO BE READY)
	time.Sleep(time.Duration(ansibleWaitTimeout) * time.Second)

	// RUN ANSIBLE
	ansibleSuccess, err := v.
		ExecuteAnsible(
			ctx,
			terraformDirResult,
			ansiblePlaybooks,
			ansibleRequirementsFile,
			terraformDirResult.File("inventory.yaml"),
			"",
			ansibleParameters,
			vaultRoleID,
			vaultSecretID,
			vaultURL,
			ansibleUser,
			ansiblePassword,
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

	// RETURN UPDATED WORKDIR WITH INVENTORY
	return terraformDirResult, nil
}

// ProfileConfig represents the YAML structure from parameter-config.yaml
type ProfileConfig struct {
	Operation               string   `yaml:"operation"`
	Variables               []string `yaml:"variables"`
	AnsiblePlaybooks        []string `yaml:"ansiblePlaybooks"`
	AnsibleParameters       []string `yaml:"ansibleParameters"`
	AnsibleInventoryType    string   `yaml:"ansibleInventoryType"`
	AnsibleWaitTimeout      int      `yaml:"ansibleWaitTimeout"`
	EncryptedFile           string   `yaml:"encryptedFile"`
	AnsibleRequirementsFile string   `yaml:"ansibleRequirementsFile"`
	TerraformMaxRetries     int      `yaml:"terraformMaxRetries"`
	TerraformRetryDelay     int      `yaml:"terraformRetryDelay"`
}

func (v *Vm) BakeLocalByProfile(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	profile *dagger.File,
	// +optional
	sopsKey *dagger.Secret,
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
	ansibleUser *dagger.Secret,
	// +optional
	ansiblePassword *dagger.Secret,
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

	// READ AND PARSE PROFILE
	if profile == nil {
		return nil, fmt.Errorf("profile file is required")
	}

	profileContent, err := profile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading profile file failed: %w", err)
	}

	var config ProfileConfig
	if err := yaml.Unmarshal([]byte(profileContent), &config); err != nil {
		return nil, fmt.Errorf("parsing profile YAML failed: %w", err)
	}

	// CONVERT ARRAYS TO COMMA-SEPARATED STRINGS
	variables := strings.Join(config.Variables, ",")
	ansiblePlaybooks := strings.Join(config.AnsiblePlaybooks, ",")
	ansibleParameters := strings.Join(config.AnsibleParameters, ",")

	// GET FILE REFERENCES FROM CONFIG
	var encryptedFile *dagger.File
	if config.EncryptedFile != "" {
		encryptedFile = src.File(config.EncryptedFile)
	}

	var ansibleRequirementsFile *dagger.File
	if config.AnsibleRequirementsFile != "" {
		ansibleRequirementsFile = src.File(config.AnsibleRequirementsFile)
	}

	// SET DEFAULTS FOR RETRY PARAMETERS IF NOT IN PROFILE
	maxRetries := config.TerraformMaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := config.TerraformRetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}

	// CALL BakeLocal WITH CONVERTED PARAMETERS
	return v.BakeLocal(
		ctx,
		src,
		config.Operation,
		variables,
		encryptedFile,
		sopsKey,
		vaultRoleID,
		vaultSecretID,
		vaultToken,
		vaultURL,
		ansiblePlaybooks,
		ansibleRequirementsFile,
		ansibleUser,
		ansiblePassword,
		ansibleParameters,
		config.AnsibleInventoryType,
		config.AnsibleWaitTimeout,
		requirementsTemplate,
		requirementsData,
		maxRetries,
		retryDelay,
		inventoryType,
	)
}

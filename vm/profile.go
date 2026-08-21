package main

import (
	"context"
	"dagger/vm/internal/dagger"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

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
	ExportPaths             []string `yaml:"exportPaths"`
	ExportTargetNames       []string `yaml:"exportTargetNames"`
	SopsFileExtension       string   `yaml:"sopsFileExtension"`
	ExportDestinationPath   string   `yaml:"exportDestinationPath"`
}

func (m *Vm) BakeLocalByProfile(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	profile *dagger.File,
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
	ansibleUser *dagger.Secret,
	// +optional
	ansiblePassword *dagger.Secret,
	// Extra environment for the Ansible container, as a secret in dotenv format
	// (NAME=value per line), for playbooks using lookup('env', ...).
	// +optional
	envSecrets *dagger.Secret,
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
	// AGE public key for SOPS encryption of exported files
	// +optional
	agePublicKey *dagger.Secret,
	// SOPS config file (.sops.yaml)
	// +optional
	sopsConfig *dagger.File,
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
	exportPaths := strings.Join(config.ExportPaths, ",")
	exportTargetNames := strings.Join(config.ExportTargetNames, ",")

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

	// SET SOPS FILE EXTENSION DEFAULT
	sopsFileExtension := config.SopsFileExtension
	if sopsFileExtension == "" {
		sopsFileExtension = "yaml"
	}

	// CALL BakeLocal WITH CONVERTED PARAMETERS
	return m.BakeLocal(
		ctx,
		src,
		config.Operation,
		variables,
		encryptedFile,
		sopsKey,
		awsAccessKeyID,
		awsSecretAccessKey,
		vaultRoleID,
		vaultSecretID,
		vaultToken,
		vaultURL,
		ansiblePlaybooks,
		ansibleRequirementsFile,
		ansibleUser,
		ansiblePassword,
		envSecrets,
		ansibleParameters,
		config.AnsibleInventoryType,
		config.AnsibleWaitTimeout,
		requirementsTemplate,
		requirementsData,
		maxRetries,
		retryDelay,
		inventoryType,
		exportPaths,
		agePublicKey,
		sopsFileExtension,
		sopsConfig,
		exportTargetNames,
		config.ExportDestinationPath,
	)
}

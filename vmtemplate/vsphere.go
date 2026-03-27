package main

import (
	"context"
	"dagger/vmtemplate/internal/dagger"
	"fmt"
)

func (m *Vmtemplate) RunVsphereWorkflow(
	ctx context.Context,
	// The Packer configuration file name (after rendering, e.g., "vsphere-base-os.pkr.hcl")
	packerConfig string,
	// The Packer version to use
	// +optional
	// +default="1.13.1"
	packerVersion string,
	// The Packer arch
	// +optional
	// +default="linux_amd64"
	arch string,
	// If true, only init packer w/out build
	// +optional
	// +default=false
	initOnly bool,
	// vaultAddr
	// +optional
	vaultAddr string,
	// vaultRoleID
	// +optional
	vaultRoleID *dagger.Secret,
	// vaultSecretID
	// +optional
	vaultSecretID *dagger.Secret,
	// vaultToken
	// +optional
	vaultToken *dagger.Secret,
	// Directory containing packer template files (.tmpl), e.g., packer/templates/packer
	packerTemplatesDir *dagger.Directory,
	// Comma-separated list of packer template files to render (e.g., "vsphere-base-os.pkr.hcl.tmpl,user-data.tmpl")
	packerTemplates string,
	// Directory containing test VM template files (.tmpl), e.g., packer/templates/test-vm
	// +optional
	testVmTemplatesDir *dagger.Directory,
	// Comma-separated list of test VM template files to render (e.g., "test-vm.tf.tmpl,state.tf.tmpl")
	// +optional
	testVmTemplates string,
	// Directory containing build-specific variables and static files (e.g., base-os.yaml, meta-data)
	buildDir *dagger.Directory,
	// Directory containing shared environment variable files (e.g., packer/environments)
	// +optional
	envDir *dagger.Directory,
	// Comma-separated list of YAML variable files to merge, in priority order (last wins)
	variablesFiles string,
	// Comma-separated key=value overrides with highest priority (e.g., "isoChecksum=abc123,cpus=16")
	// +optional
	overrides string,
	// Enable test VM creation and validation before promotion
	// +optional
	// +default=false
	testVm bool,
	// Comma-separated Ansible playbook paths for test VM validation
	// +optional
	testPlaybooks string,
	// Ansible requirements file for test playbooks
	// +optional
	testRequirements *dagger.File,
	// Seconds to wait for test VM before running Ansible
	// +optional
	// +default=30
	ansibleWaitTimeout int,
	// SSH user for test VM
	// +optional
	sshUser *dagger.Secret,
	// SSH password for test VM
	// +optional
	sshPassword *dagger.Secret,
	// Ansible parameters for test playbooks (e.g., "key1=value1,key2=value2")
	// +optional
	ansibleParameters string,
	// Ansible inventory type: "simple" or "cluster"
	// +optional
	// +default="simple"
	ansibleInventoryType string,
	// Enable golden image promotion (rename, move, delete old)
	// +optional
	// +default=false
	promoteTemplate bool,
	// Target name for the golden template (e.g., "ubuntu25-base")
	// +optional
	goldenTemplateName string,
	// vCenter folder to move the golden template to (e.g., "/LabUL/vm/golden")
	// +optional
	goldenTemplateFolder string,
	// vCenter URL for govc operations
	// +optional
	vcenter *dagger.Secret,
	// vCenter username for govc operations
	// +optional
	vcenterUsername *dagger.Secret,
	// vCenter password for govc operations
	// +optional
	vcenterPassword *dagger.Secret,
) (string, error) {

	// STEP 1: RENDER ALL TEMPLATES UPFRONT
	fmt.Println("RENDERING PACKER BUILD CONFIG...")

	renderedPackerDir, err := m.RenderBuildConfig(
		ctx, packerTemplatesDir, packerTemplates, buildDir, variablesFiles, envDir, overrides,
	)
	if err != nil {
		return "", fmt.Errorf("rendering packer config failed: %w", err)
	}

	// MERGE RENDERED FILES WITH STATIC BUILD FILES (base-os.yaml, meta-data)
	configDir := buildDir.WithDirectory(".", renderedPackerDir)

	// RENDER TEST VM TEMPLATES IF TESTING IS ENABLED
	var renderedTestVmDir *dagger.Directory
	if testVm && testVmTemplates != "" && testVmTemplatesDir != nil {
		fmt.Println("RENDERING TEST VM CONFIG...")

		renderedTestVmDir, err = m.RenderBuildConfig(
			ctx, testVmTemplatesDir, testVmTemplates, buildDir, variablesFiles, envDir, overrides,
		)
		if err != nil {
			return "", fmt.Errorf("rendering test VM config failed: %w", err)
		}
	}

	// STEP 2: BAKE THE PACKER TEMPLATE
	fmt.Println("BAKING PACKER TEMPLATE...")

	vmTemplateName, err := m.Bake(
		ctx,
		configDir,
		packerConfig,
		packerVersion,
		arch,
		initOnly,
		vaultAddr,
		vaultRoleID,
		vaultSecretID,
		vaultToken,
	)

	if err != nil {
		return "", fmt.Errorf("baking packer template failed: %w", err)
	}

	fmt.Println("VM Template Name:", vmTemplateName)

	// STEP 3: OPTIONAL TEST VM VALIDATION
	if testVm {
		fmt.Println("STARTING TEST VM VALIDATION...")

		testTfDir := renderedTestVmDir
		testVmVariables := "vault_addr=" + vaultAddr + ",vm_name=testvm-dagger,vsphere_vm_template=" + vmTemplateName

		// CREATE TEST VM + RUN ANSIBLE TESTS VIA VM MODULE
		testResultDir := dag.VM().BakeLocal(
			testTfDir,
			dagger.VMBakeLocalOpts{
				Operation:               "apply",
				Variables:               testVmVariables,
				VaultRoleID:             vaultRoleID,
				VaultSecretID:           vaultSecretID,
				VaultToken:              vaultToken,
				AnsiblePlaybooks:        testPlaybooks,
				AnsibleRequirementsFile: testRequirements,
				AnsibleUser:             sshUser,
				AnsiblePassword:         sshPassword,
				AnsibleParameters:       ansibleParameters,
				AnsibleInventoryType:    ansibleInventoryType,
				AnsibleWaitTimeout:      ansibleWaitTimeout,
				InventoryType:           ansibleInventoryType,
			},
		)

		// SYNC TO EXECUTE AND CHECK FOR ERRORS
		testResultDir, testErr := testResultDir.Sync(ctx)

		if testErr != nil {
			fmt.Println("TEST VM VALIDATION FAILED, DESTROYING TEST VM...")
			destroyDir := dag.VM().ExecuteTerraform(
				testResultDir,
				dagger.VMExecuteTerraformOpts{
					Operation:     "destroy",
					Variables:     testVmVariables,
					VaultRoleID:   vaultRoleID,
					VaultSecretID: vaultSecretID,
					VaultToken:    vaultToken,
				},
			)
			_, _ = destroyDir.Sync(ctx)
			return vmTemplateName, fmt.Errorf("test VM validation failed: %w", testErr)
		}

		fmt.Println("TEST VM VALIDATION PASSED, DESTROYING TEST VM...")

		destroyDir := dag.VM().ExecuteTerraform(
			testResultDir,
			dagger.VMExecuteTerraformOpts{
				Operation:     "destroy",
				Variables:     testVmVariables,
				VaultRoleID:   vaultRoleID,
				VaultSecretID: vaultSecretID,
				VaultToken:    vaultToken,
			},
		)

		if _, destroyErr := destroyDir.Sync(ctx); destroyErr != nil {
			return vmTemplateName, fmt.Errorf("test VM destroy failed: %w", destroyErr)
		}

		fmt.Println("TEST VM DESTROYED SUCCESSFULLY")
	}

	// STEP 4: OPTIONAL GOLDEN IMAGE PROMOTION
	if promoteTemplate {
		if goldenTemplateName == "" {
			return vmTemplateName, fmt.Errorf("goldenTemplateName is required when promoteTemplate is enabled")
		}

		fmt.Println("STARTING GOLDEN IMAGE PROMOTION...")

		oldTemplateName := goldenTemplateName + "-old"

		fmt.Printf("RENAMING EXISTING GOLDEN TEMPLATE %s → %s\n", goldenTemplateName, oldTemplateName)
		if err := dag.Packer().Vcenteroperation(
			ctx,
			vcenter,
			vcenterUsername,
			vcenterPassword,
			dagger.PackerVcenteroperationOpts{
				Operation: "rename",
				Source:    goldenTemplateName,
				Target:    oldTemplateName,
			},
		); err != nil {
			return vmTemplateName, fmt.Errorf("renaming existing golden template failed: %w", err)
		}

		fmt.Printf("RENAMING NEW TEMPLATE %s → %s\n", vmTemplateName, goldenTemplateName)
		if err := dag.Packer().Vcenteroperation(
			ctx,
			vcenter,
			vcenterUsername,
			vcenterPassword,
			dagger.PackerVcenteroperationOpts{
				Operation: "rename",
				Source:    vmTemplateName,
				Target:    goldenTemplateName,
			},
		); err != nil {
			return vmTemplateName, fmt.Errorf("renaming new template failed: %w", err)
		}

		if goldenTemplateFolder != "" {
			fmt.Printf("MOVING TEMPLATE %s → %s\n", goldenTemplateName, goldenTemplateFolder)
			if err := dag.Packer().Vcenteroperation(
				ctx,
				vcenter,
				vcenterUsername,
				vcenterPassword,
				dagger.PackerVcenteroperationOpts{
					Operation: "move",
					Source:    goldenTemplateName,
					Target:    goldenTemplateFolder,
				},
			); err != nil {
				return vmTemplateName, fmt.Errorf("moving template to golden folder failed: %w", err)
			}
		}

		fmt.Printf("DELETING OLD TEMPLATE %s\n", oldTemplateName)
		if err := dag.Packer().Vcenteroperation(
			ctx,
			vcenter,
			vcenterUsername,
			vcenterPassword,
			dagger.PackerVcenteroperationOpts{
				Operation: "delete",
				Source:    oldTemplateName,
				Target:    "template",
			},
		); err != nil {
			return vmTemplateName, fmt.Errorf("deleting old template failed: %w", err)
		}

		fmt.Println("GOLDEN IMAGE PROMOTION COMPLETED")
	}

	return vmTemplateName, nil
}

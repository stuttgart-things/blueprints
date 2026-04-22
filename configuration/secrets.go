package main

import (
	"context"
	"dagger/configuration/internal/dagger"
	"fmt"
)

func (v *Configuration) CreateSecretsFile(
	ctx context.Context,
	// Private AGE key secret used for SOPS decrypt (AGE-SECRET-KEY-...)
	ageKey *dagger.Secret,
	// SOPS-encrypted data file (YAML/JSON) whose values feed the template
	encryptedDataFile *dagger.File,
	// Go template file (e.g. secret.json.tmpl) rendered against the decrypted data
	templateFile *dagger.File,
	// Public AGE recipient secret used for SOPS re-encrypt (age1...); required when encrypt=true
	// +optional
	ageRecipient *dagger.Secret,
	// File extension for the SOPS-encrypted output
	// +optional
	// +default="json"
	fileExtension string,
	// Optional .sops.yaml used for both decrypt and encrypt
	// +optional
	sopsConfig *dagger.File,
	// When true, SOPS-encrypt the rendered file; when false, return the plaintext render
	// +optional
	// +default="true"
	encrypt bool,
) (*dagger.File, error) {

	decryptOpts := dagger.SopsDecryptOpts{}
	if sopsConfig != nil {
		decryptOpts.SopsConfig = sopsConfig
	}
	decrypted := dag.Sops().Decrypt(ageKey, encryptedDataFile, decryptOpts)

	const (
		dataName     = "data.yaml"
		templateName = "template.tmpl"
	)
	workDir := dag.Directory().
		WithFile(dataName, decrypted).
		WithFile(templateName, templateFile)

	rendered := dag.Templating().RenderFromFile(
		templateName,
		dataName,
		dagger.TemplatingRenderFromFileOpts{
			Src:        workDir,
			StrictMode: true,
		},
	)

	entries, err := rendered.Entries(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing rendered directory: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("templating produced no files")
	}
	renderedFile := rendered.File(entries[0])

	if !encrypt {
		return renderedFile, nil
	}

	if ageRecipient == nil {
		return nil, fmt.Errorf("ageRecipient is required when encrypt=true")
	}

	encryptOpts := dagger.SopsEncryptOpts{FileExtension: fileExtension}
	if sopsConfig != nil {
		encryptOpts.SopsConfig = sopsConfig
	}
	return dag.Sops().Encrypt(ageRecipient, renderedFile, encryptOpts), nil
}

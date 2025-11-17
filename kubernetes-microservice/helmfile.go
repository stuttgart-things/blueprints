package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
	"fmt"
)

// AnalyzeHelmfile reads a helmfile and uses AI to analyze what cluster resources it references,
// then queries those resources and provides validation/recommendations
func (m *KubernetesMicroservice) AnalyzeHelmfile(
	ctx context.Context,
	// The helmfile directory to analyze
	src *dagger.Directory,
	// Path to the helmfile within the directory
	// +optional
	// +default="helmfile.yaml"
	helmfilePath string,
	// Kubeconfig for cluster queries
	kubeConfig *dagger.Secret,
	// +optional
	// +default="claude-3-5-sonnet-20241022"
	model string,
	// +optional
	// +default="helmfile-analysis.yaml"
	outputFile string,
) (*dagger.File, error) {

	// Step 1: Read the helmfile content
	helmfileContent, err := src.File(helmfilePath).Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read helmfile: %w", err)
	}

	// Step 2: Use AI to analyze the helmfile and determine what to query
	analyzeEnv := dag.Env().
		WithStringInput("helmfile_content", helmfileContent, "the helmfile to analyze").
		WithStringOutput("analysis_scope", "description of what cluster resources need to be queried")

	analyzeWork := dag.LLM().
		WithModel(model).
		WithEnv(analyzeEnv).
		WithPrompt(`
			You are a Kubernetes and Helm expert. Analyze this helmfile and determine what cluster resources should be queried to validate it.

			Helmfile content:
			$helmfile_content

			Look for:
			1. Storage-related configurations (persistence.enabled, storageClass references)
			2. Ingress configurations (ingress.enabled, ingressClassName, TLS/cert-manager annotations)
			3. Certificate/Issuer references (cert-manager.io/cluster-issuer, etc.)
			4. Service type configurations (LoadBalancer, NodePort that might need validation)
			5. Resource quotas or limits that should be checked against cluster capacity
			6. Any explicit cluster resource references in values

			Generate a natural language scope description for the cluster query. For example:
			"Check available storage classes, cluster issuers for cert-manager, and ingress classes. Also verify if the namespace has any existing ingress resources."

			Return your scope in the 'analysis_scope' output.
		`)

	analysisScope, err := analyzeWork.Env().Output("analysis_scope").AsString(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis scope: %w", err)
	}

	// Step 3: Use the Config function to query the cluster with the helmfile as context
	helmfileFile := src.File(helmfilePath)

	configResult, err := m.Config(
		ctx,
		analysisScope,
		kubeConfig,
		model,
		"", // cluster-wide query
		outputFile,
		helmfileFile,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze cluster for helmfile: %w", err)
	}

	return configResult, nil
}

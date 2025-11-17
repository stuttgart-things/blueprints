package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
	"encoding/json"
	"fmt"
	"strings"
)

func (m *KubernetesMicroservice) Config(
	ctx context.Context,
	// The scope/prompt that defines what cluster information to gather
	promptScope string,
	kubeConfig *dagger.Secret,
	// +optional
	// +default="claude-3-5-sonnet-20241022"
	model string,
	// +optional
	namespace string,
	// +optional
	// +default="cluster-analysis.yaml"
	outputFile string,
	// +optional
	// Additional context file (e.g., helmfile, manifest) to validate against cluster state
	contextFile *dagger.File,
) (*dagger.File, error) {

	// Read context file if provided
	var contextContent string
	if contextFile != nil {
		var err error
		contextContent, err = contextFile.Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read context file: %w", err)
		}
	}

	// Step 1: Use AI to determine what kubectl commands to run based on the user's request
	planEnvironment := dag.Env().
		WithStringInput("user_request", promptScope, "the user's request describing what they want to know").
		WithStringInput("namespace", namespace, "optional namespace filter, empty means cluster-wide").
		WithStringOutput("kubectl_commands", "JSON array of kubectl commands to execute")

	// Add context file content if provided
	if contextContent != "" {
		planEnvironment = planEnvironment.
			WithStringInput("context_file", contextContent, "additional context file content for validation")
	}

	planWork := dag.LLM().
		WithModel(model).
		WithEnv(planEnvironment).
		WithPrompt(`
			You are a Kubernetes expert. Based on the user's request, determine what kubectl commands to run to gather the necessary cluster information.

			User's request: $user_request
			Namespace filter: $namespace (if empty, consider cluster-wide resources)
			Context file (if provided): $context_file

			Examples of what users might ask:
			- "which cluster issuer is installed" -> check clusterissuers (cluster-wide resource, namespace: "")
			- "what databases are running in default namespace" -> check statefulsets, deployments (namespace: "default")
			- "show me all ingress configurations across the cluster" -> check ingress resources (namespace: "ALL")
			- "what storage classes exist" -> check storageclasses (cluster-wide resource, namespace: "")
			- "list all nodes" -> check nodes (cluster-wide resource, namespace: "")
			- "show all pods in the entire cluster" -> check pods (namespace: "ALL")
			- "what's running everywhere" -> check deployments, pods, services (namespace: "ALL")
			- "get all ingress resources and the used issuers" -> check ingress (namespace: "ALL"), clusterissuers (namespace: ""), issuers (namespace: "ALL")
			- "find pods with ingress in the name" -> check pods (namespace: "ALL", additionalCommand: "grep ingress")

			When a context file is provided:
			- Analyze the context file to identify what cluster resources need to be queried
			- For helmfile/manifest validation: check referenced resources like storageclasses, clusterissuers, ingressclasses, ingress domains
			- Extract specific values from context file that need cluster validation (e.g., domain names, storage class names, issuer names)

			Generate output 'kubectl_commands' containing a JSON array of kubectl command objects. Each object should have:
			{
				"operation": "get",
				"resourceKind": "resource-type",
				"namespace": "namespace-name, empty for cluster-wide resources, or 'ALL' for all namespaces",
				"description": "why this resource is being queried",
				"additionalCommand": "optional shell command to filter/process output (e.g., 'grep ingress', 'wc -l' to count)"
			}

			IMPORTANT:
			- DO NOT use 'grep Running' to filter pods by status
			- Leave additionalCommand empty for pod queries - the analysis LLM will count and filter as needed
			- Only use additionalCommand for genuine text filtering needs (e.g., 'grep ingress' to find names containing 'ingress')

			Cluster-wide resources (no namespace): nodes, clusterroles, clusterrolebindings, clusterissuers, storageclasses, ingressclasses, persistentvolumes, customresourcedefinitions, namespaces
			Namespace-scoped resources: pods, services, deployments, statefulsets, ingress, configmaps, secrets, pvc, certificates, issuers, etc.

			Namespace handling:
			- If namespace parameter is provided and resource is namespace-scoped: use that specific namespace
			- If resource is cluster-wide: use empty string "" for namespace
			- If user wants to see namespace-scoped resources across ALL namespaces: use "ALL" for namespace

			Return ONLY valid JSON array in the 'kubectl_commands' output, no additional text or markdown formatting.
			Example outputs:
			[
				{"operation": "get", "resourceKind": "clusterissuers", "namespace": "", "description": "check for cert-manager cluster issuers"},
				{"operation": "get", "resourceKind": "storageclasses", "namespace": "", "description": "check available storage classes"}
			]
			OR
			[
				{"operation": "get", "resourceKind": "deployments", "namespace": "default", "description": "check deployments in default namespace"},
				{"operation": "get", "resourceKind": "services", "namespace": "default", "description": "check services in default namespace"}
			]
			OR
			[
				{"operation": "get", "resourceKind": "pods", "namespace": "ALL", "description": "list all pods across all namespaces"},
				{"operation": "get", "resourceKind": "ingress", "namespace": "ALL", "description": "check ingress across all namespaces"}
			]
			OR (with filtering)
			[
				{"operation": "get", "resourceKind": "pods", "namespace": "ALL", "description": "find pods with 'ingress' in the name", "additionalCommand": "grep ingress"},
				{"operation": "get", "resourceKind": "ingress", "namespace": "ALL", "description": "get all ingress resources"},
				{"operation": "get", "resourceKind": "clusterissuers", "namespace": "", "description": "get cluster-wide issuers"},
				{"operation": "get", "resourceKind": "issuers", "namespace": "ALL", "description": "get namespace-scoped issuers"}
			]
		`)

	// Get the planned kubectl commands from the environment output
	commandsJSON, err := planWork.Env().Output("kubectl_commands").AsString(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM output: %w", err)
	}

	// Debug: Check if commandsJSON is empty
	if commandsJSON == "" {
		return nil, fmt.Errorf("LLM returned empty response for kubectl_commands output")
	}

	// Step 2: Parse the JSON plan and execute kubectl commands dynamically
	type KubectlCommand struct {
		Operation         string `json:"operation"`
		ResourceKind      string `json:"resourceKind"`
		Namespace         string `json:"namespace"`
		Description       string `json:"description"`
		AdditionalCommand string `json:"additionalCommand,omitempty"`
	}

	// Clean the JSON response - remove markdown code fences if present
	cleanJSON := strings.TrimSpace(commandsJSON)
	// Remove ```json and ``` markers
	if strings.HasPrefix(cleanJSON, "```json") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
		cleanJSON = strings.TrimSpace(cleanJSON)
	} else if strings.HasPrefix(cleanJSON, "```") {
		cleanJSON = strings.TrimPrefix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	}
	if strings.HasSuffix(cleanJSON, "```") {
		cleanJSON = strings.TrimSuffix(cleanJSON, "```")
		cleanJSON = strings.TrimSpace(cleanJSON)
	}

	var commands []KubectlCommand
	err = json.Unmarshal([]byte(cleanJSON), &commands)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubectl commands plan: %w\nRaw response: %s\nCleaned JSON: %s", err, commandsJSON, cleanJSON)
	}

	// Execute each planned kubectl command and collect results
	clusterDataEnvironment := dag.Env().
		WithStringInput("user_request", promptScope, "the user's original request").
		WithStringInput("namespace", namespace, "optional namespace filter").
		WithStringInput("kubectl_plan", commandsJSON, "the planned kubectl commands").
		WithStringOutput("configuration", "the generated application configuration values")

	// Add context file to analysis environment if provided
	if contextContent != "" {
		clusterDataEnvironment = clusterDataEnvironment.
			WithStringInput("context_file", contextContent, "context file content for validation against cluster state")
	}

	// Loop through each planned command and execute it
	for i, cmd := range commands {
		result, err := dag.Kubernetes().Command(
			ctx,
			dagger.KubernetesCommandOpts{
				Operation:         cmd.Operation,
				ResourceKind:      cmd.ResourceKind,
				Namespace:         cmd.Namespace,
				KubeConfig:        kubeConfig,
				AdditionalCommand: cmd.AdditionalCommand,
			},
		)

		// Store the result even if there's an error (will be empty or error message)
		if err != nil {
			result = fmt.Sprintf("Error querying %s: %v", cmd.ResourceKind, err)
		}

		// Add each result to the environment with a unique key
		envKey := fmt.Sprintf("query_%d_%s", i, cmd.ResourceKind)
		envDescription := fmt.Sprintf("%s: %s", cmd.ResourceKind, cmd.Description)
		clusterDataEnvironment = clusterDataEnvironment.
			WithStringInput(envKey, result, envDescription)
	}

	// Step 3: Use AI to analyze the gathered information and generate configuration
	// Build the dynamic prompt showing all query results
	promptBuilder := `You are a Kubernetes configuration expert.

User's request: $user_request
Namespace filter: $namespace (if empty, cluster-wide query)
Context file (if provided): $context_file

Planned queries executed: $kubectl_plan

Query results from the cluster:`

	// Add references to each query result
	for i, cmd := range commands {
		envKey := fmt.Sprintf("query_%d_%s", i, cmd.ResourceKind)
		promptBuilder += fmt.Sprintf("\n- %s (%s): $%s", cmd.ResourceKind, cmd.Description, envKey)
	}

	promptBuilder += `

Analyze the query results and answer the user's question.

Provide:
- Direct answer with specific counts/details from the data
- Summary of resources found
- Important status observations
- YAML configuration if requested

When a context file is provided:
- Validate values in the context file against actual cluster resources
- Check if referenced resources exist (e.g., storageClass, clusterIssuer, ingressClass)
- Verify domain names match existing ingress resources
- Highlight any mismatches or missing resources
- Suggest corrections with actual cluster values

Be concise and data-driven.
`

	configWork := dag.LLM().
		WithModel(model).
		WithEnv(clusterDataEnvironment).
		WithPrompt(promptBuilder)

	// Get the AI-generated configuration/response
	// Try LastReply() first as the LLM generates direct output
	configuration, err := configWork.LastReply(ctx)
	if err != nil {
		return nil, err
	}

	// If LastReply is empty, fall back to the environment output
	if configuration == "" || configuration == "(no reply)" {
		configuration, err = configWork.Env().Output("configuration").AsString(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Return as a file
	return dag.Directory().
		WithNewFile(outputFile, configuration).
		File(outputFile), nil
}

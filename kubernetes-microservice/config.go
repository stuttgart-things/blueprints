package main

import (
	"context"
	"dagger/kubernetes-microservice/internal/dagger"
	"encoding/json"
	"fmt"
	"strings"
)

func (m *KubernetesMicroservice) GenerateKubernetesAppConfiguration(
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
) (*dagger.File, error) {

	// Step 1: Use AI to determine what kubectl commands to run based on the user's request
	planEnvironment := dag.Env().
		WithStringInput("user_request", promptScope, "the user's request describing what they want to know").
		WithStringInput("namespace", namespace, "optional namespace filter, empty means cluster-wide").
		WithStringOutput("kubectl_commands", "JSON array of kubectl commands to execute")

	planWork := dag.LLM().
		WithModel(model).
		WithEnv(planEnvironment).
		WithPrompt(`
			You are a Kubernetes expert. Based on the user's request, determine what kubectl commands to run to gather the necessary cluster information.

			User's request: $user_request
			Namespace filter: $namespace (if empty, consider cluster-wide resources)

			Examples of what users might ask:
			- "which cluster issuer is installed" -> check clusterissuers (cluster-wide resource, namespace: "")
			- "what databases are running in default namespace" -> check statefulsets, deployments (namespace: "default")
			- "show me all ingress configurations across the cluster" -> check ingress resources (namespace: "ALL")
			- "what storage classes exist" -> check storageclasses (cluster-wide resource, namespace: "")
			- "list all nodes" -> check nodes (cluster-wide resource, namespace: "")
			- "show all pods in the entire cluster" -> check pods (namespace: "ALL")
			- "what's running everywhere" -> check deployments, pods, services (namespace: "ALL")

			Output a JSON array of kubectl command objects. Each object should have:
			{
				"operation": "get",
				"resourceKind": "resource-type",
				"namespace": "namespace-name, empty for cluster-wide resources, or 'ALL' for all namespaces",
				"description": "why this resource is being queried"
			}

			Cluster-wide resources (no namespace): nodes, clusterroles, clusterrolebindings, clusterissuers, storageclasses, ingressclasses, persistentvolumes, customresourcedefinitions, namespaces
			Namespace-scoped resources: pods, services, deployments, statefulsets, ingress, configmaps, secrets, pvc, certificates, issuers, etc.

			Namespace handling:
			- If namespace parameter is provided and resource is namespace-scoped: use that specific namespace
			- If resource is cluster-wide: use empty string "" for namespace
			- If user wants to see namespace-scoped resources across ALL namespaces: use "ALL" for namespace

			Return ONLY valid JSON array, no additional text.
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
		`)

	// Get the planned kubectl commands
	commandsJSON, err := planWork.Env().Output("kubectl_commands").AsString(ctx)
	if err != nil {
		return nil, err
	}

	// Step 2: Parse the JSON plan and execute kubectl commands dynamically
	type KubectlCommand struct {
		Operation    string `json:"operation"`
		ResourceKind string `json:"resourceKind"`
		Namespace    string `json:"namespace"`
		Description  string `json:"description"`
	}

	// Clean the JSON response - remove markdown code fences if present
	cleanJSON := strings.TrimSpace(commandsJSON)
	// Remove ```json and ``` markers
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var commands []KubectlCommand
	err = json.Unmarshal([]byte(cleanJSON), &commands)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubectl commands plan: %w\nRaw response: %s", err, commandsJSON)
	}

	// Execute each planned kubectl command and collect results
	clusterDataEnvironment := dag.Env().
		WithStringInput("user_request", promptScope, "the user's original request").
		WithStringInput("namespace", namespace, "optional namespace filter").
		WithStringInput("kubectl_plan", commandsJSON, "the planned kubectl commands").
		WithStringOutput("configuration", "the generated application configuration values")

	// Loop through each planned command and execute it
	for i, cmd := range commands {
		result, err := dag.Helm().TalkWithKubernetes(
			ctx,
			dagger.HelmTalkWithKubernetesOpts{
				Operation:    cmd.Operation,
				ResourceKind: cmd.ResourceKind,
				Namespace:    cmd.Namespace,
				KubeConfig:   kubeConfig,
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

Planned queries executed: $kubectl_plan

Query results from the cluster:`

	// Add references to each query result
	for i, cmd := range commands {
		envKey := fmt.Sprintf("query_%d_%s", i, cmd.ResourceKind)
		promptBuilder += fmt.Sprintf("\n- %s (%s): $%s", cmd.ResourceKind, cmd.Description, envKey)
	}

	promptBuilder += `

Analyze the cluster information and generate a response.

Generate output 'configuration' containing:
- Direct answer to the user's question
- List and description of relevant resources found (if they asked what's installed/running)
- YAML configuration if they requested configuration values
- Context and recommendations where helpful
- Explanation if resources weren't found or errors occurred

Format the output in a clear, structured way. Use YAML format if generating configuration values.
Be specific about what you found (or didn't find) in the cluster.
`

	configWork := dag.LLM().
		WithModel(model).
		WithEnv(clusterDataEnvironment).
		WithPrompt(promptBuilder)

	// Get the AI-generated configuration/response
	configuration, err := configWork.Env().Output("configuration").AsString(ctx)
	if err != nil {
		return nil, err
	}

	// Return as a file
	return dag.Directory().
		WithNewFile(outputFile, configuration).
		File(outputFile), nil
}

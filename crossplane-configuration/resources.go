package main

import (
	"context"
	"dagger/crossplane-configuration/internal/dagger"
	"fmt"
	"strconv"
	"strings"
)

func (m *CrossplaneConfiguration) AddCluster(
	ctx context.Context,
	// +optional
	// +default="ghcr.io/stuttgart-things/xplane-cluster-resources:0.2.1"
	module string,
	// +optional
	// +default="crossplane-system"
	crossplaneNamespace string,
	// +optional
	// +default="kubernetes,helm"
	providers string,
	clusterName string,
	// +optional
	parametersFile *dagger.File,
	// Kubeconfig secret to create secret from
	kubeconfigCluster *dagger.Secret,
	// Kubeconfig secret crossplane cluster
	kubeconfigCrossplaneCluster *dagger.Secret,
	// +optional
	// +default="true"
	useClusterProviderConfig bool,
) *dagger.File {

	// CHECK IF SECRET EXISTS AND DELETE IF NEEDED
	secretExists, err := dag.Kubernetes().
		CheckResourceStatus(
			ctx,
			"secret",
			clusterName,
			crossplaneNamespace,
			kubeconfigCrossplaneCluster,
		)

	if err != nil {
		panic(err)
	}

	if secretExists {

		deletionStatus, err := dag.Kubernetes().
			Command(
				ctx,
				dagger.KubernetesCommandOpts{
					Operation:    "delete",
					ResourceKind: "secret " + clusterName,
					Namespace:    crossplaneNamespace,
					KubeConfig:   kubeconfigCrossplaneCluster,
				},
			)

		if err != nil {
			panic(err)
		}

		fmt.Println("Existing Kubeconfig Secret Deleted: ", deletionStatus)

	}

	// CREATE SECRET
	status, err := dag.Kubernetes().
		CreateKubeconfigSecret(
			ctx,
			kubeconfigCluster,
			dagger.KubernetesCreateKubeconfigSecretOpts{
				Namespace:         crossplaneNamespace,
				SecretName:        clusterName,
				KubeConfigCluster: kubeconfigCrossplaneCluster,
			},
		)

	if err != nil {
		panic(err)
	}

	fmt.Println("Kubeconfig Secret Status: ", status)

	// READ SECRET KEY OF KUBECONFIG
	keyNameRaw, err := dag.Kubernetes().
		Command(
			ctx,
			dagger.KubernetesCommandOpts{
				Operation:         "get",
				ResourceKind:      "secret " + clusterName + " -o json",
				Namespace:         crossplaneNamespace,
				KubeConfig:        kubeconfigCrossplaneCluster,
				AdditionalCommand: "jq -r '.data | keys[0]'",
			})

	if err != nil {
		panic(err)
	}

	keyName := strings.TrimSpace(keyNameRaw)
	fmt.Println("Kubeconfig Secret Key Name: ", keyName)

	// LOOP THROUGH PROVIDERS
	providerList := strings.Split(providers, ",")
	var configFiles []*dagger.File

	for _, provider := range providerList {
		provider = strings.TrimSpace(provider)
		fmt.Println("\n=== Processing Provider: ", provider, " ===")

		// BUILD RENDER PARAMETERS
		renderParameter := "clusterName=" + clusterName + "," +
			"credNamespace=" + crossplaneNamespace + "," +
			"credSecretName=" + clusterName + "," +
			"credKey=" + keyName + "," +
			"providerType=" + provider + "," +
			"useClusterProviderConfig=" + strconv.FormatBool(useClusterProviderConfig)

		fmt.Println("Render Parameters: ", renderParameter)

		// RENDER CONFIG
		configFile := dag.Kcl().
			Run(
				dagger.KclRunOpts{
					Source:         nil,
					OciSource:      module,
					Parameters:     renderParameter,
					ParametersFile: parametersFile,
					FormatOutput:   true,
					OutputFormat:   "yaml",
					Entrypoint:     "main.k",
				})

		// APPLY RENDERED CONFIG TO CLUSTER
		applyStatus, err := dag.Kubernetes().Kubectl(
			ctx,
			dagger.KubernetesKubectlOpts{
				Operation:       "apply",                     // kubectl operation
				SourceFile:      configFile,                  // your rendered YAML from KCL
				URLSource:       "",                          // not used here
				KustomizeSource: "",                          // not used here
				Namespace:       crossplaneNamespace,         // namespace to apply into
				KubeConfig:      kubeconfigCrossplaneCluster, // kubeconfig secret
				ServerSide:      false,                       // set true if you want --server-side
				AdditionalFlags: "",                          // e.g., "--dry-run=client -o yaml" if needed
			},
		)
		if err != nil {
			panic(err)
		}

		fmt.Println("Applied Cluster Resources for "+provider+": ", applyStatus)

		// Store config file for merging
		configFiles = append(configFiles, configFile)
	}

	// MERGE ALL CONFIG FILES
	mergedConfigFile := dag.Container().
		From("alpine:latest").
		WithExec([]string{"sh", "-c", "echo '# Merged ProviderConfigs' > /merged.yaml"}).
		WithoutEntrypoint()

	for i, cf := range configFiles {
		mergedConfigFile = mergedConfigFile.
			WithMountedFile(fmt.Sprintf("/config-%d.yaml", i), cf).
			WithExec([]string{"sh", "-c", fmt.Sprintf("cat /config-%d.yaml >> /merged.yaml", i)})
	}

	return mergedConfigFile.File("/merged.yaml")
}

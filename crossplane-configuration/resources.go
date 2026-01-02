package main

import (
	"context"
	"dagger/crossplane-configuration/internal/dagger"
	"fmt"
)

func (m *CrossplaneConfiguration) AddCluster(
	ctx context.Context,
	// +optional
	// +default="ghcr.io/stuttgart-things/xplane-cluster-resources:0.2.1"
	module string,
	// +optional
	// +default="crossplane-system"
	crossplaneNamespace string,
	clusterName string,
	// +optional
	parametersFile *dagger.File,
	// +optional
	// +default="clusterName=kubernetes-provider"
	parameters string,
	// Kubeconfig secret to create secret from
	kubeconfigCluster *dagger.Secret,
	// Kubeconfig secret crossplane cluster
	kubeconfigCrossplaneCluster *dagger.Secret,
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
	keyName, err := dag.Kubernetes().
		Command(
			ctx,
			dagger.KubernetesCommandOpts{
				Operation:         "get",
				ResourceKind:      "secret " + clusterName,
				Namespace:         crossplaneNamespace,
				KubeConfig:        kubeconfigCrossplaneCluster,
				AdditionalCommand: "",
			})

	if err != nil {
		panic(err)
	}

	fmt.Println("Kubeconfig Secret Status: ", keyName)

	// SHOULD RUN IN LOOP

	// RENDER CONFIG
	configFile := dag.Kcl().
		Run(
			dagger.KclRunOpts{
				Source:         nil,
				OciSource:      module,
				Parameters:     parameters,
				ParametersFile: parametersFile,
				FormatOutput:   true,
				OutputFormat:   "yaml",
				Entrypoint:     "main.k",
			})

		// APPLY RENDERED CONFIG TO CLUSTER
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

	fmt.Println("Applied Cluster Resources: ", applyStatus)

	return configFile
}

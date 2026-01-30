package main

import (
	"context"
	"dagger/crossplane-configuration/internal/dagger"
	"dagger/crossplane-configuration/templates"
	"encoding/base64"
	"encoding/json"
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
	// +optional
	// Kubeconfig secret crossplane cluster
	kubeconfigCrossplaneCluster *dagger.Secret,
	// +optional
	// +default="true"
	useClusterProviderConfig bool,
	// +optional
	// +default="true"
	deployToCluster bool,
	// +optional
	// +default="false"
	encryptWithSops bool,
	// +optional
	// AGE public key for SOPS encryption
	agePublicKey *dagger.Secret,
	// +optional
	// SOPS config file (.sops.yaml)
	sopsConfig *dagger.File,
) *dagger.File {

	// RENDER KUBECONFIG SECRET
	secretFile, err := m.RenderKubeconfigSecret(
		ctx,
		kubeconfigCluster,
		clusterName,
		crossplaneNamespace,
		"config",
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("Kubeconfig Secret Rendered Successfully")

	// CHECK IF SECRET EXISTS AND DELETE IF NEEDED (only if deploying)
	if deployToCluster {
		secretExists, err := dag.Kubernetes().
			CheckResourceStatus(
				ctx,
				"secret",
				clusterName,
				crossplaneNamespace,
				kubeconfigCrossplaneCluster,
			)

		// If error, assume secret doesn't exist (continue anyway)
		if err == nil && secretExists {

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
		} else if err != nil {
			fmt.Println("Secret doesn't exist yet (CheckResourceStatus error), will create new one: ", err)
		}

		// APPLY SECRET FILE
		secretApplyStatus, err := dag.Kubernetes().Kubectl(
			ctx,
			dagger.KubernetesKubectlOpts{
				Operation:       "apply",
				SourceFile:      secretFile,
				URLSource:       "",
				KustomizeSource: "",
				Namespace:       crossplaneNamespace,
				KubeConfig:      kubeconfigCrossplaneCluster,
				ServerSide:      false,
				AdditionalFlags: "",
			},
		)

		if err != nil {
			panic(err)
		}

		fmt.Println("Kubeconfig Secret Applied: ", secretApplyStatus)
	} else {
		fmt.Println("Skipping secret deployment (deployToCluster=false)")
	}

	// SET SECRET KEY (using hardcoded "config" since we rendered with it)
	keyName := "config"
	fmt.Println("Kubeconfig Secret Key Name: ", keyName)

	// LOOP THROUGH PROVIDERS
	providerList := strings.Split(providers, ",")
	var configFiles []*dagger.File

	// Add secret file to the beginning of config files for merging
	configFiles = append(configFiles, secretFile)

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

		// APPLY RENDERED CONFIG TO CLUSTER (only if deploying)
		if deployToCluster {
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
		} else {
			fmt.Println("Skipping application of " + provider + " (deployToCluster=false)")
		}

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

	mergedFile := mergedConfigFile.File("/merged.yaml")

	// ENCRYPT WITH SOPS (if enabled)
	if encryptWithSops {
		encryptedFile := dag.Sops().Encrypt(
			agePublicKey,
			mergedFile,
			dagger.SopsEncryptOpts{
				FileExtension: "yaml",
				SopsConfig:    sopsConfig,
			},
		)

		fmt.Println("Merged config encrypted with SOPS")
		return encryptedFile
	}

	return mergedFile
}

// RenderKubeconfigSecret renders a Kubernetes Secret manifest with encoded kubeconfig
// Input: kubeconfig file
// Parameters: secretName, secretNamespace, secretKey
// Output: rendered secret file
func (m *CrossplaneConfiguration) RenderKubeconfigSecret(
	ctx context.Context,
	kubeconfigFile *dagger.Secret,
	secretName string,
	secretNamespace string,
	secretKey string,
) (*dagger.File, error) {
	// Read kubeconfig secret content
	kubeconfigContent, err := kubeconfigFile.Plaintext(ctx)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig secret: %w", err)
	}

	// Encode kubeconfig to base64
	kubeconfigBase64 := base64.StdEncoding.EncodeToString([]byte(kubeconfigContent))

	// Create data map for template rendering
	data := map[string]interface{}{
		"secretName":       secretName,
		"secretNamespace":  secretNamespace,
		"secretKey":        secretKey,
		"kubeconfigBase64": kubeconfigBase64,
	}

	// Marshal data to JSON for templating
	varsJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal template variables: %w", err)
	}

	// Render the KubeconfigSecret template
	rendered, err := dag.Templating().RenderInline(
		ctx,
		templates.KubeconfigSecret,
		dagger.TemplatingRenderInlineOpts{
			Variables:  string(varsJSON),
			StrictMode: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("render kubeconfig secret template: %w", err)
	}

	// Create and return the secret file
	secretFile := dag.Container().
		From("alpine:latest").
		WithNewFile("/secret.yaml", rendered).
		WithoutEntrypoint().
		File("/secret.yaml")

	return secretFile, nil
}

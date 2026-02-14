/*
Copyright © 2025 Patrick Hermann patrick.hermann@sva.de
*/

package templates

// FunctionPackage represents the details of a Crossplane function package
type FunctionPackage struct {
	Name       string
	PackageURL string
	Version    string
	ApiVersion string
}

type TemplateDestination struct {
	Template    string
	Destination string
}

var PackageFiles = []TemplateDestination{
	{
		Template:    Example,
		Destination: "examples/{name}.yaml",
	},
	{
		Template:    Functions,
		Destination: "examples/functions.yaml",
	},
	{
		Template:    Composition,
		Destination: "compositions/{name}.yaml",
	},
	{
		Template:    Readme,
		Destination: "README.md",
	},
	{
		Template:    Definition,
		Destination: "apis/definition.yaml",
	},
	{
		Template:    Configuration,
		Destination: "crossplane.yaml",
	},
	{
		Template:    ProviderConfig,
		Destination: "examples/provider-config.yaml",
	},
}

var Example = `---
apiVersion: {{ .apiGroup }}/v1alpha1
kind: {{ .kind }}
metadata:
  name: {{ .name }}
spec:
  targetCluster: cluster-name
`

var Functions = `---{{- range .functions }}
apiVersion: {{ .ApiVersion }}
kind: Function
metadata:
  name: {{ .Name }}
spec:
  package: {{ .PackageURL }}:{{ .Version }}
---
{{- end }}
`

var Composition = `---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  labels:
    crossplane.io/xrd: {{ .xrdPlural }}.{{ .apiGroup }}
  name: {{ .name }}
spec:
  compositeTypeRef:
    apiVersion: {{ .apiGroup }}/v1alpha1
    kind: {{ .kind }}
  mode: Pipeline
  pipeline:
    - step: render-templates
      functionRef:
        name: function-go-templating
      input:
        apiVersion: gotemplating.fn.crossplane.io/v1beta1
        kind: GoTemplate
        source: Inline
        inline:
          template: |
            # Add your go-templating resources here
    - step: automatically-detect-readiness
      functionRef:
        name: function-auto-ready
`

var Definition = `---
apiVersion: apiextensions.crossplane.io/v2
kind: CompositeResourceDefinition
metadata:
  name: {{ .xrdPlural }}.{{ .apiGroup}}
spec:
  group: {{ .apiGroup }}
  defaultCompositeDeletePolicy: {{ .xrdDeletePolicy }}
  scope: {{ .xrdScope }}
  names:
    kind: {{ .kind }}
    plural: {{ .xrdPlural }}
    singular: {{ .xrdSingular }}
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                targetCluster:
                  type: string
              required:
                - targetCluster
            status:
              type: object
              properties:
                conditions:
                  type: array
                  items:
                    type: object
                    properties:
                      type:
                        type: string
                      status:
                        type: string
                      reason:
                        type: string
                      message:
                        type: string
                      lastTransitionTime:
                        type: string
                        format: date-time
                    required:
                      - type
                      - status
`

var Configuration = `---
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: {{ .kind }}
  annotations:
    meta.crossplane.io/maintainer: {{ .maintainer }}
    meta.crossplane.io/source: {{ .source }}
    meta.crossplane.io/license: {{ .license }}
    meta.crossplane.io/description: |
      manages lifecycle of {{ .kind }} w/ crossplane
    meta.crossplane.io/readme: |
      manages lifecycle of {{ .kind }} w/ crossplane
spec:
  crossplane:
    version: ">={{ .crossplaneVersion }}"
  dependsOn:
    {{- range .dependencies }}
    - provider: {{ .provider }}
      version: "{{ .version }}"
    {{- end }}
`

var Readme = "# {{ .name }}\n\nThis Crossplane Configuration provisions a `{{ .kind }}` Composite Resource Definition (XRD) along with a Composition and an example.\n\n## DEV\n\n```bash\ncrossplane render examples/{{ .name }}.yaml \\\ncompositions/{{ .name }}.yaml \\\nexamples/functions.yaml \\\n--include-function-results\n```\n\n"

var ProviderConfig = `---
apiVersion: helm.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: provider-config-helm
spec:
  credentials:
    source: InjectedIdentity
`

var KubeconfigSecret = `---
apiVersion: v1
kind: Secret
metadata:
  name: {{ .secretName }}
  namespace: {{ .secretNamespace }}
data:
  {{ .secretKey }}: {{ .kubeconfigBase64 }}
type: Opaque
`

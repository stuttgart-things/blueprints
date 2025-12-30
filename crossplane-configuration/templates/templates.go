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
		Template:    Claim,
		Destination: "examples/claim.yaml",
	},
	{
		Template:    Functions,
		Destination: "examples/functions.yaml",
	},
	{
		Template:    Composition,
		Destination: "apis/composition.yaml",
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
}

var Claim = `---
apiVersion: {{ .apiGroup }}/{{ .apiVersion }}
kind: {{ .kind }}
metadata:
  name: {{ .claimName }}
  namespace: {{ .claimNamespace }}
spec:
  # add spec fields here
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
apiVersion: {{ .compositionApiVersion }}
kind: Composition
metadata:
  labels:
    crossplane.io/xrd: {{ .xrdPlural }}.{{ .apiGroup }}
  name: {{ .name }}
spec:
  compositeTypeRef:
    apiVersion: {{ .apiGroup }}/{{ .claimApiVersion }}
    kind: {{ .kind }}
  #pipeline:
  #  - step: <REPLACE_ME>
  #    functionRef:
  #      name: function-go-templating
  #    input:
  #      apiVersion: gotemplating.fn.crossplane.io/v1beta1
  #      kind: GoTemplate
  #      source: Inline
  #      inline:
  #        template: |
  #          apiVersion: <REPLACE_ME>
  #          kind: <REPLACE_ME>
  #          metadata:
  #            annotations:
  #              gotemplating.fn.crossplane.io/composition-resource-name: $CLAIMNAME
  #              gotemplating.fn.crossplane.io/ready: "True"
  #  - step: <REPLACE_ME>
  #    functionRef:
  #      name: function-patch-and-transform
  #    input:
  #      apiVersion: pt.fn.crossplane.io/v1beta1
  #      environment: null
  #      kind: Resources
  #      patchSets: []
  #      resources:
  #        - name: <REPLACE_ME>
  #          base:
  #            apiVersion: <REPLACE_ME>
  #            kind: <REPLACE_ME>
  #          patches: {}
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
      # add spec fields here
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

var Readme = "# {{ .claimKind }}\n\nThis Crossplane Configuration provisions a `{{ .kind }}` Composite Resource Definition (XRD) along with a Composition and an example Claim.\n\n## DEV\n\n```bash\ncrossplane render examples/claim.yaml \\\napis/composition.yaml \\\nexamples/functions.yaml \\\n--include-function-results\n```\n\n"

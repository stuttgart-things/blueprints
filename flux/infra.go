package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"dagger/flux/internal/dagger"

	"gopkg.in/yaml.v3"
)

// InfraValues is the values.yaml consumed by BootstrapInfra.
//
//	source:
//	  enabled: true
//	  name: flux-infra
//	  url: https://github.com/stuttgart-things/flux.git
//	  tag: v1.24.1
//	components:
//	  openebs:
//	    enabled: true
//	    templateName: openebs
//	    params:
//	      openebsVersion: "4.5.1"
//	  cilium-lb:
//	    enabled: false
//	    templateName: infrastructure
//	    params:
//	      path: ./infra/cilium/components/lb
//	      substitute: CILIUM_LB_IP_START=10.0.0.1,CILIUM_LB_IP_STOP=10.0.0.1
type InfraValues struct {
	Source     *InfraSource              `yaml:"source"`
	Components map[string]InfraComponent `yaml:"components"`
}

// InfraSource describes the GitRepository the Kustomizations pull from.
type InfraSource struct {
	Enabled  *bool  `yaml:"enabled"`
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Tag      string `yaml:"tag"`
	Branch   string `yaml:"branch"`
	Interval string `yaml:"interval"`
}

// InfraComponent is one Flux Kustomization to render.
type InfraComponent struct {
	Enabled      *bool             `yaml:"enabled"`
	TemplateName string            `yaml:"templateName"`
	Params       map[string]string `yaml:"params"`
}

// enabled defaults to false when the key is absent, so a component has to be
// switched on deliberately rather than by forgetting to switch it off.
func enabled(b *bool) bool { return b != nil && *b }

// RenderInfra renders the infrastructure Kustomizations and returns them as a
// directory, without touching git or a cluster.
//
// BootstrapInfra returns only a report, so this is the way to see what would
// actually be committed -- `dagger call render-infra --values-file v.yaml
// export --path ./out` puts the manifests on disk for review.
func (m *Flux) RenderInfra(
	ctx context.Context,
	// values.yaml describing the source and the components
	valuesFile *dagger.File,
	// OCI KCL module rendering the Kustomizations
	// +optional
	// +default="ghcr.io/stuttgart-things/claim-flux-kustomizations:0.3.34"
	ociSource string,
	// KCL entrypoint file name
	// +optional
	// +default="main.k"
	entrypoint string,
	// Namespace the Kustomizations live in
	// +optional
	// +default="flux-system"
	namespace string,
) (*dagger.Directory, error) {
	res, err := renderInfraSet(ctx, valuesFile, ociSource, entrypoint, namespace)
	if err != nil {
		return nil, err
	}
	return res.dir, nil
}

// infraRenderResult is what one pass over the values file produced.
type infraRenderResult struct {
	dir     *dagger.Directory
	wanted  []string
	skipped []string
	report  string
	count   int
}

// BootstrapInfra renders infrastructure Kustomizations from a values file,
// commits them, and verifies that each one actually became Ready on the
// cluster.
//
// The rendering is done by the claim-flux-kustomizations KCL module, the same
// one whose output carries the `managed-by: kcl-flux-kustomizations` annotation
// in the cluster repositories -- this function drives it instead of the files
// being written by hand.
func (m *Flux) BootstrapInfra(
	ctx context.Context,
	// values.yaml describing the source and the components
	valuesFile *dagger.File,
	// OCI KCL module rendering the Kustomizations
	// +optional
	// +default="ghcr.io/stuttgart-things/claim-flux-kustomizations:0.3.34"
	ociSource string,
	// KCL entrypoint file name
	// +optional
	// +default="main.k"
	entrypoint string,
	// Target repository in "owner/repo" format
	// +optional
	repository string,
	// Branch to commit to
	// +optional
	// +default="main"
	branchName string,
	// Destination path within the repository (the cluster directory)
	// +optional
	// +default="clusters/"
	destinationPath string,
	// GitHub token for the commit
	// +optional
	gitToken *dagger.Secret,
	// Kubeconfig of the target cluster, required for verification
	// +optional
	kubeConfig *dagger.Secret,
	// Namespace the Kustomizations live in
	// +optional
	// +default="flux-system"
	namespace string,
	// Commit the rendered manifests
	// +optional
	// +default=true
	commitToGit bool,
	// Wait until every enabled component reports Ready
	// +optional
	// +default=true
	verify bool,
	// How long to wait for all components together
	// +optional
	// +default="10m"
	verifyTimeout string,
	// Flux CLI image used for verification
	// +optional
	// +default="ghcr.io/fluxcd/flux-cli:v2.9.4"
	fluxCliImage string,
) (string, error) {
	res, err := renderInfraSet(ctx, valuesFile, ociSource, entrypoint, namespace)
	if err != nil {
		return "", err
	}
	commitDir, wanted, skipped := res.dir, res.wanted, res.skipped
	var report strings.Builder
	report.WriteString(res.report)

	// --- commit -------------------------------------------------------------
	if commitToGit {
		if repository == "" || gitToken == nil {
			return "", fmt.Errorf("bootstrap-infra: commitToGit needs repository and gitToken")
		}
		msg := fmt.Sprintf("Add rendered Flux infrastructure kustomizations (%s)", strings.Join(wanted, ", "))
		if _, err := dag.Git().AddFolderToGithubBranch(
			ctx, repository, branchName, msg, gitToken, commitDir, destinationPath,
		); err != nil {
			if strings.Contains(err.Error(), "no changes to commit") {
				report.WriteString(fmt.Sprintf("  commit   no changes (%s already up-to-date)\n", repository))
			} else {
				return "", fmt.Errorf("bootstrap-infra: commit: %w", err)
			}
		} else {
			report.WriteString(fmt.Sprintf("  commit   %s@%s:%s\n", repository, branchName, destinationPath))
		}
	}

	// --- verify -------------------------------------------------------------
	if verify {
		if kubeConfig == nil {
			return "", fmt.Errorf("bootstrap-infra: verify needs a kubeConfig")
		}
		pending, err := waitForKustomizations(ctx, wanted, namespace, kubeConfig, fluxCliImage, verifyTimeout)
		if err != nil {
			return "", fmt.Errorf("bootstrap-infra: verify: %w", err)
		}
		if len(pending) > 0 {
			return report.String(), fmt.Errorf(
				"bootstrap-infra: not Ready within %s: %s", verifyTimeout, strings.Join(pending, ", "))
		}
		for _, name := range wanted {
			report.WriteString(fmt.Sprintf("  ready    %-24s Ready=True\n", name))
		}
	}

	summary := fmt.Sprintf("%d component(s) rendered, %d skipped\n", len(wanted), len(skipped))
	return summary + report.String(), nil
}

// renderInfraSet does one pass over the values file: it renders the source and
// every enabled component into a directory laid out the way a cluster
// repository expects (sources/, infra/).
func renderInfraSet(
	ctx context.Context,
	valuesFile *dagger.File,
	ociSource, entrypoint, namespace string,
) (*infraRenderResult, error) {
	raw, err := valuesFile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("bootstrap-infra: read values: %w", err)
	}

	var values InfraValues
	if err := yaml.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("bootstrap-infra: parse values: %w", err)
	}

	// Sorted so the rendered set, the commit and the log are reproducible;
	// Go map iteration order is deliberately random.
	names := make([]string, 0, len(values.Components))
	for name := range values.Components {
		names = append(names, name)
	}
	sort.Strings(names)

	var (
		report   strings.Builder
		wanted   []string
		skipped  []string
		renderCt int
	)
	commitDir := dag.Directory()

	// --- source -------------------------------------------------------------
	if values.Source != nil && enabled(values.Source.Enabled) {
		src := values.Source
		params := map[string]string{
			"templateName": "gitrepository",
			"name":         src.Name,
			"url":          src.URL,
		}
		if src.Tag != "" {
			params["tag"] = src.Tag
		}
		if src.Branch != "" {
			params["branch"] = src.Branch
		}
		if src.Interval != "" {
			params["interval"] = src.Interval
		}

		rendered, err := renderKustomization(ctx, ociSource, entrypoint, params)
		if err != nil {
			return nil, fmt.Errorf("bootstrap-infra: render source %q: %w", src.Name, err)
		}
		commitDir = commitDir.WithNewFile(fmt.Sprintf("sources/%s.yaml", src.Name), rendered)
		renderCt++
		report.WriteString(fmt.Sprintf("  source   %-24s rendered\n", src.Name))
	}

	// --- components ---------------------------------------------------------
	for _, name := range names {
		comp := values.Components[name]
		if !enabled(comp.Enabled) {
			skipped = append(skipped, name)
			report.WriteString(fmt.Sprintf("  skipped  %-24s (enabled: false)\n", name))
			continue
		}

		template := comp.TemplateName
		if template == "" {
			template = name
		}

		params := map[string]string{
			"templateName": template,
			"name":         name,
			"namespace":    namespace,
		}
		for k, v := range comp.Params {
			params[k] = v
		}

		rendered, err := renderKustomization(ctx, ociSource, entrypoint, params)
		if err != nil {
			return nil, fmt.Errorf("bootstrap-infra: render %q: %w", name, err)
		}
		commitDir = commitDir.WithNewFile(fmt.Sprintf("infra/%s.yaml", name), rendered)
		wanted = append(wanted, name)
		renderCt++
		report.WriteString(fmt.Sprintf("  rendered %-24s template=%s\n", name, template))
	}

	if renderCt == 0 {
		return nil, fmt.Errorf("bootstrap-infra: nothing enabled in the values file")
	}

	return &infraRenderResult{
		dir:     commitDir,
		wanted:  wanted,
		skipped: skipped,
		report:  report.String(),
		count:   renderCt,
	}, nil
}

// renderKustomization turns one parameter set into a Kustomization manifest.
func renderKustomization(
	ctx context.Context,
	ociSource, entrypoint string,
	params map[string]string,
) (string, error) {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Parameters travel as a FILE, not as the comma-separated --parameters
	// string. That string cannot carry `substitute` at all: its value is itself
	// a comma-separated list, so joining it with the other parameters destroys
	// the boundary before the KCL module ever sees it, and the module's own
	// split then keeps only the first pair.
	//
	// It failed silently, which is the worst part -- the render succeeded and
	// produced a valid-looking Kustomization. Measured on test-infra1
	// 2026-08-24: eight substitute variables went in, `CERT_MANAGER_NAMESPACE`
	// came out, and the wildcard certificate would have been issued for the
	// sentinel `set-INFRA_DOMAIN.invalid`.
	//
	// A YAML document has no such ambiguity, and `--parameters-file` is the
	// channel the KCL module offers for exactly this.
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range keys {
		b.WriteString(k + ": " + strconv.Quote(params[k]) + "\n")
	}

	paramsFile := dag.Directory().
		WithNewFile("parameters.yaml", b.String()).
		File("parameters.yaml")

	// FormatOutput is left at its default. It used to corrupt this module's
	// output -- the post-processor assumed an `items:` list and its guard only
	// looked for a leading `---`, so a single document starting with
	// `apiVersion:` lost that line to `sed '1d'` and had its nesting flattened
	// by `sed 's/^  //'`. Fixed in dagger/kcl v0.126.1, which is what the
	// dependency is pinned to. Setting it to false here would not have helped
	// anyway: the generated client drops optional arguments that equal the
	// zero value, so `FormatOutput: false` is never transmitted.
	return dag.Kcl().Run(dagger.KclRunOpts{
		OciSource:      ociSource,
		ParametersFile: paramsFile,
		Entrypoint:     entrypoint,
	}).Contents(ctx)
}

// waitForKustomizations polls until every name reports Ready, and returns those
// that did not make it before the deadline.
//
// It asks the cluster by name rather than running `flux check`: a check only
// says Flux itself is healthy, which stays true even when a Kustomization was
// never created at all.
//
// Each poll carries a timestamp so the exec cannot be served from the dagger
// cache. Without it an unchanged command over an unchanged container would
// replay an earlier answer, and the whole point here is to read state that
// changes underneath us -- see stuttgart-things/blueprints#182 for the same
// trap in the apply path.
func waitForKustomizations(
	ctx context.Context,
	names []string,
	namespace string,
	kubeConfig *dagger.Secret,
	fluxCliImage string,
	timeout string,
) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}

	// Preflight. Without it an unusable kubeconfig is indistinguishable from
	// "nothing is ready yet": every per-name call fails, each failure is counted
	// as not-ready, and after the timeout the function blames all the
	// components. That happened on 2026-08-24 with a kubeconfig path that
	// simply did not exist -- the report named eight healthy Kustomizations as
	// pending, and the real message ("failed to read secret file ... no such
	// file or directory") was never surfaced.
	//
	// One unfiltered list call separates the two: if the cluster cannot be
	// reached at all, say so instead of waiting out the clock and then lying
	// about which components are at fault.
	if _, err := fluxCliContainer(fluxCliImage, kubeConfig).
		WithEnvVariable("CACHEBUST", fmt.Sprintf("%d", time.Now().UnixNano())).
		WithExec([]string{"flux", "get", "kustomization", "-n", namespace}).
		Stdout(ctx); err != nil {
		return nil, fmt.Errorf("cannot reach the cluster to verify (namespace %q): %w", namespace, err)
	}

	deadline := time.Now().Add(time.Duration(parseTimeout(timeout)) * time.Second)
	pending := append([]string(nil), names...)

	for {
		still := pending[:0:0]
		for _, name := range pending {
			out, err := fluxCliContainer(fluxCliImage, kubeConfig).
				WithEnvVariable("CACHEBUST", fmt.Sprintf("%d", time.Now().UnixNano())).
				WithExec([]string{
					"flux", "get", "kustomization", name,
					"-n", namespace,
					"--status-selector", "ready=true",
				}, dagger.ContainerWithExecOpts{Expect: dagger.ReturnTypeAny}).
				Stdout(ctx)
			if err != nil || !strings.Contains(out, name) {
				still = append(still, name)
			}
		}
		pending = still

		if len(pending) == 0 {
			return nil, nil
		}
		if time.Now().After(deadline) {
			return pending, nil
		}
		time.Sleep(10 * time.Second)
	}
}

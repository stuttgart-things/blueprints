package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"dagger/vm/internal/dagger"
)

const (
	// defaultHarvesterOciSource is the KCL module that renders the three
	// manifests a Harvester VM consists of: the image-backed root-disk PVC,
	// the cloud-init Secret and the KubeVirt VirtualMachine. Rendering all
	// three from one module is what makes the Crossplane detour unnecessary
	// for bootstrap.
	defaultHarvesterOciSource = "ghcr.io/stuttgart-things/harvester-vm:0.2.0"

	// harvesterManifestFile is the rendered manifest inside the returned
	// directory. Kept as an artefact so a failed apply can be inspected and
	// re-applied by hand.
	harvesterManifestFile = "harvester-vm.yaml"

	// stampedManifestPath is where the run-stamped copy of the manifest is
	// built. See stampManifest for why the stamp exists.
	stampedManifestPath = "/harvester-vm-stamped.yaml"
)

// vmiObservation is the slice of a KubeVirt VirtualMachineInstance we poll for.
// The IP lives on the VMI, not on the VirtualMachine, and is reported by the
// in-guest QEMU guest agent — so it only appears once the guest has booted,
// obtained a lease and started the agent.
type vmiObservation struct {
	Status struct {
		Phase      string `json:"phase"`
		Interfaces []struct {
			Name      string `json:"name"`
			IPAddress string `json:"ipAddress"`
		} `json:"interfaces"`
	} `json:"status"`
}

// BakeHarvester provisions a Harvester / KubeVirt VM straight through the
// Kubernetes API and then configures it with Ansible — no Crossplane, no
// OpenTofu, no control plane of any kind on the target side.
//
// This is the bootstrap counterpart to BakeLocal: same shape (provision, read
// the machine's address back, hand it to Ansible), but the provisioning step
// is "render manifests and apply them" instead of "terraform apply". It exists
// for the chicken-and-egg case — the first VM on a Harvester cluster, the one
// that will go on to run the Crossplane management cluster that provisions
// every VM after it.
//
// The pipeline:
//
//  1. render PVC + cloud-init Secret + VirtualMachine from the harvester-vm
//     KCL module (dagger/kcl)
//  2. kubectl apply them against Harvester (dagger/kubernetes)
//  3. poll the resulting VirtualMachineInstance until it is Running AND the
//     guest agent has reported an IP
//  4. run Ansible against that IP (dagger/ansible, via ExecuteAnsible)
//
// The returned directory carries the rendered manifests, the generated
// inventory and outputs.json ({"vm_ips": [...]}) — the same output contract
// BakeLocal uses, so downstream consumers do not care which of the two
// provisioned the machine.
//
// Note this is a one-shot, imperative provisioner: there is no reconcile loop
// and no drift correction. Updating or deleting the VM afterwards is kubectl's
// job (or Crossplane's, once the management cluster this VM bootstraps is up).
func (m *Vm) BakeHarvester(
	ctx context.Context,
	// Kubeconfig for the Harvester cluster the VM is created on.
	kubeConfig *dagger.Secret,
	// Name of the VM. Forced into the KCL render as `vmName` so the manifests
	// and the VMI we poll for can never drift apart.
	vmName string,
	// Namespace for PVC, Secret and VirtualMachine. Forced into the KCL render
	// as `namespace` for the same reason.
	// +optional
	// +default="default"
	namespace string,
	// OCI reference of the harvester-vm KCL module.
	// +optional
	// +default="ghcr.io/stuttgart-things/harvester-vm:0.2.0"
	ociSource string,
	// KCL parameters as a YAML file (imageId, storageClass, storage, cpuCores,
	// memory, networkName, cloudInitSshKey, cloudInitPassword, …).
	//
	// Prefer this over kclParameters for anything sensitive: the file is
	// mounted into the render container, whereas --kcl-parameters values become
	// operation arguments and are echoed by `dagger --progress plain`.
	// +optional
	kclParametersFile *dagger.File,
	// KCL parameters as comma-separated key=value pairs. Override the file.
	// Do not put credentials here — see kclParametersFile.
	// +optional
	kclParameters string,
	// SOPS-encrypted KCL parameters file. Decrypted in-memory and used instead
	// of kclParametersFile; the plaintext never becomes an operation argument.
	// +optional
	encryptedFile *dagger.File,
	// AGE key for decrypting encryptedFile.
	// +optional
	sopsKey *dagger.Secret,
	// Skip creating the target namespace. By default BakeHarvester creates it
	// (idempotently) first, because kubectl apply does not and a missing
	// namespace is the most common way a bootstrap run dies on line one.
	// +optional
	// +default=false
	skipNamespace bool,
	// Seconds to wait for the VM to report an IP. Generous by default: the
	// guest has to boot, install/start qemu-guest-agent and get a DHCP lease.
	// +optional
	// +default=900
	waitTimeout int,
	// Seconds between VMI polls.
	// +optional
	// +default=15
	waitInterval int,
	// Ansible playbooks to run against the VM, comma-separated. Empty skips
	// the Ansible stage entirely (render + apply + wait only).
	// +optional
	ansiblePlaybooks string,
	// +optional
	ansibleRequirementsFile *dagger.File,
	// +optional
	ansibleUser *dagger.Secret,
	// +optional
	ansiblePassword *dagger.Secret,
	// +optional
	ansibleParameters string,
	// Extra environment for the Ansible container, as a secret in dotenv
	// format (NAME=value per line), for playbooks using lookup('env', ...).
	// +optional
	envSecrets *dagger.Secret,
	// +optional
	vaultRoleID *dagger.Secret,
	// +optional
	vaultSecretID *dagger.Secret,
	// +optional
	vaultURL *dagger.Secret,
	// Seconds to wait after the IP appears before Ansible connects. The agent
	// reports an address slightly before sshd is reliably up.
	// +optional
	// +default=30
	ansibleWaitTimeout int,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	requirementsTemplate string,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	requirementsData string,
	// Inventory type: "simple" (default [all] group) or "cluster".
	// +optional
	// +default="simple"
	inventoryType string,
	// Any value that changes between runs — a timestamp, a CI run id. Forces a
	// fresh fetch of the remote Ansible requirements instead of a cached
	// render. The kubectl apply gets its own stamp regardless; see
	// stampManifest.
	// +optional
	// +default=""
	cacheBuster string,
) (*dagger.Directory, error) {

	if strings.TrimSpace(vmName) == "" {
		return nil, fmt.Errorf("vmName is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	paramsFile, err := m.resolveKclParametersFile(ctx, kclParametersFile, encryptedFile, sopsKey)
	if err != nil {
		return nil, err
	}

	// The KCL module's own fallbacks for the per-VM resource names are fixed
	// strings ("dev2-disk-0", "dev4"). Left alone, a second bootstrap run in
	// the same namespace would silently attach the first VM's block boot disk.
	// Derive them from vmName instead — but only where the caller has not set
	// them, so pointing at a restored disk stays possible.
	derived, err := m.harvesterNameDefaults(ctx, paramsFile, kclParameters, vmName)
	if err != nil {
		return nil, err
	}

	// RENDER THE THREE MANIFESTS.
	// vmName and namespace are appended last so they win over both the
	// parameters file (-Y) and any caller-supplied --kcl-parameters (-D is
	// applied left to right): what we apply is what we then poll for.
	renderParameters := appendKclParameters(
		kclParameters,
		append(derived,
			fmt.Sprintf("vmName=%s", vmName),
			fmt.Sprintf("namespace=%s", namespace),
		)...,
	)

	manifests, err := m.RenderHarvesterVm(ctx, ociSource, paramsFile, renderParameters, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("rendering harvester manifests failed: %w", err)
	}

	// CREATE THE TARGET NAMESPACE.
	// The kubernetes module wraps every Command in `|| true`, so an
	// AlreadyExists is swallowed and this is effectively create-if-missing.
	if !skipNamespace {
		if _, err := dag.Kubernetes().Command(ctx, dagger.KubernetesCommandOpts{
			Operation:    "create",
			ResourceKind: "namespace " + namespace,
			KubeConfig:   kubeConfig,
			// Deliberately no Namespace: this IS the namespace operation.
			IgnoreErrors: true,
		}); err != nil {
			return nil, fmt.Errorf("creating namespace %q failed: %w", namespace, err)
		}
	}

	// APPLY.
	// No Namespace option: the rendered manifests always carry an explicit
	// metadata.namespace, and kubectl rejects a -n that disagrees with it.
	//
	// Unlike Command, Kubectl surfaces real errors (no `|| true`), so a
	// rejected manifest fails the run here rather than silently later.
	stamped, err := m.stampManifest(ctx, manifests)
	if err != nil {
		return nil, fmt.Errorf("preparing manifests for apply failed: %w", err)
	}

	applyOut, err := dag.Kubernetes().Kubectl(ctx, dagger.KubernetesKubectlOpts{
		Operation:  "apply",
		SourceFile: stamped,
		KubeConfig: kubeConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("applying harvester manifests failed: %w", err)
	}
	fmt.Printf("APPLIED HARVESTER MANIFESTS:\n%s\n", applyOut)

	// WAIT FOR THE GUEST AGENT TO REPORT AN IP.
	ip, err := m.WaitForVmIp(ctx, kubeConfig, vmName, namespace, waitTimeout, waitInterval)
	if err != nil {
		return nil, err
	}

	// ASSEMBLE THE RESULT DIRECTORY.
	// Same contract as BakeLocal so downstream stages (e.g. a chained
	// workflow reading the "stage-outputs" artifact) do not have to care
	// which provisioner ran.
	stageOutputs, err := json.Marshal(map[string]any{"vm_ips": []string{ip}})
	if err != nil {
		return nil, fmt.Errorf("marshal stage outputs: %w", err)
	}

	result := dag.Directory().
		WithFile(harvesterManifestFile, manifests).
		WithNewFile("inventory.ini", simpleInventory(ip)).
		WithNewFile("outputs.json", string(stageOutputs))

	// ANSIBLE.
	if strings.TrimSpace(ansiblePlaybooks) == "" {
		return result, nil
	}

	// The agent reports an address a little before sshd accepts connections.
	time.Sleep(time.Duration(ansibleWaitTimeout) * time.Second)

	// hosts (not inventory) — ExecuteAnsible builds the inventory from it,
	// which is why inventory.ini above is an artefact rather than an input.
	ansibleSuccess, err := m.ExecuteAnsible(
		ctx,
		nil, // src
		ansiblePlaybooks,
		ansibleRequirementsFile,
		nil, // inventory
		ip,  // hosts
		ansibleParameters,
		nil, // parametersFile
		vaultRoleID,
		vaultSecretID,
		vaultURL,
		ansibleUser,
		ansiblePassword,
		envSecrets,
		requirementsTemplate,
		requirementsData,
		inventoryType,
		cacheBuster,
	)
	if err != nil {
		return nil, fmt.Errorf("running ansible against %s failed: %w", ip, err)
	}
	if !ansibleSuccess {
		return nil, fmt.Errorf("ansible execution against %s failed", ip)
	}

	return result, nil
}

// RenderHarvesterVm renders the PVC, cloud-init Secret and VirtualMachine from
// the harvester-vm KCL module into a single apply-ready multi-document YAML.
//
// Exported on its own so a run can be inspected before anything touches a
// cluster (`... render-harvester-vm ... | tee vm.yaml`), and so the render can
// be reused for GitOps-style flows.
//
// The KCL module's top-level value is an `items:` list; dagger/kcl's Run
// post-processor converts exactly that shape into multi-document YAML when
// formatOutput is on (its default), so the yq/awk splitting the module's README
// shows for the bare `kcl run` case is not needed here.
func (m *Vm) RenderHarvesterVm(
	ctx context.Context,
	// +optional
	// +default="ghcr.io/stuttgart-things/harvester-vm:0.2.0"
	ociSource string,
	// +optional
	kclParametersFile *dagger.File,
	// +optional
	kclParameters string,
	// +optional
	encryptedFile *dagger.File,
	// +optional
	sopsKey *dagger.Secret,
) (*dagger.File, error) {

	if ociSource == "" {
		ociSource = defaultHarvesterOciSource
	}

	paramsFile, err := m.resolveKclParametersFile(ctx, kclParametersFile, encryptedFile, sopsKey)
	if err != nil {
		return nil, err
	}

	// FormatOutput is left unset on purpose: the module defaults it to true,
	// and dagger treats a false bool in an opts struct as unset anyway.
	return dag.Kcl().Run(dagger.KclRunOpts{
		OciSource:      ociSource,
		ParametersFile: paramsFile,
		Parameters:     kclParameters,
	}), nil
}

// resolveKclParametersFile returns the parameters file to render with,
// decrypting a SOPS-encrypted one on the way if that is what the caller
// supplied.
func (m *Vm) resolveKclParametersFile(
	ctx context.Context,
	kclParametersFile *dagger.File,
	encryptedFile *dagger.File,
	sopsKey *dagger.Secret,
) (*dagger.File, error) {

	if encryptedFile == nil {
		return kclParametersFile, nil
	}
	if kclParametersFile != nil {
		return nil, fmt.Errorf("pass either kclParametersFile or encryptedFile, not both")
	}

	decrypted, err := dag.Secrets().Decrypt(ctx, sopsKey, encryptedFile)
	if err != nil {
		return nil, fmt.Errorf("decrypting sops parameters failed: %w", err)
	}

	ctr, err := m.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}

	// Mounted-and-copied rather than WithNewFile: the plaintext must not
	// become an operation argument, or it lands in the build log.
	const paramsPath = "/kcl-params.yaml"

	return withSecretFile(ctr, paramsPath, decrypted, "kcl-params").File(paramsPath), nil
}

// harvesterNameDefaults returns the KCL parameters that name this VM's own
// resources, for the ones the caller has not set in either input.
//
// The parameters are only inspected for which keys are present — no value from
// the (possibly decrypted) file is copied into a returned parameter, so nothing
// sensitive can reach an operation argument this way.
func (m *Vm) harvesterNameDefaults(
	ctx context.Context,
	paramsFile *dagger.File,
	inlineParameters string,
	vmName string,
) ([]string, error) {

	present := make(map[string]bool)
	for key := range parseStringParams(inlineParameters) {
		present[key] = true
	}

	if paramsFile != nil {
		contents, err := paramsFile.Contents(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to read kcl parameters file: %w", err)
		}

		fileParams := make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(contents), &fileParams); err != nil {
			return nil, fmt.Errorf("failed to parse kcl parameters file: %w", err)
		}

		for key := range fileParams {
			present[key] = true
		}
	}

	var derived []string
	if !present["pvcName"] {
		derived = append(derived, fmt.Sprintf("pvcName=%s-disk-0", vmName))
	}
	if !present["secretName"] {
		derived = append(derived, fmt.Sprintf("secretName=%s-cloud-init", vmName))
	}

	return derived, nil
}

// WaitForVmIp polls a KubeVirt VirtualMachineInstance until it is Running and
// the in-guest QEMU guest agent has reported an IP address, and returns that
// address.
//
// Two things this deliberately does NOT do:
//
// It does not use `kubectl wait`. The kubernetes module wraps every Command in
// `(... 2>&1) || true`, so a wait that times out would come back as a success
// carrying an error message — the run would sail on and point Ansible at
// nothing. Polling and enforcing the deadline here keeps the failure a failure.
//
// It does not issue the same kubectl call twice. Dagger caches an exec by the
// digest of its arguments and inputs, so an unchanged `kubectl get` would be
// served from cache and the loop would spin forever on the first (empty)
// answer. The per-attempt marker in additionalCommand is what keeps every poll
// a real call — it is a shell comment, so it changes the digest and nothing
// else.
func (m *Vm) WaitForVmIp(
	ctx context.Context,
	// Kubeconfig for the cluster running the VM.
	kubeConfig *dagger.Secret,
	// VM name (the VMI carries the same name as its VirtualMachine).
	vmName string,
	// +optional
	// +default="default"
	namespace string,
	// +optional
	// +default=900
	waitTimeout int,
	// +optional
	// +default=15
	waitInterval int,
) (string, error) {

	if strings.TrimSpace(vmName) == "" {
		return "", fmt.Errorf("vmName is required")
	}
	if namespace == "" {
		namespace = "default"
	}
	if waitTimeout <= 0 {
		waitTimeout = 900
	}
	if waitInterval <= 0 {
		waitInterval = 15
	}

	// A run marker, so polls from THIS call cannot be served from a previous
	// call's cache either.
	runMarker := time.Now().UTC().UnixNano()
	deadline := time.Now().Add(time.Duration(waitTimeout) * time.Second)

	lastPhase := "<no VMI yet>"

	for attempt := 1; ; attempt++ {
		// `-o json` rather than `-o jsonpath=...`: the kubernetes module
		// re-joins the split arguments into a shell string, and a jsonpath's
		// braces and brackets would be at the mercy of the shell. JSON also
		// gives us the phase for a useful timeout message.
		out, err := dag.Kubernetes().Command(ctx, dagger.KubernetesCommandOpts{
			Operation:         "get",
			ResourceKind:      fmt.Sprintf("vmi %s -o json", vmName),
			Namespace:         namespace,
			KubeConfig:        kubeConfig,
			AdditionalCommand: fmt.Sprintf("true # poll %d-%d", runMarker, attempt),
			IgnoreErrors:      true,
		})
		if err != nil {
			return "", fmt.Errorf("polling VMI %s/%s failed: %w", namespace, vmName, err)
		}

		phase, ip := parseVmiAddress(out)
		if phase != "" {
			lastPhase = phase
		}
		if ip != "" {
			fmt.Printf("VM %s/%s IS %s WITH IP %s\n", namespace, vmName, phase, ip)
			return ip, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf(
				"timed out after %ds waiting for an IP on VMI %s/%s (last observed phase: %s) — "+
					"the address is reported by the in-guest qemu-guest-agent, so check that "+
					"the cloud-init installed and started it and that the VM got a lease",
				waitTimeout, namespace, vmName, lastPhase)
		}

		fmt.Printf("WAITING FOR VM %s/%s (phase: %s, attempt %d)\n", namespace, vmName, lastPhase, attempt)
		time.Sleep(time.Duration(waitInterval) * time.Second)
	}
}

// parseVmiAddress pulls the phase and first reported IP out of a `kubectl get
// vmi -o json` response.
//
// The kubernetes module merges stderr into stdout, so while the VMI does not
// exist yet the output is a NotFound message rather than JSON. That is a normal
// state of the poll, not an error: both return values stay empty and the caller
// keeps waiting.
func parseVmiAddress(out string) (phase string, ip string) {
	// kubectl prints the JSON document with `{` at the start of a line, so
	// anchor on that rather than on the first brace anywhere — a warning line
	// carrying a brace would otherwise swallow the document.
	var doc string
	if trimmed := strings.TrimLeft(out, " \t\r\n"); strings.HasPrefix(trimmed, "{") {
		doc = trimmed
	} else if i := strings.Index(out, "\n{"); i >= 0 {
		doc = out[i+1:]
	}

	if doc == "" {
		return "", ""
	}

	var vmi vmiObservation
	if err := json.Unmarshal([]byte(doc), &vmi); err != nil {
		return "", ""
	}

	for _, iface := range vmi.Status.Interfaces {
		if iface.IPAddress != "" {
			return vmi.Status.Phase, iface.IPAddress
		}
	}

	return vmi.Status.Phase, ""
}

// stampManifest appends a comment line to the rendered manifest so each run
// applies a distinct file.
//
// Without it, a second BakeHarvester run with unchanged parameters produces a
// byte-identical manifest, and dagger serves the kubectl apply from cache —
// the cluster is never touched. That is invisible and benign right up until the
// VM was deleted out of band, when the "successful" apply creates nothing and
// the IP wait then runs its full timeout against a machine that does not exist.
//
// The stamp is passed as an environment variable and written inside the
// container: the manifest carries the cloud-init Secret, so its contents must
// never become an operation argument.
func (m *Vm) stampManifest(ctx context.Context, manifests *dagger.File) (*dagger.File, error) {
	ctr, err := m.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}

	return ctr.
		WithFile("/harvester-vm.yaml", manifests).
		WithEnvVariable("BAKE_RUN_STAMP", time.Now().UTC().Format(time.RFC3339Nano)).
		WithExec([]string{"sh", "-c",
			"cp /harvester-vm.yaml " + stampedManifestPath +
				" && echo \"# bake-harvester run: ${BAKE_RUN_STAMP}\" >> " + stampedManifestPath}).
		File(stampedManifestPath), nil
}

// appendKclParameters appends key=value pairs to a comma-separated KCL
// parameter string. KCL applies -D left to right, so later entries win.
func appendKclParameters(parameters string, extra ...string) string {
	parts := make([]string, 0, len(extra)+1)
	if strings.TrimSpace(parameters) != "" {
		parts = append(parts, strings.TrimSpace(parameters))
	}
	parts = append(parts, extra...)

	return strings.Join(parts, ",")
}

// simpleInventory renders the same [all] inventory ExecuteAnsible generates
// from a hosts string, so the exported artefact matches what actually ran.
func simpleInventory(hosts ...string) string {
	inventory := "[all]\n"
	for _, host := range hosts {
		inventory += host + "\n"
	}

	return inventory
}

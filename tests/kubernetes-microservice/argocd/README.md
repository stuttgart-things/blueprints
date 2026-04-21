# argocd-onboard-cluster test fixtures

End-to-end test for `kubernetes-microservice.argocd-onboard-cluster`.
Runs as `task test-argocd` from the repo root.

## What it does

1. Creates a throwaway kind cluster (`blueprints-argocd-test`) using
   [`kind-config.yaml`](kind-config.yaml) — this is the target cluster whose
   kubeconfig gets fed into the orchestrator.
2. Generates a fresh AGE key pair under `/tmp/blueprints/argocd/`.
3. Calls `argocd-onboard-cluster` with `--encrypt=true`, exporting both
   manifests to `/tmp/blueprints/argocd/out/`.
4. Verifies:
   - `xplane-test-cluster.yaml` is SOPS-encrypted (contains `sops:` footer
     and `ENC[AES256_GCM,…]` values).
   - `xplane-test-appproject.yaml` is plain YAML with the right `kind` and
     `metadata.name`.
   - The Secret round-trips through `sops -d` back to a valid `Secret`
     manifest with `argocd.argoproj.io/secret-type=cluster`.
5. Deletes the kind cluster.

Skip the teardown by setting `KEEP_KIND=1` (useful for debugging a
failed assertion):

```bash
KEEP_KIND=1 task test-argocd
```

## Files

- [kind-config.yaml](kind-config.yaml) — single-node kind cluster config.

Everything else (kubeconfig, AGE keys, output manifests) is generated at
test time under `/tmp/blueprints/argocd/` and is not committed.

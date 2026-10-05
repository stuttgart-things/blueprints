package clusterapps

import "testing"

func TestAllowlistSecretKeywords(t *testing.T) {
	in := `spec:
  patches:
  - patch: |
      - op: replace
        secret: stays-in-the-block
    target:
      kind: Kustomization
  postBuild:
    substitute:
      INFRA_TLS_SECRET: wildcard-tls
      CERT_MANAGER_CA_FROM_SECRET_NAME: edge-ca
      MINIO_STORAGE_SIZE: '5Gi'
      DB_PASSWORD_KEY: x # already commented
  decryption:
    secretRef:
      name: sops-age
`
	want := `spec:
  patches:
  - patch: |
      - op: replace
        secret: stays-in-the-block
    target:
      kind: Kustomization
  postBuild:
    substitute:
      INFRA_TLS_SECRET: wildcard-tls ` + PragmaAllowlist + `
      CERT_MANAGER_CA_FROM_SECRET_NAME: edge-ca ` + PragmaAllowlist + `
      MINIO_STORAGE_SIZE: '5Gi'
      DB_PASSWORD_KEY: x # already commented
  decryption:
    secretRef:
      name: sops-age
`
	if got := AllowlistSecretKeywords(in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := AllowlistSecretKeywords(want); got != want {
		t.Errorf("not idempotent:\n%s", got)
	}
}

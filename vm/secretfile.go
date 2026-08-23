package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"

	"dagger/vm/internal/dagger"
)

// secretMountDir is outside any working directory so a mounted secret can never
// shadow a real file or be picked up by Container.Directory().
const secretMountDir = "/run/dagger-secrets"

// withSecretFile writes content to dstPath inside ctr without ever passing the
// plaintext as an operation argument.
//
// WithNewFile takes its contents as a plain Go string, and dagger records that
// argument in the operation graph — so the value shows up verbatim in build
// logs and in Dagger Cloud traces. Mounting the value as a secret and copying
// it into place keeps the plaintext out of every argument: the exec only ever
// sees paths.
//
// The copy is a real filesystem write, not a mount, so the file is part of the
// resulting snapshot and Container.Directory() picks it up — which a mounted
// secret on its own would not do.
//
// The digest in the command is a cache key, not decoration. Whether a mounted
// secret participates in the exec cache key is a dagger implementation detail;
// without something content-derived in the arguments, an unchanged command over
// an unchanged parent filesystem could be served from cache and quietly keep a
// stale file after the underlying secret was rotated. Hashing the whole
// decrypted blob rather than any single field keeps the digest high-entropy, so
// it is not a practical route back to the plaintext.
func withSecretFile(ctr *dagger.Container, dstPath, content, secretName string) *dagger.Container {
	mountPath := path.Join(secretMountDir, secretName)
	digest := sha256.Sum256([]byte(content))

	return ctr.
		WithMountedSecret(mountPath, dag.SetSecret(secretName, content)).
		WithExec([]string{"sh", "-c", fmt.Sprintf(
			"cp %q %q  # content:%x", mountPath, dstPath, digest[:8])}).
		WithoutMount(mountPath)
}

// directoryWithSecretFile is withSecretFile for callers that hold a Directory
// rather than a Container: it round-trips through a throwaway container so the
// plaintext still never becomes an operation argument.
func (m *Vm) directoryWithSecretFile(
	ctx context.Context,
	dir *dagger.Directory,
	dstPath, content, secretName string,
) (*dagger.Directory, error) {
	ctr, err := m.container(ctx)
	if err != nil {
		return nil, fmt.Errorf("container init failed: %w", err)
	}

	const workDir = "/secret-write"

	return withSecretFile(
		ctr.WithDirectory(workDir, dir),
		path.Join(workDir, dstPath),
		content,
		secretName,
	).Directory(workDir), nil
}

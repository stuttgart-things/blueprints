package main

import (
	"context"

	"dagger/vm/internal/dagger"
)

func (m *Vm) container(
	ctx context.Context) (*dagger.Container, error) {
	if m.BaseImage == "" {
		m.BaseImage = "cgr.dev/chainguard/wolfi-base:latest"
	}

	ctr := dag.Container().
		From(m.BaseImage)

	return ctr, nil
}

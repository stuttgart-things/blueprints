package main

import (
	"context"

	"dagger/vm/internal/dagger"
)

func (v *Vm) container(
	ctx context.Context) (*dagger.Container, error) {
	if v.BaseImage == "" {
		v.BaseImage = "cgr.dev/chainguard/wolfi-base:latest"
	}

	ctr := dag.Container().
		From(v.BaseImage)

	return ctr, nil
}

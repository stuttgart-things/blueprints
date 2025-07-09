package main

import (
	"context"
	"dagger/go-microservice/internal/dagger"
)

func (m *GoMicroservice) RunBuildStage(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	// +default="1.23.6"
	goVersion string,
	// +optional
	// +default="linux"
	os string,
	// +optional
	// +default="amd64"
	arch string,
	// +optional
	// +default="main.go"
	goMainFile string,
	// +optional
	// +default="main"
	binName string,
	// +optional
	ldflags string,
) (*dagger.Directory, error) {
	// Start timing the workflow

	binDir := dag.Go().BuildBinary(
		src,
		dagger.GoBuildBinaryOpts{
			GoVersion:  goVersion,
			Os:         os,
			Arch:       arch,
			GoMainFile: goMainFile,
			BinName:    binName,
			Ldflags:    ldflags,
		})

	return binDir, nil
}

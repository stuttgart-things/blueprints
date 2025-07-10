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
	// +optional
	// +default="GITHUB_TOKEN"
	tokenName string,
	// +optional
	token *dagger.Secret,
	// +optional
	// +default="ko.local"
	koRepo string,
	// +optional
	// +default="v0.18.0"
	koVersion string,
	// +optional
	// +default="."
	koBuildArg string,
	// +optional
	// +default="false"
	koPush string,
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

	imageID, err := dag.Go().KoBuild(
		ctx,
		src,
		dagger.GoKoBuildOpts{
			TokenName: tokenName,
			Token:     token,
			Repo:      koRepo,
			BuildArg:  koBuildArg,
			KoVersion: koVersion,
			Push:      koPush,
		},
	)
	if err != nil {
		return nil, err
	}

	binDir = binDir.WithNewFile(
		"ko-image.txt", imageID)

	return binDir, nil
}

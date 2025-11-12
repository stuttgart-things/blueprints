package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger/go-microservice/internal/dagger"

	"golang.org/x/sync/errgroup"
)

func (m *GoMicroservice) RunBuildStage(
	ctx context.Context,
	src *dagger.Directory,
	// +optional
	// +default="1.25.4"
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
	// +default=""
	packageName string,
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
	// +optional
	// +default=true
	buildBinary bool,
	// +optional
	// +default=true
	koBuild bool,
	// +optional
	// +default="build-report.txt"
	reportName string,

) (*dagger.Directory, error) {
	totalStart := time.Now()
	var (
		binDir   *dagger.Directory
		imageID  string
		result   = dag.Directory()
		errGroup errgroup.Group

		binaryBuildTime time.Duration
		koBuildTime     time.Duration
	)

	// Concurrently execute build steps
	if buildBinary {
		errGroup.Go(func() error {
			start := time.Now()
			binDir = dag.Go().BuildBinary(
				src,
				dagger.GoBuildBinaryOpts{
					GoVersion:   goVersion,
					Os:          os,
					Arch:        arch,
					GoMainFile:  goMainFile,
					BinName:     binName,
					Ldflags:     ldflags,
					PackageName: packageName,
				})

			// Force evaluation by getting directory ID
			_, err := binDir.ID(ctx)
			binaryBuildTime = time.Since(start)
			return err
		})
	}

	if koBuild {
		errGroup.Go(func() error {
			start := time.Now()
			var err error
			imageID, err = dag.Go().KoBuild(
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
			koBuildTime = time.Since(start)
			return err
		})
	}

	// Wait for all concurrent operations to complete
	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	totalDuration := time.Since(totalStart)

	// Generate build report
	report := strings.Builder{}
	report.WriteString("=== Build Report ===\n")
	report.WriteString(fmt.Sprintf("Total Duration: %s\n\n", totalDuration.Round(time.Millisecond)))

	if buildBinary {
		report.WriteString(fmt.Sprintf("Binary Build Duration: %s\n", binaryBuildTime.Round(time.Millisecond)))
		// Add binary to result
		result = result.WithDirectory("/", binDir)
	}

	if koBuild {
		report.WriteString(fmt.Sprintf("Ko Build Duration: %s\n", koBuildTime.Round(time.Millisecond)))
		report.WriteString(fmt.Sprintf("Image ID: %s\n", imageID))
	}

	result = result.WithNewFile(
		reportName,
		report.String())

	return result, nil
}

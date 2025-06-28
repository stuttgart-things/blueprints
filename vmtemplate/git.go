package main

import (
	"dagger/vmtemplate/internal/dagger"
	"fmt"
)

func (m *Vmtemplate) CloneGitRepository(
	// Source code management (SCM) version to use
	// +optional
	// +default="github"
	scm string,
	repository string,
	token *dagger.Secret) *dagger.Directory {

	switch scm {
	case "github":
		return dag.Git().CloneGitHub(repository, token)

	default:
		panic(fmt.Sprintf("unsupported git type: %s", scm))
	}
}

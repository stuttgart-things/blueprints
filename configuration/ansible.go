package main

import (
	"context"

	"dagger/configuration/internal/dagger"
)

// cacheBusterMarker is the file whose contents vary to defeat memoisation.
// Dot-prefixed and named for what it is, so it is obvious in a directory
// listing that it carries no meaning of its own.
const cacheBusterMarker = ".dagger-cache-buster"

func (m *Configuration) CreateAnsibleRequirementFiles(
	ctx context.Context,
	// +optional
	src *dagger.Directory,
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements.yaml.tmpl"
	templatePaths string,
	// Path to YAML or JSON file containing template data (supports HTTPS URLs)
	// +optional
	// +default="https://raw.githubusercontent.com/stuttgart-things/ansible/refs/heads/main/templates/requirements-data.yaml"
	dataFile string,
	// +optional
	// +default=false
	strictMode bool,
	// Any value that changes between runs -- a timestamp, a CI run id. Forces a
	// fresh render instead of a cached one.
	//
	// This is NOT the usual "+optional and then discarded" cache buster. Dagger
	// memoises RenderFromFile on its arguments, and when template and data are
	// remote those arguments are two URL STRINGS that never change; nothing can
	// tell Dagger the content behind the URL moved. Discarding the value here
	// would bust only THIS function and leave the inner call answering from its
	// own cache.
	//
	// So the value is written into a marker file inside `src`, which IS an
	// argument of the inner call. A different value means a different directory
	// digest means a genuine re-render. The file is never read: with remote
	// template and data RenderFromFile does not touch src at all, and with local
	// ones it addresses templates by explicit path, so a dot-file alongside them
	// is inert.
	//
	// The obvious approach -- appending ?cacheBuster= to the URLs -- does not
	// work: RenderFromFile derives the data format from the string's suffix and
	// rejects `.yaml?cacheBuster=...` as an unsupported format.
	//
	// RESIDUAL: this defeats Dagger's cache, not GitHub's. raw.githubusercontent
	// serves a pushed change for a few minutes before it propagates, so a render
	// seconds after a merge can still be stale. That window is bounded and
	// self-healing; the Dagger one was neither.
	//
	// Left empty nothing is added and the behaviour is exactly as before.
	// +optional
	// +default=""
	cacheBuster string,
) (*dagger.Directory, error) {

	if cacheBuster != "" {
		if src == nil {
			src = dag.Directory()
		}
		src = src.WithNewFile(cacheBusterMarker, cacheBuster)
	}

	// RENDER TEMPLATES WITH DATA FROM FILE
	renderedRequirementsFile := dag.Templating().RenderFromFile(
		templatePaths,
		dataFile,
		dagger.TemplatingRenderFromFileOpts{
			Src:        src,
			StrictMode: strictMode,
		},
	)

	return renderedRequirementsFile, nil
}

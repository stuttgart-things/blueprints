package main

import (
	"context"
	"fmt"
	"strings"

	"dagger/argocd/internal/dagger"
)

// CommitConfig commits configFile to <destinationPath>/<fileName> on
// <branchName> in <repository>. Optionally opens a pull request against
// <baseBranch> and optionally merges that PR.
//
// Returns a multi-line summary of what was committed / opened / merged.
func (m *Argocd) CommitConfig(
	ctx context.Context,
	// Rendered config file to commit
	configFile *dagger.File,
	// Repository in "owner/repo" format
	repository string,
	// GitHub token for git operations
	gitToken *dagger.Secret,
	// Branch to commit the file to
	// +optional
	// +default="main"
	branchName string,
	// Destination folder within the repository (without leading slash)
	// +optional
	// +default="argocd/clusters/"
	destinationPath string,
	// File name to write under destinationPath (used as the committed basename)
	// +optional
	// +default="cluster.yaml"
	fileName string,
	// Commit message
	// +optional
	// +default="Add Argo CD cluster registration"
	commitMessage string,
	// Open a pull request from branchName into baseBranch
	// +optional
	// +default=false
	createPR bool,
	// Base branch for the PR
	// +optional
	// +default="main"
	baseBranch string,
	// PR title (defaults to commitMessage when empty)
	// +optional
	prTitle string,
	// PR body
	// +optional
	prBody string,
	// Merge the PR after creation
	// +optional
	// +default=false
	mergePR bool,
	// Merge method: "squash", "merge", or "rebase"
	// +optional
	// +default="squash"
	mergeMethod string,
) (string, error) {
	if configFile == nil {
		return "", fmt.Errorf("config-file is required")
	}
	if fileName == "" {
		return "", fmt.Errorf("fileName is required")
	}

	fileToCommit := dag.Directory().
		WithFile(fileName, configFile).
		File(fileName)

	// Upstream git.AddFileToGithubBranch treats destinationPath as the full
	// repo-relative path (folder + filename). Join folder + fileName here so
	// callers can pass a folder via --destination-path and a basename via
	// --file-name (e.g. argocd/clusters/ + philly.yaml → argocd/clusters/philly.yaml).
	destFolder := strings.TrimSuffix(destinationPath, "/")
	fullDestPath := fileName
	if destFolder != "" {
		fullDestPath = destFolder + "/" + fileName
	}

	var results []string

	if branchName != baseBranch {
		// Probe whether the branch already exists. The underlying
		// dag.Git().CreateGithubBranch silently force-resets an existing
		// branch back to baseBranch's SHA (see
		// stuttgart-things/blueprints#156 — the impl tries `gh api refs`
		// to create, and on failure runs a PATCH with force=true). For a
		// PR-driven workflow where the branch was opened by the Backstage
		// scaffolder with the user's values.yaml, that wipes the
		// scaffolder's commits. Skip CreateGithubBranch when the branch
		// is already there and just commit on top.
		exists, err := branchExists(ctx, repository, branchName, gitToken)
		if err != nil {
			return "", fmt.Errorf("ensure-branch (probe): %w", err)
		}
		if exists {
			results = append(results, fmt.Sprintf("Branch %s already exists; committing on top", branchName))
		} else {
			if _, err := dag.Git().CreateGithubBranch(
				ctx,
				repository,
				branchName,
				gitToken,
				dagger.GitCreateGithubBranchOpts{BaseBranch: baseBranch},
			); err != nil {
				return "", fmt.Errorf("ensure-branch: %w", err)
			}
			results = append(results, fmt.Sprintf("Created branch %s from %s", branchName, baseBranch))
		}
	}

	commitSHA, err := dag.Git().AddFileToGithubBranch(
		ctx,
		repository,
		branchName,
		commitMessage,
		gitToken,
		fileToCommit,
		fullDestPath,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no changes to commit") {
			results = append(results, fmt.Sprintf("No changes to commit (file already up-to-date in %s on %s)", repository, branchName))
		} else {
			return "", fmt.Errorf("commit-config: %w", err)
		}
	} else {
		results = append(results, fmt.Sprintf("Committed %s to %s on %s (sha=%s)", fullDestPath, repository, branchName, commitSHA))
	}

	if !createPR {
		return strings.Join(results, "\n"), nil
	}

	if branchName == baseBranch {
		return "", fmt.Errorf("create-pr: branchName (%q) must differ from baseBranch (%q)", branchName, baseBranch)
	}

	if prTitle == "" {
		prTitle = commitMessage
	}

	prURL, err := dag.Git().CreateGithubPullRequest(
		ctx,
		repository,
		branchName,
		prTitle,
		prBody,
		gitToken,
		dagger.GitCreateGithubPullRequestOpts{BaseBranch: baseBranch},
	)
	if err != nil {
		return "", fmt.Errorf("create-pr: %w", err)
	}
	results = append(results, fmt.Sprintf("Opened PR: %s", prURL))

	if !mergePR {
		return strings.Join(results, "\n"), nil
	}

	method := strings.ToLower(strings.TrimSpace(mergeMethod))
	switch method {
	case "squash", "merge", "rebase":
	default:
		return "", fmt.Errorf("merge-pr: unknown mergeMethod %q (use squash|merge|rebase)", mergeMethod)
	}

	mergeOutput, err := dag.Container().
		From("cgr.dev/chainguard/wolfi-base:latest").
		WithExec([]string{"apk", "add", "--no-cache", "github-cli"}).
		WithSecretVariable("GH_TOKEN", gitToken).
		WithExec([]string{
			"gh", "pr", "merge", strings.TrimSpace(prURL),
			"--" + method,
			"--delete-branch",
		}).
		Stdout(ctx)
	if err != nil {
		return strings.Join(results, "\n"), fmt.Errorf("merge-pr: %w", err)
	}
	results = append(results, fmt.Sprintf("Merged PR (%s):\n%s", method, strings.TrimSpace(mergeOutput)))

	return strings.Join(results, "\n"), nil
}

// branchExists checks whether <repository>'s <branchName> ref exists on
// GitHub. Returns (true, nil) when the ref resolves, (false, nil) when
// GitHub returns 404, and (false, err) on any other failure (auth, rate
// limit, network).
//
// Uses `gh api -i` (response with headers) so we can distinguish a real
// 404 from a 401/403 (auth) or transient errors. We don't use `git
// ls-remote` here to keep the probe cheap and avoid pulling history.
func branchExists(
	ctx context.Context,
	repository string,
	branchName string,
	gitToken *dagger.Secret,
) (bool, error) {
	probeCmd := fmt.Sprintf(
		`gh api -i "repos/%s/git/refs/heads/%s" 2>&1 | head -n1 || true`,
		repository, branchName)

	out, err := dag.Container().
		From("cgr.dev/chainguard/wolfi-base:latest").
		WithExec([]string{"apk", "add", "--no-cache", "github-cli"}).
		WithSecretVariable("GH_TOKEN", gitToken).
		WithExec([]string{"sh", "-c", probeCmd}, dagger.ContainerWithExecOpts{Expect: dagger.ReturnTypeAny}).
		Stdout(ctx)
	if err != nil {
		return false, fmt.Errorf("branch probe: %w", err)
	}

	first := strings.TrimSpace(out)
	switch {
	case strings.Contains(first, " 200 "):
		return true, nil
	case strings.Contains(first, " 404 "):
		return false, nil
	default:
		return false, fmt.Errorf("unexpected gh api response probing %s on %s: %s", branchName, repository, first)
	}
}

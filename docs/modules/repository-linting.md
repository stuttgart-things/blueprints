# Repository Linting Module

Repository checks, aggregate findings, create GitHub issues, and AI-powered analysis.

## Features

- Validate repositories across multiple technologies
- Aggregate and export linting findings
- Create GitHub issues automatically
- AI-powered report analysis
- AI-enhanced issue creation

## Usage

### Validate Repository

Validate a repository and export findings:

```bash
dagger call -m repository-linting validate-multiple-technologies \
  --src tests/repository-linting/test-repo \
  export --path /tmp/all-findings.txt
```

### Create GitHub Issue

Create a labeled issue in a repository:

```bash
dagger call -m repository-linting create-github-issue \
  --repository stuttgart-things/stuttgart-things \
  --token env:GITHUB_TOKEN \
  --title "Test Issue from Dagger" \
  --body "This issue was automatically created using Dagger!" \
  --label automation \
  --label test
```

### AI Analysis of Linting Report

Analyze a linting report with AI:

```bash
# Set up OpenRouter
export OPENAI_BASE_URL="https://openrouter.ai/api/v1"
export OPENAI_API_KEY="sk-or-v1-..."  # pragma: allowlist secret

dagger call -m repository-linting analyze-report \
  --report-file /tmp/all-findings.txt \
  --model="minimax/minimax-m2:free" \
  export --path=/tmp/ai.txt
```

### AI Analysis with GitHub Issue Creation

Analyze report and create an issue automatically:

```bash
export OPENAI_BASE_URL="https://openrouter.ai/api/v1"
export OPENAI_API_KEY="sk-or-v1-..."  # pragma: allowlist secret

dagger call -m repository-linting analyze-report-and-create-issue \
  --report-file /tmp/platform-engineering-showcase-lint-findings.txt \
  --repository stuttgart-things/stuttgart-things \
  --token env:GITHUB_TOKEN \
  --model="minimax/minimax-m2:free"
```

### AI-Enhanced Issue Creation

Create a GitHub issue with AI-enhanced formatting:

```bash
export GEMINI_API_KEY="AIzaS..."  # pragma: allowlist secret

dagger call -m repository-linting create-issue \
  --repository stuttgart-things/stuttgart-things \
  --token env:GITHUB_TOKEN \
  --content "add hello world go code" \
  --model="gemini-2.5-flash"
```

## Parameters

### validate-multiple-technologies

| Parameter | Description |
|-----------|-------------|
| `--src` | Repository path to validate |

### create-github-issue

| Parameter | Description |
|-----------|-------------|
| `--repository` | GitHub repository (owner/repo) |
| `--token` | GitHub token |
| `--title` | Issue title |
| `--body` | Issue body |
| `--label` | Labels to add (can be repeated) |

### analyze-report

| Parameter | Description |
|-----------|-------------|
| `--report-file` | Path to linting report |
| `--model` | AI model to use |

## Supported AI Providers

- **OpenRouter**: Set `OPENAI_BASE_URL` and `OPENAI_API_KEY`
- **Gemini**: Set `GEMINI_API_KEY`

## Test Data

Example test data can be found in:

- `tests/repository-linting/test-repo/`

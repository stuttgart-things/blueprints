# stuttgart-things/blueprints/repository-linting

## USAGE/FUNCTIONS

### Repository Linting Module

Validate a test repository and export all findings:

```bash
dagger call -m repository-linting validate-multiple-technologies \
  --src tests/repository-linting/test-repo \
  export --path /tmp/all-findings.txt
```

With pre-commit hooks, secrets scanning, and fail control:

```bash
dagger call -m repository-linting validate-multiple-technologies \
  --src tests/repository-linting/test-repo \
  --enable-pre-commit=true \
  --enable-secrets=true \
  --fail-on any \
  export --path /tmp/all-findings.txt
```

- `--src tests/repository-linting/test-repo` selects the repository to validate
- `--enable-pre-commit` enables pre-commit hook linting (default: false)
- `--enable-secrets` enables detect-secrets scanning (default: false)
- `--fail-on any` fails the pipeline if any linter produces findings (default: `none`)
- `export --path /tmp/all-findings.txt` saves the merged findings to a text file

All linters run in parallel. Results are merged in fixed order: YAML, Markdown, Pre-Commit, Secrets.

### Run Pre-Commit Hooks

Run pre-commit hooks standalone on a repository:

```bash
dagger call -m repository-linting run-pre-commit \
  --src tests/repository-linting/test-repo \
  export --path /tmp/precommit.txt
```

Skip specific hooks:

```bash
dagger call -m repository-linting run-pre-commit \
  --src tests/repository-linting/test-repo \
  --skip-hooks trailing-whitespace \
  --skip-hooks end-of-file-fixer \
  export --path /tmp/precommit.txt
```

### Scan Secrets

Scan a repository for secrets using detect-secrets:

```bash
dagger call -m repository-linting scan-secrets \
  --src tests/repository-linting/test-repo \
  export --path /tmp/secrets.json
```

### Auto-Fix Secrets

Use AI to add `pragma: allowlist secret` comments to flagged lines:

```bash
dagger call -m repository-linting auto-fix-secrets \
  --src tests/repository-linting/test-repo \
  export --path /tmp/fixed-repo
```

#### 📂 Test Data

Example test data can be found in:
- `tests/repository-linting/test-repo/`

### Create GitHub Issue via Dagger

You can automatically create a GitHub issue using Dagger:

```bash
dagger call -m repository-linting create-github-issue \
  --repository stuttgart-things/stuttgart-things \
  --token env:GITHUB_TOKEN \
  --title "🧪 Test Issue from Dagger" \
  --body "This issue was automatically created using Dagger!" \
  --label automation \
  --label test
```

This command creates a labeled issue in the specified repository using your GitHub token.

### AI Analysis of Linting Report

You can use the AI-powered analysis function via Dagger CLI as follows:

```sh
# EXAMPLE w/ OPENROUTER / MODEL
export OPENAI_BASE_URL="https://openrouter.ai/api/v1"
export OPENAI_API_KEY="sk-or-v1-b7#..." # pragma: allowlist secret

dagger call -m repository-linting analyze-report \
--report-file /tmp/all-findings.txt \
--model="minimax/minimax-m2:free" \
export --path=/tmp/ai.txt
```

This command analyzes the linting report in `/tmp/all-findings.txt` and writes the AI-generated review to `/tmp/ai.txt`.

### AI Analysis of Linting Report And create github issue

```sh
# EXAMPLE USAGE w/ OPENROUTER
export OPENAI_BASE_URL="https://openrouter.ai/api/v1" # pragma: allowlist secret
export OPENAI_API_KEY="sk-or-..." # pragma: allowlist secret

# EXAMPLE w/ GEMINI
#export GEMINI_API_KEY="<your-api-key>" # pragma: allowlist secret
dagger call -m repository-linting analyze-report-and-create-issue \
  --report-file /tmp/platform-engineering-showcase-lint-findings.txt \
  --repository stuttgart-things/stuttgart-things \
  --token env:GITHUB_TOKEN \
  --model="minimax/minimax-m2:free"
```

**Important:** Never commit real secrets to your repository. Use environment variables, secret managers, or CI/CD vaults for sensitive data. Mark example lines with `# pragma: allowlist secret` to avoid false positives.

### Create github issue w/ help of ai agent

create a github issue with ai-enhanced formatting

```bash
export GEMINI_API_KEY="AIzaS..." # pragma: allowlist secret

dagger call -m repository-linting \
create-issue \
--repository stuttgart-things/stuttgart-things \
--token env:GITHUB_TOKEN \
--content "add helo world go code" \
--model="gemini-2.5-flash"
```

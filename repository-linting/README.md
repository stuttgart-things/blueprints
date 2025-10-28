# stuttgart-things/blueprints/repository-linting

## USAGE/FUNCTIONS

### Repository Linting Module

Validate a test repository and export all findings:

```bash
dagger call -m repository-linting validate-multiple-technologies \
  --src tests/repository-linting/test-repo \
  export --path /tmp/all-findings.txt
```

- `--src tests/repository-linting/test-repo` selects the repository to validate
- `export --path /tmp/all-findings.txt` saves the findings to a text file

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
dagger call -m repository-linting analyze-report \
--report-file /tmp/all-findings.txt \
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

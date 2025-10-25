
## Using GEMINI API Keys

To enable GEMINI-powered AI features, set your API key as an environment variable:

```sh
export GEMINI_API_KEY="<your-api-key>" # pragma: allowlist secret
```

**Important:** Never commit real secrets to your repository. Use environment variables, secret managers, or CI/CD vaults for sensitive data. Mark example lines with `# pragma: allowlist secret` to avoid false positives.
# Example: AI Analysis of Linting Report

You can use the AI-powered analysis function via Dagger CLI as follows:

```sh
dagger call -m repository-linting analyze-report --report-file /tmp/all-findings.txt export --path=/tmp/ai.txt
```

This command analyzes the linting report in `/tmp/all-findings.txt` and writes the AI-generated review to `/tmp/ai.txt`.
# Repository Linting Module

Validate multiple technologies in a repository using Dagger.

## 🚀 Quick Start

### Prerequisites
- Dagger CLI ([Installation](https://docs.dagger.io/install))
- Docker

### Example

Validate a test repository and export all findings:

```bash
dagger call -m repository-linting validate-multiple-technologies --src tests/repository-linting/test-repo export --path /tmp/all-findings.txt
```

- `--src tests/repository-linting/test-repo` selects the repository to validate
- `export --path /tmp/all-findings.txt` saves the findings to a text file

## 📂 Test Data

Example test data can be found in:
- `tests/repository-linting/test-repo/`

## 📖 More Modules

See the main README for more Dagger modules and examples.

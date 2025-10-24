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

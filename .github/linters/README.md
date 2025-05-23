# Linting Configuration

This directory contains linting configurations for the OpenFero project.

## Overview

The project uses a parallelized GitHub Actions workflow (`lint.yml`) that replaces the previous super-linter configuration. This approach provides:

- **Better Performance**: Individual linters run in parallel only when relevant files change
- **Targeted Feedback**: Each linter focuses on specific file types
- **Easier Maintenance**: Individual linter configurations are easier to customize

## Supported Linters

### Go
- **Tool**: golangci-lint
- **Config**: `.golangci.yml`
- **Files**: `**/*.go`

### YAML
- **Tool**: yamllint
- **Config**: `.yamllint`
- **Files**: `**/*.yml`, `**/*.yaml`

### JavaScript
- **Tool**: ESLint
- **Config**: Generated dynamically with ES2022 support
- **Files**: `**/*.js` (excluding minified files)

### CSS
- **Tool**: stylelint
- **Config**: Generated dynamically with standard rules
- **Files**: `**/*.css` (excluding minified files)

### Shell Scripts
- **Tool**: ShellCheck
- **Files**: `**/*.sh`

### Dockerfile
- **Tool**: Hadolint
- **Files**: `**/Dockerfile*`, `**/*.dockerfile`

### Markdown
- **Tool**: markdownlint
- **Config**: Generated dynamically with relaxed rules
- **Files**: `**/*.md`

## Migration from Super-Linter

The new workflow maintains the same validation coverage as the previous super-linter setup:

- ✅ Go linting (now enabled with golangci-lint)
- ✅ YAML linting (now enabled with yamllint)
- ✅ JavaScript linting (now enabled with ESLint)
- ✅ All other file types supported by super-linter

### Disabled in Super-Linter (now enabled)
- `VALIDATE_GO: false` → Now using golangci-lint with custom config
- `VALIDATE_YAML: false` → Now using yamllint with custom config  
- `VALIDATE_JAVASCRIPT_STANDARD: false` → Now using ESLint

## Local Development

To run linters locally before pushing:

```bash
# Go (requires golangci-lint)
golangci-lint run --config=.github/linters/.golangci.yml

# YAML (requires yamllint)
find . -name "*.yml" -o -name "*.yaml" | grep -v node_modules | xargs yamllint -c .github/linters/.yamllint

# JavaScript (requires ESLint)
find . -name "*.js" -not -path "*/node_modules/*" -not -name "*.min.js" | xargs eslint

# Shell scripts (requires shellcheck)
shellcheck scripts/*.sh
```

## Configuration Files

- `.golangci.yml` - Go linting rules and settings
- `.yamllint` - YAML linting rules and formatting requirements

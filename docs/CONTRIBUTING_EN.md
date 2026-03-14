# NAS-OS Contribution Guide

[中文](../CONTRIBUTING.md) | **English**

Thank you for considering contributing to NAS-OS! This document will help you understand how to participate in project development.

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How to Contribute](#how-to-contribute)
- [Development Environment](#development-environment)
- [Code Standards](#code-standards)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)
- [Project Structure](#project-structure)

---

## Code of Conduct

- Respect all contributors
- Constructive discussions and feedback
- Focus on what's best for the project
- Be friendly and patient with newcomers

---

## How to Contribute

### Reporting Bugs

1. Search [Existing Issues](https://github.com/crazyqin/nas-os/issues) to ensure the problem hasn't been reported
2. Create a new Issue using the Bug Report template
3. Provide detailed reproduction steps and environment information

### Proposing New Features

1. First discuss your idea in [Discussions](https://github.com/crazyqin/nas-os/discussions)
2. Create an Issue using the Feature Request template
3. Explain the use case and technical approach

### Submitting Code

1. Fork the repository
2. Create a feature branch
3. Write code and tests
4. Submit a Pull Request

---

## Development Environment

### Prerequisites

| Tool | Version | Description |
|------|---------|-------------|
| Go | 1.21+ | Programming language |
| btrfs-progs | Latest | Storage tools |
| Docker | 20.10+ | Container runtime |
| Git | 2.x | Version control |

### Installing Development Tools

```bash
# Go development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest

# Code formatting
go install mvdan.cc/gofumpt@latest
```

### Clone Project

```bash
git clone https://github.com/crazyqin/nas-os.git
cd nas-os
go mod tidy
```

### Running Tests

```bash
# Unit tests
make test

# Coverage report
make test-coverage

# Race detection
make test-race

# Full test suite
make test-all
```

### Local Development

```bash
# Build
make build

# Development mode (requires root)
sudo ./nasd

# Or use air for hot reload
air
```

---

## Code Standards

### Go Code Style

Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Core Principles**:

1. **Naming Conventions**
   - Package names: lowercase words, no underscores
   - Exported functions/types: Capitalized, camelCase
   - Private functions/variables: lowercase start, camelCase
   - Interfaces: verbs or `-er` suffix (e.g., `Reader`, `Writer`)

2. **Error Handling**
   ```go
   // ✅ Correct: Handle errors immediately
   if err := doSomething(); err != nil {
       return fmt.Errorf("failed to do something: %w", err)
   }
   
   // ❌ Wrong: Ignoring errors
   doSomething()
   ```

3. **Comment Standards**
   ```go
   // DoSomething performs an operation.
   // Parameter name specifies the operation name.
   // Returns the operation result or an error.
   func DoSomething(name string) (*Result, error) {
       // ...
   }
   ```

4. **Code Organization**
   - Organize code by functional modules
   - Keep related files in the same package
   - Separate interface definitions from implementations

### Code Quality Checks

```bash
# Format
gofmt -w .

# Static analysis
golangci-lint run

# Code check
go vet ./...
```

### File Structure

```
internal/module/
├── module.go       # Module definition and interface
├── handler.go      # API handlers
├── service.go      # Business logic
├── repository.go   # Data access
├── model.go        # Data models
└── module_test.go  # Unit tests
```

---

## Commit Guidelines

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

| Type | Description | Example |
|------|-------------|---------|
| `feat` | New feature | feat(storage): add snapshot recovery |
| `fix` | Bug fix | fix(smb): fix permission validation |
| `docs` | Documentation update | docs: update API documentation |
| `style` | Code formatting | style: format code |
| `refactor` | Refactoring | refactor(storage): optimize volume management |
| `test` | Tests | test(quota): add quota unit tests |
| `chore` | Build/tools | chore: update CI configuration |
| `perf` | Performance optimization | perf(cache): optimize LRU cache |

### Scopes

Common scopes:
- `storage` - Storage management
- `smb` / `nfs` - File sharing
- `users` - User management
- `monitor` - System monitoring
- `docker` - Docker integration
- `api` - API related
- `webui` - Web interface
- `docs` - Documentation

### Examples

```bash
# Simple commit
git commit -m "feat(storage): add snapshot recovery"

# Commit with body
git commit -m "feat(storage): add snapshot recovery" -m "
- Support BTRFS snapshot recovery
- Support incremental recovery
- Add recovery progress tracking

Closes #123"

# Breaking change
git commit -m "feat(api)!: refactor storage API" -m "
BREAKING CHANGE: Storage volume API response format changed
- Use `volume` instead of `vol`
- Add `status` field"
```

---

## Pull Request Process

### Creating a PR

1. **Fork and create branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Write code**
   - Follow code standards
   - Add necessary tests
   - Update related documentation

3. **Local verification**
   ```bash
   make test-all
   make lint
   make build
   ```

4. **Commit and push**
   ```bash
   git add .
   git commit -m "feat(module): add some feature"
   git push origin feature/my-feature
   ```

5. **Create Pull Request**
   - Use PR template
   - Link related Issues
   - Describe changes

### PR Checklist

- [ ] Code passes all tests
- [ ] Code passes lint checks
- [ ] New features have corresponding tests
- [ ] Documentation updated
- [ ] CHANGELOG updated (if applicable)
- [ ] PR title follows commit convention

### Review Process

1. At least 1 Reviewer approval required
2. All CI checks must pass
3. Resolve all review comments
4. Squash merge to main branch

---

## Issue Guidelines

### Bug Report

Use Bug Report template, including:
- Clear problem description
- Reproduction steps
- Expected vs actual behavior
- Environment information
- Relevant logs

### Feature Request

Use Feature Request template, including:
- Feature description and use case
- Technical approach (optional)
- Impact assessment

---

## Project Structure

```
nas-os/
├── cmd/                    # Executables
│   ├── nasd/              # Main service
│   └── nasctl/            # CLI tool
├── internal/              # Internal modules
│   ├── storage/           # Storage management
│   ├── smb/               # SMB service
│   ├── nfs/               # NFS service
│   ├── users/             # User management
│   ├── monitor/           # System monitoring
│   ├── docker/            # Docker integration
│   └── web/               # Web service
├── pkg/                   # Public libraries
├── webui/                 # Frontend interface
├── docs/                  # Documentation
├── configs/               # Configuration files
├── scripts/               # Scripts
└── tests/                 # Test files
```

### Module Description

| Directory | Description |
|-----------|-------------|
| `cmd/nasd` | Main service entry |
| `internal/` | Internal implementation, not exposed |
| `pkg/` | Reusable public libraries |
| `docs/` | User and developer documentation |
| `tests/` | Integration and E2E tests |

---

## Getting Help

- 📖 [Documentation](docs/)
- 💬 [Discussions](https://github.com/crazyqin/nas-os/discussions)
- 🐛 [Issues](https://github.com/crazyqin/nas-os/issues)

---

*Thank you for your contribution!* 🎉
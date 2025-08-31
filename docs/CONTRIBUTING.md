# Contributing to PostgreSQL Operator

Thank you for your interest in contributing to the PostgreSQL Operator for CloudNativePG! This document provides guidelines and information for contributors.

## Code of Conduct

This project adheres to a code of conduct that we expect all contributors to follow. Please be respectful and professional in all interactions.

## How to Contribute

### Reporting Issues

- Use GitHub Issues to report bugs or request features
- Search existing issues to avoid duplicates
- Provide detailed information including:
  - Operating system and version
  - Kubernetes version
  - CloudNativePG version
  - Steps to reproduce the issue
  - Expected vs actual behavior
  - Relevant logs or error messages

### Development Setup

1. **Prerequisites**:
   ```bash
   # Required tools
   - Go 1.24+
   - Docker
   - kubectl
   - kind or minikube (for local testing)
   - kubebuilder
   ```

2. **Clone and Setup**:
   ```bash
   git clone https://github.com/silverswarm/pg-operator.git
   cd pg-operator
   
   # Install dependencies
   go mod download
   
   # Generate code and manifests
   make generate
   make manifests
   ```

3. **Local Development**:
   ```bash
   # Run tests
   make test
   
   # Run locally (requires kubeconfig)
   make install  # Install CRDs
   make run      # Run controller locally
   
   # Build binary
   make build
   ```

### Making Changes

1. **Fork the Repository**:
   - Fork the project on GitHub
   - Clone your fork locally
   - Add upstream remote: `git remote add upstream https://github.com/silverswarm/pg-operator.git`

2. **Create a Branch**:
   ```bash
   git checkout -b feature/my-new-feature
   # or
   git checkout -b fix/bug-description
   ```

3. **Development Guidelines**:
   - Follow Go best practices and conventions
   - Add tests for new functionality
   - Update documentation as needed
   - Ensure all tests pass: `make test`
   - Run linting: `make lint`
   - Verify manifests are up-to-date: `make generate && make manifests`

4. **Commit Messages**:
   - Use clear, descriptive commit messages
   - Follow conventional commit format when possible:
     ```
     feat: add support for cross-namespace connections
     fix: resolve database creation race condition
     docs: update README with new examples
     test: add unit tests for user permissions
     ```

### Testing

- **Unit Tests**: Run `make test` to execute unit tests
- **Integration Tests**: Ensure your changes work with actual CNPG clusters
- **End-to-End Tests**: Run `make test-e2e` if available
- **Manual Testing**: Test in a real Kubernetes cluster with CNPG

### Documentation

- Update README.md for user-facing changes
- Add or update code comments for complex logic
- Update examples in `examples/` directory
- Update API documentation in CRD spec comments

### Submitting Changes

1. **Push to Your Fork**:
   ```bash
   git push origin feature/my-new-feature
   ```

2. **Create Pull Request**:
   - Open a PR against the main branch
   - Provide detailed description of changes
   - Reference related issues
   - Ensure all checks pass

3. **Review Process**:
   - Maintainers will review your PR
   - Address any feedback or requested changes
   - Once approved, your PR will be merged

## Development Standards

### Code Style

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Keep functions focused and small
- Add appropriate error handling
- Follow Kubernetes operator patterns

### Testing Standards

- Write unit tests for new functions
- Test error conditions and edge cases
- Mock external dependencies

### Documentation Standards

- Document public APIs and functions
- Include examples for complex features
- Keep documentation up-to-date
- Use clear, concise language

## Release Process

1. Version bumps follow semantic versioning (semver)
2. Releases are created from the main branch
3. GitHub Actions automatically builds and publishes releases
4. Release notes are generated from commit messages

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Documentation**: Check the README and examples directory

## Recognition

Contributors will be recognized in release notes and may be invited to become maintainers based on their contributions.

Thank you for contributing to the PostgreSQL Operator! 🚀
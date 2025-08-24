# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of PostgreSQL Operator for CloudNativePG
- PostGresConnection CRD for managing connections to CNPG clusters
- Database CRD for creating databases, users, and permissions
- Automatic secret generation for database credentials
- Cross-namespace support for CNPG clusters
- Multi-architecture Docker images (AMD64/ARM64)
- Comprehensive documentation and examples
- CI/CD pipeline with GitHub Actions
- Security scanning with Trivy

### Features
- **Seamless CNPG Integration**: Works with CloudNativePG secrets and services out of the box
- **Database Management**: Create and manage PostgreSQL databases declaratively
- **User & Permission Management**: Create users with fine-grained permissions
- **Automatic Secret Generation**: Generate Kubernetes secrets for database credentials
- **Cross-Namespace Support**: Connect to CNPG clusters in different namespaces
- **Security First**: SSL/TLS connections with configurable security levels

### Technical Details
- Built with Kubebuilder v4
- Go 1.24+ support
- Kubernetes 1.28+ compatibility
- CloudNativePG integration
- PostgreSQL driver support
- Comprehensive test suite
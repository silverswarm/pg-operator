# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please follow these steps:

### Private Disclosure

**Do not report security vulnerabilities through public GitHub issues.**

Instead, please report security vulnerabilities by:

1. **Email**: Send an email to security@silverswarm.io
2. **Subject**: Include "[SECURITY] PostgreSQL Operator" in the subject line
3. **Content**: Provide detailed information about the vulnerability

### What to Include

Please include as much information as possible:

- Type of vulnerability (e.g., authentication bypass, privilege escalation, etc.)
- Full paths of source files related to the vulnerability
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if available)
- Impact assessment
- Suggested fix (if available)

### Response Process

1. **Acknowledgment**: We will acknowledge receipt within 48 hours
2. **Investigation**: We will investigate and validate the report
3. **Resolution**: We will work on a fix and coordinate disclosure
4. **Publication**: We will publish a security advisory after the fix is released

### Security Considerations

The PostgreSQL Operator handles sensitive operations including:

- Database credentials and secrets
- Network connections to PostgreSQL clusters
- Kubernetes RBAC permissions
- Container security contexts

### Best Practices for Users

To use the operator securely:

1. **RBAC**: Use least-privilege RBAC configurations
2. **Network Policies**: Implement network policies to restrict access
3. **Secrets Management**: Use Kubernetes secrets properly
4. **Image Security**: 
   - Use official releases from `ghcr.io/silverswarm/pg-operator`
   - Verify image signatures
   - Scan images for vulnerabilities
5. **TLS**: Always use SSL/TLS connections (`sslMode: require` or higher)
6. **Updates**: Keep the operator updated to the latest version

### Security Scanning

This project includes:

- **Trivy scanning**: Automated vulnerability scanning of container images
- **GitHub Security**: Code scanning and dependency alerts
- **Supply Chain Security**: Signed releases and SBOM generation

### Disclosure Timeline

- **Day 0**: Vulnerability reported
- **Day 1-2**: Acknowledgment sent
- **Day 3-14**: Investigation and validation
- **Day 15-90**: Fix development and testing
- **Day 90+**: Coordinated public disclosure

We aim to resolve critical vulnerabilities within 90 days of disclosure.

Thank you for helping keep the PostgreSQL Operator secure!
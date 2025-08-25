# PostgreSQL Operator for CloudNativePG

[![CI](https://github.com/silverswarm/pg-operator/workflows/CI/badge.svg)](https://github.com/silverswarm/pg-operator/actions/workflows/ci.yml)
[![Release](https://github.com/silverswarm/pg-operator/workflows/Release/badge.svg)](https://github.com/silverswarm/pg-operator/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/silverswarm/pg-operator)](https://goreportcard.com/report/github.com/silverswarm/pg-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/silverswarm/pg-operator)](https://github.com/silverswarm/pg-operator/releases)

A Kubernetes operator that simplifies database and user management for CloudNativePG (CNPG) PostgreSQL clusters.

## Features

- 🔗 **Seamless CNPG Integration** - Works with CloudNativePG secrets and services out of the box
- 🗄️ **Database Management** - Create and manage PostgreSQL databases declaratively
- 👥 **User & Permission Management** - Create users with fine-grained permissions
- 🔐 **Automatic Secret Generation** - Generate Kubernetes secrets for database credentials
- 🌐 **Cross-Namespace Support** - Connect to CNPG clusters in different namespaces
- 🔒 **Security First** - SSL/TLS connections with configurable security levels

## Quick Start

### Prerequisites

- Kubernetes cluster with CNPG operator installed
- A running CloudNativePG cluster

### Installation

#### From GitHub Releases (Recommended)

```bash
# Install the latest release
kubectl apply -f https://github.com/silverswarm/pg-operator/releases/latest/download/install.yaml
```

#### From Source

```bash
git clone https://github.com/silverswarm/pg-operator.git
cd pg-operator
make deploy IMG=ghcr.io/silverswarm/pg-operator:latest
```

### Basic Usage

1. **Create a PostgreSQL Connection**:

```yaml
apiVersion: postgres.silverswarm.io/v1
kind: PostGresConnection
metadata:
  name: my-connection
spec:
  clusterName: "postgres-cluster"  # Your CNPG cluster name
```

2. **Create a Database with Users**:

```yaml
apiVersion: postgres.silverswarm.io/v1
kind: Database
metadata:
  name: myapp-database
spec:
  connectionRef:
    name: "my-connection"
  databaseName: "myapp"
  users:
    - name: "app_user"
      permissions:
        - "CONNECT"
        - "CREATE"
      createSecret: true
    - name: "readonly_user"
      permissions:
        - "CONNECT"
        - "SELECT"
      createSecret: true
```

3. **Use the Generated Secrets**:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      containers:
      - name: app
        image: myapp:latest
        env:
        - name: DB_USERNAME
          valueFrom:
            secretKeyRef:
              name: myapp-app-user  # Auto-generated secret
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: myapp-app-user
              key: password
```

## Configuration

### PostGresConnection

| Field | Description | Default |
|-------|-------------|---------|
| `clusterName` | CNPG cluster name | Required |
| `clusterNamespace` | CNPG cluster namespace | Same as connection |
| `useAppSecret` | Use app user instead of superuser | `false` |
| `sslMode` | SSL connection mode | `require` |
| `host` | Custom host (overrides service discovery) | `{clusterName}-rw` |
| `port` | Custom port | `5432` |

### Database

| Field | Description | Default |
|-------|-------------|---------|
| `connectionRef` | Reference to PostGresConnection | Required |
| `databaseName` | Database name to create | Required |
| `owner` | Database owner | `postgres` |
| `encoding` | Database encoding | `UTF8` |
| `users` | List of users to create | `[]` |

### User Permissions

Available permissions:
- `CONNECT` - Connect to database
- `CREATE` - Create schemas and tables
- `USAGE` - Use schemas
- `SELECT` - Read data
- `INSERT` - Insert data
- `UPDATE` - Update data
- `DELETE` - Delete data
- `ALL` - All privileges

## Advanced Examples

### Cross-Namespace Connection

```yaml
apiVersion: postgres.silverswarm.io/v1
kind: PostGresConnection
metadata:
  name: shared-connection
  namespace: app-namespace
spec:
  clusterName: "shared-postgres"
  clusterNamespace: "postgres-system"
```

### High-Security Connection

```yaml
apiVersion: postgres.silverswarm.io/v1
kind: PostGresConnection
metadata:
  name: secure-connection
spec:
  clusterName: "secure-postgres"
  sslMode: "verify-full"
  useAppSecret: true  # Use less-privileged app user
```

### Custom Secret Names

```yaml
apiVersion: postgres.silverswarm.io/v1
kind: Database
metadata:
  name: tenant-database
spec:
  connectionRef:
    name: "my-connection"
  databaseName: "tenant_db"
  users:
    - name: "tenant_user"
      permissions: ["ALL"]
      secretName: "tenant-credentials"  # Custom secret name
```

## CNPG Integration

The operator automatically integrates with CloudNativePG:

- **Secret Discovery**: Uses `{cluster-name}-superuser` and `{cluster-name}-app` secrets
- **Service Discovery**: Connects via `{cluster-name}-rw` service for read-write operations
- **Cross-Namespace**: Supports FQDN service resolution for cross-namespace connections
- **URI Support**: Can use CNPG-generated connection URIs when available

## Development

### Prerequisites

- Go 1.24+
- Docker
- Kubernetes cluster (kind, minikube, etc.)
- kubebuilder

### Setup

```bash
git clone https://github.com/silverswarm/pg-operator.git
cd pg-operator

# Install dependencies
make generate
make manifests

# Run tests
make test

# Run locally (requires KUBECONFIG)
make install
make run
```

### Building

```bash
# Build binary
make build

# Build and push image
make docker-build docker-push IMG=<your-registry>/pg-operator:tag
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
6. Submit a pull request

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Status

| Component | Status |
|-----------|--------|
| PostGresConnection CRD | ✅ Stable |
| Database CRD | ✅ Stable |
| CNPG Integration | ✅ Stable |
| Cross-namespace support | ✅ Stable |
| Multi-arch images | ✅ Stable |
| Security scanning | ✅ Enabled |

## Documentation

- 📖 [Documentation](docs/) - Architecture, contributing guidelines, and changelog
- 📋 [Examples](examples/) - Usage examples and configurations
- 🔧 [Development Setup](scripts/dev-setup.sh) - One-command development environment

## Support

- 🐛 [Issues](https://github.com/silverswarm/pg-operator/issues)
- 💬 [Discussions](https://github.com/silverswarm/pg-operator/discussions)
#!/usr/bin/env bash
# Development environment setup script
# This script sets up the development environment for the pg-operator

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "🚀 Setting up pg-operator development environment..."

# Check if required tools are installed
check_tool() {
    if ! command -v "$1" &> /dev/null; then
        echo "❌ $1 is required but not installed."
        echo "   Please install it and run this script again."
        exit 1
    else
        echo "✅ $1 is installed"
    fi
}

echo "📋 Checking required tools..."
check_tool "go"
check_tool "docker"
check_tool "kubectl"
check_tool "kind"

# Check Go version
GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | cut -c3-)
REQUIRED_GO_VERSION="1.24"
if [[ "$(printf '%s\n' "$REQUIRED_GO_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_GO_VERSION" ]]; then
    echo "❌ Go version $REQUIRED_GO_VERSION or higher is required (found: $GO_VERSION)"
    exit 1
fi
echo "✅ Go version $GO_VERSION meets requirements"

# Install development dependencies
echo "📦 Installing development dependencies..."
if command -v mise &> /dev/null; then
    echo "  Installing tools with mise..."
    mise install
else
    echo "  Installing kubebuilder manually..."
    # Install kubebuilder if not present
    if ! command -v kubebuilder &> /dev/null; then
        echo "  Downloading kubebuilder..."
        os=$(go env GOOS)
        arch=$(go env GOARCH)
        curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/${os}/${arch}"
        chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
    fi
fi

# Generate code and manifests
echo "🔧 Generating code and manifests..."
make generate manifests

# Setup kind cluster for development
if ! kind get clusters | grep -q "pg-operator-dev"; then
    echo "🔨 Creating kind cluster for development..."
    cat <<EOF | kind create cluster --name pg-operator-dev --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 30000
    protocol: TCP
EOF
else
    echo "✅ Kind cluster 'pg-operator-dev' already exists"
fi

# Install CloudNativePG operator
echo "📦 Installing CloudNativePG operator..."
kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.24/releases/cnpg-1.24.0.yaml

# Wait for CNPG to be ready
echo "⏳ Waiting for CloudNativePG operator to be ready..."
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=cloudnative-pg -n cnpg-system --timeout=300s

# Install CRDs
echo "📋 Installing pg-operator CRDs..."
make install

echo "✅ Development environment setup complete!"
echo ""
echo "🎯 Next steps:"
echo "   1. Run tests: make test"
echo "   2. Run locally: make run"
echo "   3. Build and load image: make docker-build && kind load docker-image controller:latest --name pg-operator-dev"
echo "   4. Deploy to cluster: make deploy IMG=controller:latest"
echo ""
echo "📖 Useful commands:"
echo "   - View cluster info: kubectl cluster-info"
echo "   - Delete kind cluster: kind delete cluster --name pg-operator-dev"
echo "   - Run specific test: go test -v ./internal/controller/..."
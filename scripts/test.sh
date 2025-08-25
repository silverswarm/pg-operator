#!/usr/bin/env bash
# Comprehensive test script for pg-operator
# Runs unit tests, integration tests, and generates coverage reports

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Default values
COVERAGE=false
VERBOSE=false
RACE=false
INTEGRATION=false
E2E=false
PACKAGE="./..."

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--coverage)
            COVERAGE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        -i|--integration)
            INTEGRATION=true
            shift
            ;;
        -e|--e2e)
            E2E=true
            shift
            ;;
        -p|--package)
            PACKAGE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -c, --coverage     Generate coverage report"
            echo "  -v, --verbose      Verbose output"
            echo "  -r, --race         Enable race detector"
            echo "  -i, --integration  Run integration tests"
            echo "  -e, --e2e          Run e2e tests"
            echo "  -p, --package      Specific package to test (default: ./...)"
            echo "  -h, --help         Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option $1"
            exit 1
            ;;
    esac
done

echo "🧪 Running pg-operator tests..."

# Build test flags
TEST_FLAGS=()
if [[ "$VERBOSE" == "true" ]]; then
    TEST_FLAGS+=("-v")
fi
if [[ "$RACE" == "true" ]]; then
    TEST_FLAGS+=("-race")
fi
if [[ "$COVERAGE" == "true" ]]; then
    TEST_FLAGS+=("-coverprofile=coverage.out" "-covermode=atomic")
fi

# Clean previous coverage files
if [[ "$COVERAGE" == "true" ]]; then
    rm -f coverage.out coverage.html
fi

# Run unit tests
if [[ "$E2E" != "true" ]] && [[ "$INTEGRATION" != "true" ]]; then
    echo "📋 Running unit tests..."
    go test "${TEST_FLAGS[@]}" "$PACKAGE"
fi

# Run integration tests
if [[ "$INTEGRATION" == "true" ]]; then
    echo "🔗 Running integration tests..."
    # Check if kind cluster exists
    if ! kind get clusters | grep -q "pg-operator-dev"; then
        echo "❌ Integration tests require a kind cluster named 'pg-operator-dev'"
        echo "   Run: ./scripts/dev-setup.sh to create one"
        exit 1
    fi
    
    # Set test environment
    export USE_EXISTING_CLUSTER=true
    export KUBECONFIG="$HOME/.kube/config"
    
    go test "${TEST_FLAGS[@]}" -tags=integration ./test/...
fi

# Run e2e tests
if [[ "$E2E" == "true" ]]; then
    echo "🎯 Running e2e tests..."
    # Check if kind cluster exists
    if ! kind get clusters | grep -q "pg-operator-dev"; then
        echo "❌ E2E tests require a kind cluster named 'pg-operator-dev'"
        echo "   Run: ./scripts/dev-setup.sh to create one"
        exit 1
    fi
    
    # Deploy operator first
    echo "  Deploying operator for e2e tests..."
    make deploy IMG=controller:latest
    
    # Wait for deployment
    kubectl wait --for=condition=Available deployment/operator-controller-manager -n operator-system --timeout=300s
    
    # Run e2e tests
    export USE_EXISTING_CLUSTER=true
    export KUBECONFIG="$HOME/.kube/config"
    
    go test "${TEST_FLAGS[@]}" -tags=e2e ./test/e2e/...
fi

# Generate coverage report
if [[ "$COVERAGE" == "true" ]] && [[ -f "coverage.out" ]]; then
    echo "📊 Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    
    # Calculate coverage percentage
    COVERAGE_PERCENT=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    echo "📈 Total coverage: $COVERAGE_PERCENT"
    
    if command -v open &> /dev/null; then
        echo "🌐 Opening coverage report in browser..."
        open coverage.html
    else
        echo "📄 Coverage report saved to coverage.html"
    fi
fi

echo "✅ Tests completed successfully!"
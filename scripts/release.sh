#!/usr/bin/env bash
# Release script for pg-operator
# Handles version bumping, tagging, and release preparation

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Default values
DRY_RUN=false
VERSION=""
PUSH=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --push)
            PUSH=true
            shift
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -v, --version VERSION  Version to release (e.g., v1.0.0)"
            echo "  --dry-run              Show what would be done without making changes"
            echo "  --push                 Push changes and tags to origin"
            echo "  -h, --help             Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option $1"
            exit 1
            ;;
    esac
done

# Validate version format
if [[ -z "$VERSION" ]]; then
    echo "❌ Version is required"
    echo "   Usage: $0 --version v1.0.0"
    exit 1
fi

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ Invalid version format. Use semantic versioning (e.g., v1.0.0)"
    exit 1
fi

echo "🚀 Preparing release $VERSION..."

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [[ "$CURRENT_BRANCH" != "main" ]]; then
    echo "❌ Releases must be created from the main branch"
    echo "   Current branch: $CURRENT_BRANCH"
    exit 1
fi

# Check if working tree is clean
if [[ -n $(git status --porcelain) ]]; then
    echo "❌ Working tree is not clean. Please commit or stash changes."
    git status --short
    exit 1
fi

# Check if version already exists
if git tag -l | grep -q "^$VERSION$"; then
    echo "❌ Version $VERSION already exists"
    exit 1
fi

# Update version in files (if VERSION file exists)
if [[ -f "VERSION" ]]; then
    echo "📝 Updating VERSION file..."
    if [[ "$DRY_RUN" == "false" ]]; then
        echo "${VERSION#v}" > VERSION
        git add VERSION
    else
        echo "   Would update VERSION file to ${VERSION#v}"
    fi
fi

# Run tests before release
echo "🧪 Running tests..."
if [[ "$DRY_RUN" == "false" ]]; then
    ./scripts/test.sh --coverage
else
    echo "   Would run: ./scripts/test.sh --coverage"
fi

# Generate manifests
echo "🔧 Generating manifests..."
if [[ "$DRY_RUN" == "false" ]]; then
    make manifests generate
    
    # Check if there are any changes to commit
    if [[ -n $(git status --porcelain) ]]; then
        git add -A
        git commit -m "chore: update generated files for $VERSION"
    fi
else
    echo "   Would run: make manifests generate"
fi

# Build and test container image
echo "🏗️ Building container image..."
if [[ "$DRY_RUN" == "false" ]]; then
    make docker-build IMG="ghcr.io/silverswarm/pg-operator:$VERSION"
else
    echo "   Would build: ghcr.io/silverswarm/pg-operator:$VERSION"
fi

# Create release commit if there are changes
if [[ "$DRY_RUN" == "false" ]] && [[ -n $(git status --porcelain) ]]; then
    git add -A
    git commit -m "chore: prepare release $VERSION"
fi

# Create git tag
echo "🏷️ Creating git tag $VERSION..."
if [[ "$DRY_RUN" == "false" ]]; then
    git tag -a "$VERSION" -m "Release $VERSION"
else
    echo "   Would create tag: $VERSION"
fi

# Generate release manifests
echo "📦 Generating release manifests..."
RELEASE_DIR="dist/release-$VERSION"
if [[ "$DRY_RUN" == "false" ]]; then
    mkdir -p "$RELEASE_DIR"
    
    # Generate install.yaml
    cd config/manager && kustomize edit set image controller="ghcr.io/silverswarm/pg-operator:$VERSION"
    cd "$PROJECT_ROOT"
    kustomize build config/default > "$RELEASE_DIR/install.yaml"
    
    # Generate manifests for different configurations
    kustomize build config/crd > "$RELEASE_DIR/crds.yaml"
    
    echo "   Release manifests generated in $RELEASE_DIR/"
else
    echo "   Would generate manifests in dist/release-$VERSION/"
fi

# Push changes and tags
if [[ "$PUSH" == "true" ]]; then
    echo "📤 Pushing changes and tags to origin..."
    if [[ "$DRY_RUN" == "false" ]]; then
        git push origin main
        git push origin "$VERSION"
    else
        echo "   Would push: git push origin main && git push origin $VERSION"
    fi
fi

echo ""
echo "✅ Release $VERSION prepared successfully!"
echo ""
echo "🎯 Next steps:"
if [[ "$PUSH" != "true" ]]; then
    echo "   1. Review the changes"
    echo "   2. Push to origin: git push origin main && git push origin $VERSION"
fi
echo "   3. Push container image: make docker-push IMG=ghcr.io/silverswarm/pg-operator:$VERSION"
echo "   4. Create GitHub release with manifests from $RELEASE_DIR/"
echo ""
echo "📄 Release artifacts:"
if [[ "$DRY_RUN" == "false" ]]; then
    echo "   - Container image: ghcr.io/silverswarm/pg-operator:$VERSION"
    echo "   - Install manifest: $RELEASE_DIR/install.yaml"
    echo "   - CRD manifest: $RELEASE_DIR/crds.yaml"
fi
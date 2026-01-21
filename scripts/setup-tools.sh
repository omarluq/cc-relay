#!/usr/bin/env bash
# Setup script for cc-relay development tools
# Installs all formatters, linters, and development utilities

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_command() {
    if command -v "$1" &> /dev/null; then
        echo_info "$1 is already installed ($(command -v "$1"))"
        return 0
    else
        return 1
    fi
}

# Check if Go is installed
if ! check_command go; then
    echo_error "Go is not installed. Please install Go 1.24+ from https://golang.org/dl/"
    exit 1
fi

echo_info "Installing development tools for cc-relay..."
echo ""

# Install Go tools
echo_info "Installing Go tools..."

# goimports
if ! check_command goimports; then
    echo_info "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
fi

# gofumpt (stricter gofmt)
if ! check_command gofumpt; then
    echo_info "Installing gofumpt..."
    go install mvdan.cc/gofumpt@latest
fi

# golangci-lint
if ! check_command golangci-lint; then
    echo_info "Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" latest
fi

# govulncheck (security scanning)
if ! check_command govulncheck; then
    echo_info "Installing govulncheck..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
fi

# buf (protobuf linting and generation)
if ! check_command buf; then
    echo_info "Installing buf..."
    go install github.com/bufbuild/buf/cmd/buf@latest
fi

# air (live reload for Go apps)
if ! check_command air; then
    echo_info "Installing air for live reload..."
    go install github.com/air-verse/air@latest
fi

# task (task runner)
if ! check_command task; then
    echo_info "Installing task..."
    go install github.com/go-task/task/v3/cmd/task@latest
fi

echo ""
echo_info "Installing YAML tools..."

# yamlfmt
if ! check_command yamlfmt; then
    echo_info "Installing yamlfmt..."
    go install github.com/google/yamlfmt/cmd/yamlfmt@latest
fi

# yamllint (requires Python)
if ! check_command yamllint; then
    if command -v pip3 &> /dev/null || command -v pip &> /dev/null; then
        echo_info "Installing yamllint..."
        if command -v pip3 &> /dev/null; then
            pip3 install --user yamllint
        else
            pip install --user yamllint
        fi
    else
        echo_warn "pip not found. Skipping yamllint installation."
        echo_warn "Install Python and pip, then run: pip install yamllint"
    fi
fi

echo ""
echo_info "Installing Markdown tools..."

# markdownlint-cli (requires Node.js)
if ! check_command markdownlint; then
    if command -v npm &> /dev/null; then
        echo_info "Installing markdownlint-cli..."
        npm install -g markdownlint-cli
    elif command -v yarn &> /dev/null; then
        echo_info "Installing markdownlint-cli..."
        yarn global add markdownlint-cli
    else
        echo_warn "npm/yarn not found. Skipping markdownlint installation."
        echo_warn "Install Node.js, then run: npm install -g markdownlint-cli"
    fi
fi

echo ""
echo_info "Installing lefthook..."

# lefthook
if ! check_command lefthook; then
    echo_info "Installing lefthook..."
    go install github.com/evilmartians/lefthook@latest
fi

# Install lefthook hooks
if [ -f "lefthook.yml" ]; then
    echo_info "Installing git hooks with lefthook..."
    lefthook install
else
    echo_warn "lefthook.yml not found. Run this script from the project root."
fi

echo ""
echo_info "Setting up Air configuration..."

# Create Air configuration if it doesn't exist
if [ ! -f ".air.toml" ]; then
    air init
    echo_info "Created .air.toml configuration"
fi

echo ""
echo_info "Verifying installations..."
echo ""

# Verify critical tools
MISSING_TOOLS=()

for tool in gofmt goimports gofumpt golangci-lint govulncheck buf yamlfmt lefthook task air; do
    if check_command "$tool"; then
        echo_info "✓ $tool"
    else
        echo_error "✗ $tool - MISSING"
        MISSING_TOOLS+=("$tool")
    fi
done

echo ""
if [ ${#MISSING_TOOLS[@]} -eq 0 ]; then
    echo_info "=========================================="
    echo_info "All tools installed successfully!"
    echo_info "=========================================="
    echo ""
    echo_info "Next steps:"
    echo_info "  1. Run 'lefthook install' to enable git hooks"
    echo_info "  2. Run 'task --list' to see available tasks"
    echo_info "  3. Run 'air' for live reload development"
    echo_info "  4. Make a commit to test the hooks"
    echo ""
else
    echo_warn "=========================================="
    echo_warn "Some tools failed to install:"
    for tool in "${MISSING_TOOLS[@]}"; do
        echo_warn "  - $tool"
    done
    echo_warn "=========================================="
    exit 1
fi

#!/bin/bash

set -euo pipefail

GOPATH=$(go env GOPATH)
GOBINDIR="${GOPATH}/bin"

install_golangci-lint() {
  VER="${GOLANGCILINT_VERSION}"
  if type golangci-lint >/dev/null 2>&1; then
    echo "golangci-lint already installed"
    return
  fi

  echo "Installing golangci-lint. Version: ${VER}"
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@${VER}
}

install_golangci-lint

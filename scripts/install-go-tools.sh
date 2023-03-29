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
  curl -L "https://github.com/golangci/golangci-lint/releases/download/v${VER}/golangci-lint-${VER}-linux-amd64.tar.gz" | tar xzv -C "${GOBINDIR}" "golangci-lint-${VER}-linux-amd64/golangci-lint" --strip 1
}

install_golangci-lint

#!/bin/bash

set -o pipefail

if [[ -x "$(command -v golangci-lint)" ]]; then
  golangci-lint run 
else
  echo "WARNING: gometalinter not found, skipping lint tests" >&2
fi

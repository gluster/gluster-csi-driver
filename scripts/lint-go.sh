#!/bin/bash

set -o pipefail

if [[ -x "$(command -v gometalinter)" ]]; then
  # shellcheck disable=SC2086,SC2068
  gometalinter -j ${GO_METALINTER_THREADS:-1} --sort path --sort line --sort column --deadline=9m \
    --enable="gofmt" --exclude "method NodeGetId should be NodeGetID" \
    --vendor --debug ${@-./...} |& stdbuf -oL grep "linter took\\|:warning:\\|:error:"
else
  echo "WARNING: gometalinter not found, skipping lint tests" >&2
fi

#! /bin/bash

set -e

# Allow overriding default docker command
DOCKER_CMD=${DOCKER_CMD:-docker}

# Allow disabling tests during build
RUN_TESTS=${RUN_TESTS:-1}

VERSION="$(git describe --dirty --always --tags | sed 's/-/./2' | sed 's/-/./2')"
BUILDDATE="$(date -u '+%Y-%m-%dT%H:%M:%S.%NZ')"

$DOCKER_CMD build \
        -t glusterfs-csi-driver \
        --build-arg RUN_TESTS="$RUN_TESTS" \
        --build-arg version="$VERSION" \
        --build-arg builddate="$BUILDDATE" \
        -f pkg/glusterfs/Dockerfile \
        .

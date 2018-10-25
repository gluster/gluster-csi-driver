#! /bin/bash

set -e

# Set driver name
DRIVER="${DRIVER:-glusterfs-csi-driver}"

# Set which docker repo to tag
REPO="${REPO:-gluster/}"

# Base image to use for final container images
FINAL_BASE="${FINAL_BASE:-centos:7.5.1804}"

# Allow overriding default docker command
DOCKER_CMD=${DOCKER_CMD:-docker}

# Allow disabling tests during build
RUN_TESTS=${RUN_TESTS:-1}

# Cleanup build context when done
RM_BUILD=${RM_BUILD:-1}

VERSION="$(git describe --dirty --always --tags | sed 's/-/./2' | sed 's/-/./2')"
BUILDDATE="$(date -u '+%Y-%m-%dT%H:%M:%S.%NZ')"

GO_DEP_VERSION="${GO_DEP_VERSION}"
GO_METALINTER_VERSION="${GO_METALINTER_VERSION:-v2.0.11}"
GO_METALINTER_THREADS=${GO_METALINTER_THREADS:-4}

build_args=()
build_args+=( --build-arg "RUN_TESTS=$RUN_TESTS" )
build_args+=( --build-arg "GO_DEP_VERSION=$GO_DEP_VERSION" )
build_args+=( --build-arg "GO_METALINTER_VERSION=$GO_METALINTER_VERSION" )
build_args+=( --build-arg "GO_METALINTER_THREADS=$GO_METALINTER_THREADS" )
build_args+=( --build-arg "FINAL_BASE=$FINAL_BASE" )
build_args+=( --build-arg "version=$VERSION" )
build_args+=( --build-arg "builddate=$BUILDDATE" )

# Print Docker version
echo "=== Docker Version ==="
$DOCKER_CMD version

# Run container build
$DOCKER_CMD build \
        -t glusterfs-csi-driver-build \
        --target build \
        "${build_args[@]}" \
        -f pkg/glusterfs/Dockerfile \
        . \
|| exit 1

#-- Build final container
$DOCKER_CMD build \
        -t "${REPO}${DRIVER}" \
        --target driver \
        "${build_args[@]}" \
        -f pkg/glusterfs/Dockerfile \
        . \
|| exit 1

# If running tests, extract profile data
if [ "$RUN_TESTS" -ne 0 ]; then
        rm -f profile.cov
        $DOCKER_CMD run glusterfs-csi-driver-build \
                cat /profile.cov > profile.cov
fi

if [ "$RM_BUILD" -ne 0 ]; then
        $DOCKER_CMD rmi -f glusterfs-csi-driver-build
fi

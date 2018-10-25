#! /bin/bash

set -e

# Allow overriding default docker command
DOCKER_CMD=${DOCKER_CMD:-docker}

# Allow disabling tests during build
RUN_TESTS=${RUN_TESTS:-1}

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
        -t glusterfs-csi-driver \
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

$DOCKER_CMD rmi glusterfs-csi-driver-build

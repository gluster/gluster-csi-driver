#! /bin/bash

set -e

# Set which drivers to build
DRIVERS="${DRIVERS:-glusterfs-controller glusterfs-node}"

# Set which docker repo to tag
REPO="${REPO:-gluster/}"

# Allow overriding default docker command
RUNTIME_CMD=${RUNTIME_CMD:-docker}

build="build"
if [[ "${RUNTIME_CMD}" == "buildah" ]]; then
  build="bud"
fi

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
echo "=== $RUNTIME_CMD version ==="
$RUNTIME_CMD version

#-- Build containers
for driver in ${DRIVERS}; do
        $RUNTIME_CMD $build \
                -t "${REPO}${driver}-csi-driver" \
                --build-arg DRIVER="$driver" \
                "${build_args[@]}" \
                -f extras/Dockerfile \
                . \
        || exit 1

        # If running tests, extract profile data
        if [ "$RUN_TESTS" -ne 0 ]; then
                rm -f profile.cov
                $RUNTIME_CMD run --entrypoint cat "${REPO}${driver}-csi-driver" \
                        /profile.cov > profile.cov
        fi
done

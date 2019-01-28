#! /bin/bash

set -e

# Set driver name
DRIVER="${DRIVER:-glusterfs-csi-driver}"

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
GO_METALINTER_VERSION="${GO_METALINTER_VERSION:-v2.0.12}"
GO_METALINTER_THREADS=${GO_METALINTER_THREADS:-4}

build_args=()
build_args+=(--build-arg "RUN_TESTS=$RUN_TESTS")
build_args+=(--build-arg "GO_DEP_VERSION=$GO_DEP_VERSION")
build_args+=(--build-arg "GO_METALINTER_VERSION=$GO_METALINTER_VERSION")
build_args+=(--build-arg "GO_METALINTER_THREADS=$GO_METALINTER_THREADS")
build_args+=(--build-arg "version=$VERSION")
build_args+=(--build-arg "builddate=$BUILDDATE")

# Print Docker version
echo "=== $RUNTIME_CMD version ==="
$RUNTIME_CMD version

#-- Build glusterfs csi driver container
$RUNTIME_CMD $build \
	-t "${REPO}${DRIVER}" \
	"${build_args[@]}" \
	-f pkg/glusterfs/Dockerfile \
	. ||
	exit 1

# If running tests, extract profile data
if [ "$RUN_TESTS" -ne 0 ]; then
	rm -f profile.cov
	$RUNTIME_CMD run --entrypoint cat "${REPO}${DRIVER}" \
		/profile.cov >profile.cov
fi

DRIVER="glustervirtblock-csi-driver"

#-- Build gluster block csi driver container
$RUNTIME_CMD $build \
	-t "${REPO}${DRIVER}" \
	"${build_args[@]}" \
	-f pkg/gluster-virtblock/Dockerfile \
	. ||
	exit 1

# If running tests, extract profile data
if [ "$RUN_TESTS" -ne 0 ]; then
	rm -f profile.cov
	$RUNTIME_CMD run --entrypoint cat "${REPO}${DRIVER}" \
		/profile.cov > profile.cov
fi

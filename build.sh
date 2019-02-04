#! /bin/bash

set -e -o pipefail

# Set which docker repo to use
REPO="${REPO:-gluster}"

# Allow overriding default docker command
RUNTIME_CMD=${RUNTIME_CMD:-docker}

GO_DEP_VERSION="${GO_DEP_VERSION}"
GO_METALINTER_VERSION="${GO_METALINTER_VERSION:-v3.0.0}"
GO_METALINTER_THREADS=${GO_METALINTER_THREADS:-4}

build="build"
if [[ "${RUNTIME_CMD}" == "buildah" ]]; then
	build="bud"
fi

# Allow disabling tests during build
RUN_TESTS=${RUN_TESTS:-1}

SCRIPT=$(basename "$0")

function usage {
	cmd="$1"
	cat - <<USAGE
Usage: $cmd <drivername>

Available drivers:
    glusterfs
    glustervirtblock
USAGE
}

if [[ $# -ne 1 ]]; then
	echo "ERROR: No driver name specified."
	usage "$SCRIPT"
	exit 1
fi

case $1 in
glusterfs)
	DRIVER=glusterfs-csi-driver
	DOCKERFILE=pkg/glusterfs/Dockerfile
	;;
glustervirtblock)
	DRIVER=glustervirtblock-csi-driver
	DOCKERFILE=pkg/gluster-virtblock/Dockerfile
	;;
*)
	echo "ERROR: Invalid driver name specified."
	usage "$SCRIPT"
	exit 1
	;;
esac


# This sets the version variable to (hopefully) a semver compatible string. We
# expect released versions to have a tag of vX.Y.Z (with Y & Z optional), so we
# only look for those tags. For version info on non-release commits, we want to
# include the git commit info as a "build" suffix ("+stuff" at the end). There
# is also special casing here for when no tags match.
VERSION_GLOB="v[0-9]*"
# Get the nearest "version" tag if one exists. If not, this returns the full
# git hash
NEAREST_TAG="$(git describe --always --tags --match "$VERSION_GLOB" --abbrev=0)"
# Full output of git describe for us to parse: TAG-<N>-g<hash>-<dirty>
FULL_DESCRIBE="$(git describe --always --tags --match "$VERSION_GLOB" --dirty)"
# If full matches against nearest, we found a valid tag earlier
if [[ $FULL_DESCRIBE =~ ${NEAREST_TAG}-(.*) ]]; then
        # Build suffix is the last part of describe w/ "-" replaced by "."
        VERSION="$NEAREST_TAG+${BASH_REMATCH[1]//-/.}"
else
        # We didn't find a valid tag, so assume version 0 and everything ends up
        # in build suffix.
        VERSION="0.0.0+g${FULL_DESCRIBE//-/.}"
fi

BUILDDATE="$(date -u '+%Y-%m-%dT%H:%M:%S.%NZ')"


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
	-t "${REPO}/${DRIVER}" \
	"${build_args[@]}" \
	--network host \
	-f "$DOCKERFILE" \
	. ||
	exit 1

# If running tests, extract profile data
if [ "$RUN_TESTS" -ne 0 ]; then
	rm -f profile.cov
	$RUNTIME_CMD run --entrypoint cat "${REPO}/${DRIVER}" \
		/profile.cov > profile.cov
fi

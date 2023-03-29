# Copyright 2018 The Gluster CSI Authors.

# Licensed under GNU LESSER GENERAL PUBLIC LICENSE Version 3, 29 June 2007
# You may obtain a copy of the License at
#    https://opensource.org/licenses/lgpl-3.0.html

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#-- Create build environment

FROM golang:1.20 as build

ENV GOPATH="/go/" \
    SRCDIR="/go/src/github.com/gluster/gluster-csi-driver/"
ENV SCRIPTSDIR="${SRCDIR}scripts/"

# Install go tools
ARG GOLANGCILINT_VERSION=latest
COPY scripts/install-go-tools.sh "${SCRIPTSDIR}"
RUN mkdir -p /go/bin; ${SCRIPTSDIR}/install-go-tools.sh

# Vendor dependencies
COPY go.mod go.sum "${SRCDIR}/"
WORKDIR "${SRCDIR}"
RUN go mod download

# Copy source directories
COPY cmd/ "${SRCDIR}/cmd"
COPY pkg/ "${SRCDIR}/pkg"
COPY scripts/ "${SCRIPTSDIR}"

#-- Test phase

ARG RUN_TESTS=1
ARG GO_METALINTER_THREADS=1
ENV TEST_COVERAGE=stdout \
    GO_COVER_DIR=/build/

RUN mkdir /build && \
    [ $RUN_TESTS -eq 0 ] || ( ${SCRIPTSDIR}/lint-go.sh && \
    ${SCRIPTSDIR}/test-go.sh ) && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o /build/glusterfs-csi-driver cmd/glusterfs/main.go

#-- Final container

FROM fedora:37 as final

# Install dependencies
RUN yum update -y && \
    yum -y install glusterfs-fuse && \
    yum clean all -y && \
    rm -rf /var/cache/yum && \
    rpm -qa | grep gluster | tee /gluster-rpm-versions.txt

# Copy glusterfs-csi-driver from build phase
COPY --from=build /build /

# The version of the driver (git describe --dirty --always --tags | sed 's/-/./2' | sed 's/-/./2')
ARG version="(unknown)"
# Container build time (date -u '+%Y-%m-%dT%H:%M:%S.%NZ')
ARG builddate="(unknown)"

LABEL build-date="${builddate}"
LABEL io.k8s.description="FUSE-based CSI driver for Gluster file access"
LABEL name="glusterfs-csi-driver"
LABEL summary="FUSE-based CSI driver for Gluster file access"
LABEL vcs-type="git"
LABEL vcs-url="https://github.com/gluster/gluster-csi-driver"
LABEL vendor="gluster.org"
LABEL version="${version}"

ENTRYPOINT ["/glusterfs-csi-driver"]

# Copyright 2018 The Gluster CSI Authors.

# Licensed under GNU LESSER GENERAL PUBLIC LICENSE Version 3, 29 June 2007
# You may obtain a copy of the License at
#    https://opensource.org/licenses/lgpl-3.0.html

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


#-- Build phase
FROM openshift/origin-release:golang-1.10 AS build

ENV GOPATH="/go/" \
    SRCDIR="/go/src/github.com/gluster/gluster-csi-driver/"

RUN yum install -y \
    git

# Install dep
RUN mkdir -p /go/bin
RUN curl -L https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Install gometalinter
ARG GO_METALINTER_VERSION=v2.0.11
RUN curl -L 'https://raw.githubusercontent.com/alecthomas/gometalinter/master/scripts/install.sh' \
    | bash -s -- -b "${GOPATH}bin" "${GO_METALINTER_VERSION}"

# Vendor dependencies
COPY Gopkg.lock Gopkg.toml "${SRCDIR}"
WORKDIR "${SRCDIR}"
RUN /go/bin/dep ensure -v -vendor-only

# Build executable
COPY . "${SRCDIR}"
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o /glusterfs-csi-driver cmd/glusterfs/main.go

# Ensure the binary is statically linked
RUN ldd /glusterfs-csi-driver | grep -q "not a dynamic executable"

# Run tests
ARG RUN_TESTS=1
RUN [ $RUN_TESTS -eq 0 ] || { \
      set -o pipefail \
      && gometalinter -j4 --sort=path --sort=line --sort=column \
      --enable="gofmt" \
      --exclude="method NodeGetId should be NodeGetID" \
      --deadline 9m --vendor --debug ./... \
      |& stdbuf -oL awk '/linter took/ || !/^DEBUG/'; \
    }
RUN [ $RUN_TESTS -eq 0 ] || { \
      GOPACKAGES="$(go list ./... | grep -v vendor | grep -v e2e)"; \
      go test -covermode=count -coverprofile=/profile.cov $GOPACKAGES; \
    }


#-- Final container
FROM centos:7.5.1804

# Install dependencies
RUN yum -y install centos-release-gluster && \
    yum -y install glusterfs-fuse && \
    yum clean all -y && \
    rm -rf /var/cache/yum && \
    rpm -qa | grep gluster | tee /gluster-rpm-versions.txt

# Copy glusterfs-csi-driver from build phase
COPY --from=build /glusterfs-csi-driver /glusterfs-csi-driver

# The version of the driver (git describe --dirty --always --tags | sed 's/-/./2' | sed 's/-/./2')
ARG version="(unknown)"
# Container build time (date -u '+%Y-%m-%dT%H:%M:%S.%NZ')
ARG builddate="(unknown)"

LABEL build-date="${builddate}"
LABEL io.k8s.description="FUSE-based CSI driver for Gluster file access"
LABEL name="glusterfs-csi-driver"
LABEL Summary="FUSE-based CSI driver for Gluster file access"
LABEL vcs-type="git"
LABEL vcs-url="https://github.com/gluster/gluster-csi-driver"
LABEL vendor="gluster.org"
LABEL version="${version}"

ENTRYPOINT ["/glusterfs-csi-driver"]

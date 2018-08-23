#!/bin/bash

# This script sets up the centos-ci environment and runs the PR tests for gluster-csi-driver.

# if anything fails, we'll abort
set -e

REQ_GO_VERSION='1.9.4'
# install Go
if ! yum -y install "golang >= $REQ_GO_VERSION"
then
	# not the right version, install manually
	# download URL comes from https://golang.org/dl/
	curl -O https://storage.googleapis.com/golang/go${REQ_GO_VERSION}.linux-amd64.tar.gz
	tar xzf go${REQ_GO_VERSION}.linux-amd64.tar.gz -C /usr/local
	export PATH=$PATH:/usr/local/go/bin
fi

# also needs git, hg, bzr, svn gcc and make
yum -y install git mercurial bzr subversion gcc make

export CSISRC=$GOPATH/src/github.com/gluster/gluster-csi-driver
cd "$CSISRC"

# install the build and test requirements
./scripts/install-reqs.sh

# install vendored dependencies
make vendor-install

# verify build
make csi-driver

# run tests
make test TESTOPTIONS=-v

#!/bin/bash

# This script sets up the centos-ci environment and runs the PR tests for gluster-csi-driver.

# if anything fails, we'll abort
set -e

# Install buildah & Docker
yum-config-manager \
        --add-repo \
        https://download.docker.com/linux/centos/docker-ce.repo
yum -y install buildah docker-ce
systemctl start docker

# Build glusterfs and glustervirtblock containers
./build.sh glusterfs
./build.sh glustervirtblock

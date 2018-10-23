#!/bin/bash

GOPATH=$(go env GOPATH)
GOBINDIR="${GOPATH}/bin"

install_dep() {
  DEPVER="${GO_DEP_VERSION}"
  if type dep >/dev/null 2>&1; then
    local version
    version=$(dep version | awk '/^ version/{print $3}')
    if [[ "${version}" == "${DEPVER}" || ${version} >  ${DEPVER} ]]; then
      echo "dep ${DEPVER} or greater is already installed"
      return
    fi
  fi

  echo "Installing dep. Version: ${DEPVER:-latest}"
  export INSTALL_DIRECTORY="${GOBINDIR}"
  export DEP_RELEASE_TAG="${DEPVER}"
  curl -L  https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
}

install_gometalinter() {
  GMLVER="${GO_METALINTER_VERSION}"
  if type gometalinter >/dev/null 2>&1; then
    echo "gometalinter already installed"
    return
  fi

  echo "Installing gometalinter. Version: ${GMLVER}"
  curl -L https://raw.githubusercontent.com/alecthomas/gometalinter/master/scripts/install.sh | bash -s -- -b "${GOBINDIR}" "${GMLVER}"
}

install_dep
install_gometalinter

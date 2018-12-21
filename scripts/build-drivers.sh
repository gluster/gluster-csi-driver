#!/bin/bash

set -e

DRIVERS=( "${@}" )
BUILDDIR="${BUILDDIR:-build}"

if [ ${#DRIVERS[@]} -eq 0 ]; then
  while IFS=$'\n' read -r line; do DRIVERS+=("$line"); done < <(ls cmd)
fi

mkdir -p "${BUILDDIR}"

while [ ${#DRIVERS[@]} -ne 0 ]; do
  DRIVER="${DRIVERS[0]}"
  CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o "${BUILDDIR}/${DRIVER}-csi-driver" "cmd/${DRIVER}/main.go"
  ldd "${BUILDDIR}/${DRIVER}-csi-driver" | grep -q "not a dynamic executable"
  DRIVERS=( "${DRIVERS[@]:1}" )
done

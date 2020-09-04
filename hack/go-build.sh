#!/usr/bin/env bash

set -eu

REPO=github.com/openshift/cluster-baremetal-operator
WHAT=${1:-cluster-baremetal-operator}
GLDFLAGS=${GLDFLAGS:-}

eval $(go env | grep -e "GOHOSTOS" -e "GOHOSTARCH")

: "${GOOS:=${GOHOSTOS}}"
: "${GOARCH:=${GOHOSTARCH}}"

# Go to the root of the repo
cd "$(git rev-parse --show-cdup)"

GLDFLAGS+="-extldflags '-static'"

eval $(go env)

echo "Building ${REPO}/controllers to bin/${WHAT}"
GO111MODULE=${GO111MODULE} CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS} -ldflags "${GLDFLAGS}" -o bin/${WHAT} ${REPO}


#!/bin/bash
if [ "$IS_CONTAINER" != "" ]; then
  ALL_FILES=$(find . -name '*.go' ! -path '*/vendor/*' ! -path '*/.build/*')
  BAD_FILES=$(gofmt -l ${ALL_FILES})
  if [ -n "${BAD_FILES})" ]; then
     for f in $BAD_FILES; do
         echo $f
         gofmt -s -w $f
     done
     git diff
     exit 1
  fi
else
  docker run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/openshift/machine-api-operator:z" \
    --workdir /go/src/github.com/openshift/machine-api-operator \
    --env GO111MODULE="$GO111MODULE" \
    --env GOFLAGS="$GOFLAGS" \
    openshift/origin-release:golang-1.13 \
    ./hack/go-fmt.sh "${@}"
fi

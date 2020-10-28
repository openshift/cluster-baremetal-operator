#!/bin/sh

# Ignore the rule that says we should always quote variables, because
# in this script we *do* want globbing.
# shellcheck disable=SC2086

set -e

ARTIFACTS=${ARTIFACTS:-/tmp}

eval "$(go env)"
cd "${GOPATH}"/src/github.com/openshift/cluster-baremetal-operator
export XDG_CACHE_HOME="/tmp/.cache"

INPUT_FILES="manifests/*.yaml"
cksum $INPUT_FILES > "$ARTIFACTS/lint.cksums.before"
make manifests
cksum $INPUT_FILES > "$ARTIFACTS/lint.cksums.after"
diff "$ARTIFACTS/lint.cksums.before" "$ARTIFACTS/lint.cksums.after"

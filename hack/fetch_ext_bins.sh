#!/usr/bin/env bash
# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# Enable tracing in this script off by setting the TRACE variable in your
# environment to any value:
#
# $ TRACE=1 test.sh
TRACE=${TRACE:-""}
if [ -n "$TRACE" ]; then
  set -x
fi

# Kubernetes version for envtest binaries
# Note: The old kubebuilder-tools GCS bucket is deprecated.
k8s_version=1.34.0

# Turn colors in this script off by setting the NO_COLOR variable in your
# environment to any value:
#
# $ NO_COLOR=1 test.sh
NO_COLOR=${NO_COLOR:-""}
if [ -z "$NO_COLOR" ]; then
  header=$'\e[1;33m'
  reset=$'\e[0m'
else
  header=''
  reset=''
fi

function header_text {
  echo "$header$*$reset"
}

# Skip fetching and untaring the tools by setting the SKIP_FETCH_TOOLS variable
# in your environment to any value:
#
# $ SKIP_FETCH_TOOLS=1 ./fetch_ext_bins.sh
#
# If you skip fetching tools, this script will use the tools already on your
# machine.
SKIP_FETCH_TOOLS=${SKIP_FETCH_TOOLS:-""}

# fetch k8s API gen tools using setup-envtest
function fetch_tools {
  if [ -n "$SKIP_FETCH_TOOLS" ]; then
    return 0
  fi

  header_text "fetching tools"
  
  # Use setup-envtest (vendored) to download the binaries
  # This replaces the deprecated kubebuilder-tools GCS bucket
  go run sigs.k8s.io/controller-runtime/tools/setup-envtest use "$k8s_version" --bin-dir /tmp/kubebuilder/bin -p path
}

function setup_envs {
  header_text "setting up env vars"

  # Setup env vars for envtest
  export KUBEBUILDER_ASSETS=/tmp/kubebuilder/bin/k8s/$k8s_version-$(go env GOOS)-$(go env GOARCH)
  export PATH=$KUBEBUILDER_ASSETS:$PATH
  export TEST_ASSET_KUBECTL=$KUBEBUILDER_ASSETS/kubectl
  export TEST_ASSET_KUBE_APISERVER=$KUBEBUILDER_ASSETS/kube-apiserver
  export TEST_ASSET_ETCD=$KUBEBUILDER_ASSETS/etcd
}

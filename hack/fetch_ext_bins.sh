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

# Kubernetes version for envtest binaries (should match k8s.io/api version in go.mod)
# This branch uses k8s.io/api v0.30.1
k8s_version=1.30.0

# setup-envtest version (from release-0.19, compatible with Go 1.22+)
# Note: release-0.18 uses deprecated GCS bucket that returns 401
setup_envtest_version="v0.0.0-20250308055145-5fe7bb3edc86"

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

  # Use setup-envtest to download the Kubernetes binaries
  # Using go run with -mod=mod to bypass vendor mode restriction
  # release-0.19 has the fix for the deprecated GCS bucket (release-0.18 returns 401)
  header_text "downloading envtest binaries for k8s ${k8s_version}"
  go run -mod=mod "sigs.k8s.io/controller-runtime/tools/setup-envtest@${setup_envtest_version}" use "$k8s_version" --bin-dir /tmp/kubebuilder/bin -p path
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

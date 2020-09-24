#!/bin/bash
set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/..
source $REPO_ROOT/hack/utils.sh

# Generate manifests
go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go "crd:trivialVersions=true" rbac:roleName=manager-role webhook paths=./... output:crd:artifacts:config=config/crd/bases
sed -i '/^    controller-gen.kubebuilder.io\/version: (devel)/d' config/crd/bases/*

# Copy crds into the manifests
install_crd \
  "config/crd/bases/metal3.io_provisionings.yaml" \
  "manifests/0000_30_cluster-baremetal-operator_00_metal3provisioning.crd.yaml"

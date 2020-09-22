#!/bin/bash
set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/..
source $REPO_ROOT/hack/utils.sh

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS="crd:trivialVersions=true,crdVersions=v1"

# Generate manifests
go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go $CRD_OPTIONS rbac:roleName=manager-role webhook paths=./... output:crd:artifacts:config=config/crd/bases
sed -i '/^    controller-gen.kubebuilder.io\/version: (devel)/d' config/crd/bases/*

# Copy crds into the manifests
install_crd \
  "config/crd/bases/metal3.io_provisionings.yaml" \
  "manifests/0000_31_cluster-baremetal-operator_02_metal3provisioning.crd.yaml"

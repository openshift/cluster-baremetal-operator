#!/bin/bash
set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/..
source $REPO_ROOT/hack/utils.sh

# install bmh crd
tmpfile="$(mktemp bmhcrd-XXXXXXXX)"
curl -o $tmpfile https://raw.githubusercontent.com/openshift/baremetal-operator/master/deploy/crds/metal3.io_baremetalhosts_crd.yaml
if [ $? -eq 0 ]; then
	install_crd $tmpfile "manifests/0000_30_cluster-baremetal-operator_01_baremetalhost.crd.yaml"
fi
rm -f $tmpfile

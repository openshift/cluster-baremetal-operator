#!/bin/bash
set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE}")/..
source $REPO_ROOT/hack/utils.sh

# install bmh crd
tmpfile="$(mktemp bmhcrd-XXXXXXXX)"
curl -o $tmpfile https://raw.githubusercontent.com/metal3-io/baremetal-operator/master/config/crd/bases/metal3.io_baremetalhosts.yaml
if [ $? -eq 0 ]; then
	install_crd $tmpfile "manifests/0000_31_cluster-baremetal-operator_03_baremetalhost.crd.yaml"
fi
rm -f $tmpfile

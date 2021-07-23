package provisioning

import (
	"testing"

	"github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func Test_imageCacheProvisioningOSDownloadURL(t *testing.T) {
	input := "https://releases-art-rhcos.svc.ci.openshift.org/art/storage/releases/rhcos-4.2/42.80.20190725.1/rhcos-42.80.20190725.1-openstack.qcow2?sha256sum=123"
	want := "http://metal3-state.namespace.svc.cluster.local:6180/images/rhcos-42.80.20190725.1-openstack.qcow2/rhcos-42.80.20190725.1-openstack.qcow2"
	got, err := imageCacheProvisioningOSDownloadURL("namespace", v1alpha1.ProvisioningSpec{ProvisioningOSDownloadURL: input})
	if err != nil {
		t.Errorf("imageCacheProvisioningOSDownloadURL() error = %v", err)
		return
	}
	if got != want {
		t.Errorf("imageCacheProvisioningOSDownloadURL() = %v, want %v", got, want)
	}
}

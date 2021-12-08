/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioning

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestGetMetal3DeploymentConfig(t *testing.T) {
	tCases := []struct {
		name          string
		configName    string
		spec          *metal3iov1alpha1.ProvisioningSpec
		expectedValue string
	}{
		{
			name:          "Managed ProvisioningIPCIDR",
			configName:    provisioningIP,
			spec:          managedProvisioning().build(),
			expectedValue: "172.30.20.3/24",
		},
		{
			name:          "Managed ProvisioningInterface",
			configName:    provisioningInterface,
			spec:          managedProvisioning().build(),
			expectedValue: "eth0",
		},
		{
			name:          "Unmanaged DeployKernelUrl",
			configName:    deployKernelUrl,
			spec:          unmanagedProvisioning().build(),
			expectedValue: "http://localhost:6181/images/ironic-python-agent.kernel",
		},
		{
			name:          "Disabled DeployKernelUrl",
			configName:    deployKernelUrl,
			spec:          disabledProvisioning().build(),
			expectedValue: "http://localhost:6181/images/ironic-python-agent.kernel",
		},
		{
			name:          "Disabled IronicEndpoint",
			configName:    ironicEndpoint,
			spec:          disabledProvisioning().build(),
			expectedValue: "https://localhost:6385/v1/",
		},
		{
			name:          "Disabled InspectorEndpoint",
			configName:    ironicInspectorEndpoint,
			spec:          disabledProvisioning().build(),
			expectedValue: "https://localhost:5050/v1/",
		},
		{
			name:          "Unmanaged HttpPort",
			configName:    httpPort,
			spec:          unmanagedProvisioning().build(),
			expectedValue: "6180",
		},
		{
			name:          "Managed DHCPRange",
			configName:    dhcpRange,
			spec:          managedProvisioning().build(),
			expectedValue: "172.30.20.11,172.30.20.101,24",
		},
		{
			name:          "Managed IPv6 DHCPRange",
			configName:    dhcpRange,
			spec:          managedIPv6Provisioning().build(),
			expectedValue: "fd2e:6f44:5dd8:b856::10,fd2e:6f44:5dd8:b856::ff,80",
		},
		{
			name:          "Disabled DHCPRange",
			configName:    dhcpRange,
			spec:          disabledProvisioning().build(),
			expectedValue: "",
		},
		{
			name:          "Disabled RhcosImageUrl",
			configName:    machineImageUrl,
			spec:          disabledProvisioning().build(),
			expectedValue: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			actualValue := getMetal3DeploymentConfig(tc.configName, tc.spec)
			assert.NotNil(t, actualValue)
			assert.Equal(t, tc.expectedValue, *actualValue, fmt.Sprintf("%s : Expected : %s Actual : %s", tc.configName, tc.expectedValue, *actualValue))
			return
		})
	}
}

type provisioningBuilder struct {
	metal3iov1alpha1.ProvisioningSpec
}

func managedProvisioning() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "eth0",
			ProvisioningIP:            "172.30.20.3",
			ProvisioningMacAddresses:  []string{"34:b3:2d:81:f8:fb", "34:b3:2d:81:f8:fc", "34:b3:2d:81:f8:fd"},
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningDHCPRange:     "172.30.20.11,172.30.20.101",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Managed",
		},
	}
}

func managedIPv6Provisioning() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "eth0",
			ProvisioningIP:            "fd2e:6f44:5dd8:b856::2",
			ProvisioningNetworkCIDR:   "fd2e:6f44:5dd8:b856::0/80",
			ProvisioningMacAddresses:  []string{"34:b3:2d:81:f8:fb", "34:b3:2d:81:f8:fc", "34:b3:2d:81:f8:fd"},
			ProvisioningDHCPRange:     "fd2e:6f44:5dd8:b856::10,fd2e:6f44:5dd8:b856::ff",
			ProvisioningOSDownloadURL: "http://[fd2e:6f44:5dd8:b856::2]/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Managed",
		},
	}
}

func unmanagedProvisioning() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "ensp0",
			ProvisioningIP:            "172.30.20.3",
			ProvisioningMacAddresses:  []string{"34:b3:2d:81:f8:fb", "34:b3:2d:81:f8:fc", "34:b3:2d:81:f8:fd"},
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Unmanaged",
		},
	}
}

func disabledProvisioning() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "",
			ProvisioningMacAddresses:  []string{"34:b3:2d:81:f8:fb", "34:b3:2d:81:f8:fc", "34:b3:2d:81:f8:fd"},
			ProvisioningIP:            "172.30.20.3",
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Disabled",
		},
	}
}

func configWithPreProvisioningOSDownloadURLs() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "eth0",
			ProvisioningIP:            "172.30.20.3",
			ProvisioningMacAddresses:  []string{"34:b3:2d:81:f8:fb", "34:b3:2d:81:f8:fc", "34:b3:2d:81:f8:fd"},
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningDHCPRange:     "172.30.20.11,172.30.20.101",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Managed",
			PreProvisioningOSDownloadURLs: metal3iov1alpha1.PreProvisioningOSDownloadURLs{
				KernelURL:    "http://172.22.0.1/images/rhcos-49.84.202107010027-0-live-kernel-x86_64",
				InitramfsURL: "http://172.22.0.1/images/rhcos-49.84.202107010027-0-live-initramfs.x86_64.img",
				RootfsURL:    "http://172.22.0.1/images/rhcos-49.84.202107010027-0-live-rootfs.x86_64.img",
				IsoURL:       "http://172.22.0.1/images/rhcos-4.9/49.84.202107010027-0/x86_64/rhcos-49.84.202107010027-0-live.x86_64.iso",
			},
		},
	}
}

func (pb *provisioningBuilder) build() *metal3iov1alpha1.ProvisioningSpec {
	return &pb.ProvisioningSpec
}

func (pb *provisioningBuilder) ProvisioningInterface(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningInterface = value
	return pb
}

func (pb *provisioningBuilder) ProvisioningIP(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningIP = value
	return pb
}

func (pb *provisioningBuilder) ProvisioningDHCPExternal(value bool) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningDHCPExternal = value
	return pb
}

func (pb *provisioningBuilder) ProvisioningNetworkCIDR(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningNetworkCIDR = value
	return pb
}

func (pb *provisioningBuilder) ProvisioningDHCPRange(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningDHCPRange = value
	return pb
}

func (pb *provisioningBuilder) VirtualMediaViaExternalNetwork(value bool) *provisioningBuilder {
	pb.ProvisioningSpec.VirtualMediaViaExternalNetwork = value
	return pb
}

func (pb *provisioningBuilder) ProvisioningNetwork(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningNetwork = metal3iov1alpha1.ProvisioningNetwork(value)
	return pb
}

func (pb *provisioningBuilder) ProvisioningOSDownloadURL(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningOSDownloadURL = value
	return pb
}

func enableMultiNamespace() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "",
			ProvisioningIP:            "172.30.20.3",
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Disabled",
			WatchAllNamespaces:        true,
		},
	}
}

func disableMultiNamespace() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "",
			ProvisioningIP:            "172.30.20.3",
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Disabled",
			WatchAllNamespaces:        false,
		},
	}
}

func (pb *provisioningBuilder) WatchAllNamespaces(value bool) *provisioningBuilder {
	pb.ProvisioningSpec.WatchAllNamespaces = value
	return pb
}

func TestWatchAllNamespaces(t *testing.T) {
	tCases := []struct {
		name          string
		spec          *metal3iov1alpha1.ProvisioningSpec
		expectedValue bool
	}{
		{
			name:          "Default",
			spec:          managedProvisioning().build(),
			expectedValue: false,
		},
		{
			name:          "Single Namespace",
			spec:          disableMultiNamespace().build(),
			expectedValue: false,
		},
		{
			name:          "Multiple Namespaces",
			spec:          enableMultiNamespace().build(),
			expectedValue: true,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			assert.NotNil(t, tc.spec.WatchAllNamespaces)
			assert.Equal(t, tc.expectedValue, tc.spec.WatchAllNamespaces, fmt.Sprintf("WatchAllNamespaces : Expected : %s Actual : %s", strconv.FormatBool(tc.expectedValue), strconv.FormatBool(tc.spec.WatchAllNamespaces)))
			return
		})
	}
}

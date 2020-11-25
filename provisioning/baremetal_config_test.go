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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const testBaremetalProvisioningCR = "test-provisioning-configuration"

func TestValidateManagedProvisioningConfig(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testBaremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name          string
		spec          *metal3iov1alpha1.ProvisioningSpec
		expectedError bool
		expectedMode  metal3iov1alpha1.ProvisioningNetwork
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name:          "ValidManaged",
			spec:          managedProvisioning().build(),
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
		},
		{
			// ProvisioningNetwork is not specified and ProvisioningDHCPExternal is the default value
			name:          "ImpliedManaged",
			spec:          managedProvisioning().ProvisioningNetwork("").build(),
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
		},
		{
			// ProvisioningInterface is not specified.
			name:          "InvalidManaged",
			spec:          managedProvisioning().ProvisioningInterface("").build(),
			expectedError: true,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
			expectedMsg:   "ProvisioningInterface",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := ValidateBaremetalProvisioningConfig(baremetalCR)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, getProvisioningNetworkMode(baremetalCR), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
			}
			return
		})
	}
}

func TestValidateUnmanagedProvisioningConfig(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testBaremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name          string
		spec          *metal3iov1alpha1.ProvisioningSpec
		expectedError bool
		expectedMode  metal3iov1alpha1.ProvisioningNetwork
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name:          "ValidUnmanaged",
			spec:          unmanagedProvisioning().build(),
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkUnmanaged,
		},
		{
			//ProvisioningDHCPExternal is true and ProvisioningNetwork missing
			name:          "ImpliedUnmanaged",
			spec:          unmanagedProvisioning().ProvisioningNetwork("").ProvisioningDHCPExternal(true).build(),
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkUnmanaged,
		},
		{
			// ProvisioningInterface is missing
			name:          "InvalidUnmanaged",
			spec:          unmanagedProvisioning().ProvisioningInterface("").build(),
			expectedError: true,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkUnmanaged,
			expectedMsg:   "ProvisioningInterface",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := ValidateBaremetalProvisioningConfig(baremetalCR)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, getProvisioningNetworkMode(baremetalCR), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
			}
			return
		})
	}
}

func TestValidateDisabledProvisioningConfig(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testBaremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name          string
		spec          *metal3iov1alpha1.ProvisioningSpec
		expectedError bool
		expectedMode  metal3iov1alpha1.ProvisioningNetwork
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name:          "ValidDisabled",
			spec:          disabledProvisioning().build(),
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkDisabled,
		},
		{
			// Missing ProvisioningOSDownloadURL
			name:          "InvalidDisabled",
			spec:          disabledProvisioning().ProvisioningOSDownloadURL("").build(),
			expectedError: true,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkDisabled,
			expectedMsg:   "ProvisioningOSDownloadURL",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := ValidateBaremetalProvisioningConfig(baremetalCR)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, getProvisioningNetworkMode(baremetalCR), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
			}
			return
		})
	}
}

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
			expectedValue: "http://172.30.20.3:6180/images/ironic-python-agent.kernel",
		},
		{
			name:          "Disabled DeployKernelUrl",
			configName:    deployKernelUrl,
			spec:          disabledProvisioning().build(),
			expectedValue: "http://172.30.20.3:6180/images/ironic-python-agent.kernel",
		},
		{
			name:          "Unmanaged DeployRamdiskUrl",
			configName:    deployRamdiskUrl,
			spec:          unmanagedProvisioning().build(),
			expectedValue: "http://172.30.20.3:6180/images/ironic-python-agent.initramfs",
		},
		{
			name:          "Disabled DeployRamdiskUrl",
			configName:    deployRamdiskUrl,
			spec:          disabledProvisioning().build(),
			expectedValue: "http://172.30.20.3:6180/images/ironic-python-agent.initramfs",
		},
		{
			name:          "Disabled IronicEndpoint",
			configName:    ironicEndpoint,
			spec:          disabledProvisioning().build(),
			expectedValue: "http://localhost:6385/v1/",
		},
		{
			name:          "Disabled InspectorEndpoint",
			configName:    ironicInspectorEndpoint,
			spec:          disabledProvisioning().build(),
			expectedValue: "http://localhost:5050/v1/",
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
			expectedValue: "172.30.20.11, 172.30.20.101",
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
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Managed",
		},
	}
}

func unmanagedProvisioning() *provisioningBuilder {
	return &provisioningBuilder{
		metal3iov1alpha1.ProvisioningSpec{
			ProvisioningInterface:     "ensp0",
			ProvisioningIP:            "172.30.20.3",
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
			ProvisioningIP:            "172.30.20.3",
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Disabled",
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

func (pb *provisioningBuilder) ProvisioningNetwork(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningNetwork = metal3iov1alpha1.ProvisioningNetwork(value)
	return pb
}

func (pb *provisioningBuilder) ProvisioningOSDownloadURL(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningOSDownloadURL = value
	return pb
}

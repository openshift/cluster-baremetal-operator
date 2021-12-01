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

package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testBaremetalProvisioningCR = "test-provisioning-configuration"

func TestValidateManagedProvisioningConfig(t *testing.T) {
	baremetalCR := &Provisioning{
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
		spec          *ProvisioningSpec
		expectedError bool
		expectedMode  ProvisioningNetwork
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name:          "ValidManaged",
			spec:          managedProvisioning().build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkManaged,
		},
		{
			// ProvisioningNetwork is not specified and ProvisioningDHCPExternal is the default value
			name:          "ImpliedManaged",
			spec:          managedProvisioning().ProvisioningNetwork("").build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkManaged,
		},
		{
			// Provisioning IP is in the DHCP Range
			name:          "InvalidManagedProvisioningIPInDHCPRange",
			spec:          managedProvisioning().ProvisioningIP("172.30.20.20").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "value must be outside of the provisioningDHCPRange",
		},
		{
			// OSDownloadURL Image must end in qcow2.gz or qcow2.xz
			name:          "InvalidManagedDownloadURLSuffix",
			spec:          managedProvisioning().ProvisioningOSDownloadURL("http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.zip?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "OS image and must end in",
		},
		{
			// ProvisioningIP is not in the NetworkCIDR
			name:          "InvalidManagedProvisioningIPCIDR",
			spec:          managedProvisioning().ProvisioningIP("172.30.30.3").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "is not in the range defined by the provisioningNetworkCIDR",
		},
		{
			// ProvisioningIP missing
			name:          "MissingProvisioningIP",
			spec:          managedProvisioning().ProvisioningIP("").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "could not parse provisioningIP",
		},
		{
			// DHCPRange is invalid
			name:          "InvalidManagedDHCPRangeIPIncorrect",
			spec:          managedProvisioning().ProvisioningDHCPRange("172.30.20.11, 172.30.20.xxx").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "could not parse provisioningDHCPRange",
		},
		{
			// DHCPRange is not properly formatted
			name:          "InvalidManagedIncorrectDHCPRange",
			spec:          managedProvisioning().ProvisioningDHCPRange("172.30.20.11:172.30.30.100").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "not a valid provisioningDHCPRange",
		},
		{
			// OS URL has invalid checksum
			name:          "InvalidManagedNoChecksumURL",
			spec:          managedProvisioning().ProvisioningOSDownloadURL("http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=sputnik").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "the sha256 parameter in the provisioningOSDownloadURL",
		},
		{
			// DHCPRange is not part of the network CIDR
			name:          "InvalidManagedDHCPRangeOutsideCIDR",
			spec:          managedProvisioning().ProvisioningDHCPRange("172.30.30.11, 172.30.30.100").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "is not part of the provisioningNetworkCIDR",
		},
		{
			// DHCP Range is not set
			name:          "InvalidManagedDHCPRangeNotSet",
			spec:          managedProvisioning().ProvisioningDHCPRange("").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "provisioningDHCPRange is required in Managed mode",
		},
		{
			// OS URL is not http/https
			name:          "InvalidManagedURLNotHttp",
			spec:          managedProvisioning().ProvisioningOSDownloadURL("gopher://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkManaged,
			expectedMsg:   "unsupported scheme",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := baremetalCR.ValidateBaremetalProvisioningConfig(EnabledFeatures{
				ProvisioningNetwork: map[ProvisioningNetwork]bool{
					ProvisioningNetworkManaged: true,
				},
			})
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected errors: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, baremetalCR.getProvisioningNetworkMode(), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
				if !strings.Contains(err.Error(), tc.expectedMsg) {
					t.Errorf("unexpected errors: %v", err)
				}
			}
			return
		})
	}
}

func TestValidateUnmanagedProvisioningConfig(t *testing.T) {
	baremetalCR := &Provisioning{
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
		spec          *ProvisioningSpec
		expectedError bool
		expectedMode  ProvisioningNetwork
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name:          "ValidUnmanaged",
			spec:          unmanagedProvisioning().build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkUnmanaged,
		},
		{
			//ProvisioningDHCPExternal is true and ProvisioningNetwork missing
			name:          "ImpliedUnmanaged",
			spec:          unmanagedProvisioning().ProvisioningNetwork("").ProvisioningDHCPExternal(true).build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkUnmanaged,
		},
		{
			//ProvisioningDHCPRange is set and should be ignored
			name:          "ValidUnmanagedIgnoreDHCPRange",
			spec:          unmanagedProvisioning().ProvisioningDHCPRange("172.30.10.11,172.30.10.30").ProvisioningDHCPExternal(true).build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkUnmanaged,
		},
		{
			// Invalid provisioning IP.
			name:          "InvalidUnmanagedBadIP",
			spec:          unmanagedProvisioning().ProvisioningIP("172.30.20.500").ProvisioningDHCPExternal(true).build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkUnmanaged,
			expectedMsg:   "provisioningIP",
		},
		{
			// Missing provisioning IP.
			name:          "MissingProvisioningIP",
			spec:          unmanagedProvisioning().ProvisioningIP("").ProvisioningDHCPExternal(true).build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkUnmanaged,
			expectedMsg:   "provisioningIP",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := baremetalCR.ValidateBaremetalProvisioningConfig(EnabledFeatures{
				ProvisioningNetwork: map[ProvisioningNetwork]bool{
					ProvisioningNetworkUnmanaged: true,
				},
			})
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected errors: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, baremetalCR.getProvisioningNetworkMode(), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
			}
			return
		})
	}
}

func TestValidateDisabledProvisioningConfig(t *testing.T) {
	baremetalCR := &Provisioning{
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
		spec          *ProvisioningSpec
		expectedError bool
		expectedMode  ProvisioningNetwork
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name:          "ValidDisabled",
			spec:          disabledProvisioning().build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkDisabled,
		},
		{
			// All fields are specified, except ProvisioningIP and CIDR
			name:          "ValidDisabledNoNetwork",
			spec:          disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkDisabled,
		},
		{
			name:          "InvalidDisabledBadDownloadURL",
			spec:          disabledProvisioning().ProvisioningOSDownloadURL("http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.zip?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkDisabled,
			expectedMsg:   "provisioningOSDownloadURL",
		},
		{
			// Missing ProvisioningOSDownloadURL
			name:          "InvalidDisabled",
			spec:          disabledProvisioning().ProvisioningOSDownloadURL("").build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkDisabled,
		},
		{
			// IP and CIDR set with bad CIDR
			name:          "InvalidDisabledBadCIDR",
			spec:          disabledProvisioning().ProvisioningIP("172.22.0.3").ProvisioningNetworkCIDR("172.22.0.0/33").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkDisabled,
			expectedMsg:   "could not parse provisioningNetworkCIDR",
		},
		{
			// Only IP is set and not CIDR
			name:          "InvalidDisabledOnlyIP",
			spec:          disabledProvisioning().ProvisioningIP("172.22.0.3").ProvisioningNetworkCIDR("").build(),
			expectedError: true,
			expectedMode:  ProvisioningNetworkDisabled,
			expectedMsg:   "provisioningNetworkCIDR",
		},
		{
			// No Provisioning IP or Network
			name:          "NoProvisioningIPNetwork",
			spec:          disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedError: false,
			expectedMode:  ProvisioningNetworkDisabled,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := baremetalCR.ValidateBaremetalProvisioningConfig(EnabledFeatures{
				ProvisioningNetwork: map[ProvisioningNetwork]bool{
					ProvisioningNetworkDisabled: true,
				},
			})
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected errors: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, baremetalCR.getProvisioningNetworkMode(), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
				if !strings.Contains(err.Error(), tc.expectedMsg) {
					t.Errorf("Non-matching errors: %v", err)
				}
			}
			return
		})
	}
}

func TestValidateSupportedFeatures(t *testing.T) {
	baremetalCR := &Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testBaremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name            string
		spec            *ProvisioningSpec
		enabledfeatures EnabledFeatures
		expectedError   bool
		expectedMode    ProvisioningNetwork
		expectedMsg     string
	}{
		{
			name:            "managed feature enabled",
			enabledfeatures: EnabledFeatures{ProvisioningNetwork: map[ProvisioningNetwork]bool{ProvisioningNetworkManaged: true}},
			spec:            managedProvisioning().build(),
			expectedError:   false,
			expectedMode:    ProvisioningNetworkManaged,
		},
		{
			name:            "managed feature disabled",
			enabledfeatures: EnabledFeatures{ProvisioningNetwork: map[ProvisioningNetwork]bool{}},
			spec:            managedProvisioning().build(),
			expectedError:   true,
			expectedMode:    ProvisioningNetworkManaged,
		},
		{
			name:            "unmanaged feature enabled",
			enabledfeatures: EnabledFeatures{ProvisioningNetwork: map[ProvisioningNetwork]bool{ProvisioningNetworkUnmanaged: true}},
			spec:            unmanagedProvisioning().build(),
			expectedError:   false,
			expectedMode:    ProvisioningNetworkUnmanaged,
		},
		{
			name:            "managed feature disabled",
			enabledfeatures: EnabledFeatures{ProvisioningNetwork: map[ProvisioningNetwork]bool{}},
			spec:            unmanagedProvisioning().build(),
			expectedError:   true,
			expectedMode:    ProvisioningNetworkUnmanaged,
		},
		{
			name:            "disabled feature enabled",
			spec:            disabledProvisioning().build(),
			enabledfeatures: EnabledFeatures{ProvisioningNetwork: map[ProvisioningNetwork]bool{ProvisioningNetworkDisabled: true}},
			expectedError:   false,
			expectedMode:    ProvisioningNetworkDisabled,
		},
		{
			name:            "disabled feature disabled",
			spec:            disabledProvisioning().build(),
			enabledfeatures: EnabledFeatures{ProvisioningNetwork: map[ProvisioningNetwork]bool{}},
			expectedError:   true,
			expectedMode:    ProvisioningNetworkDisabled,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = *tc.spec
			err := baremetalCR.ValidateBaremetalProvisioningConfig(tc.enabledfeatures)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected errors: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, baremetalCR.getProvisioningNetworkMode(), "enabled results did not match")
			if tc.expectedError {
				assert.True(t, strings.Contains(err.Error(), tc.expectedMsg))
				if !strings.Contains(err.Error(), tc.expectedMsg) {
					t.Errorf("Non-matching errors: %v", err)
				}
			}
			return
		})
	}
}

type provisioningBuilder struct {
	ProvisioningSpec
}

func managedProvisioning() *provisioningBuilder {
	return &provisioningBuilder{
		ProvisioningSpec{
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
		ProvisioningSpec{
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
		ProvisioningSpec{
			ProvisioningInterface:     "",
			ProvisioningIP:            "172.30.20.3",
			ProvisioningNetworkCIDR:   "172.30.20.0/24",
			ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			ProvisioningNetwork:       "Disabled",
		},
	}
}

func (pb *provisioningBuilder) build() *ProvisioningSpec {
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
	pb.ProvisioningSpec.ProvisioningNetwork = ProvisioningNetwork(value)
	return pb
}

func (pb *provisioningBuilder) ProvisioningOSDownloadURL(value string) *provisioningBuilder {
	pb.ProvisioningSpec.ProvisioningOSDownloadURL = value
	return pb
}

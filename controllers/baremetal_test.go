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

package controllers

import (
	"strings"
	"testing"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateManagedProvisioningConfig(t *testing.T) {
	baremetalCR := &metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: baremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name          string
		spec          metal3iov1alpha1.ProvisioningSpec
		expectedError bool
		expectedMode  string
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "ValidManaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "ensp0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningNetwork:       "Managed",
			},
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
		},
		{
			// ProvisioningNetwork is not specified but ProvisioningDHCPExternal is.
			name: "ImpliedManaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "eth0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningDHCPExternal:  false,
			},
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
		},
		{
			// Verifying default behavior where both ProvisioningNetwork and ProvisioningDHCPExternal are not specified.
			name: "DefaultManaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "eth0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			},
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
		},
		{
			// ProvisioningInterface is not specified.
			name: "InvalidManaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningNetwork:       "Managed",
			},
			expectedError: true,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkManaged,
			expectedMsg:   "ProvisioningInterface",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = tc.spec
			err := validateBaremetalProvisioningConfig(baremetalCR)
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
			Name: baremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name          string
		spec          metal3iov1alpha1.ProvisioningSpec
		expectedError bool
		expectedMode  string
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "ValidUnmanaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "ensp0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningNetwork:       "Unmanaged",
			},
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkUnmanaged,
		},
		{
			//ProvisioningDHCPExternal is true and ProvisioningNetwork missing
			name: "ImpliedUnmanaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "ensp0",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningDHCPExternal:  true,
			},
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkUnmanaged,
		},
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "InvalidUnmanaged",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningNetwork:       "Unmanaged",
			},
			expectedError: true,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkUnmanaged,
			expectedMsg:   "ProvisioningInterface",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = tc.spec
			err := validateBaremetalProvisioningConfig(baremetalCR)
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
			Name: baremetalProvisioningCR,
		},
	}

	tCases := []struct {
		name          string
		spec          metal3iov1alpha1.ProvisioningSpec
		expectedError bool
		expectedMode  string
		expectedMsg   string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "ValidDisabled",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				ProvisioningNetwork:       "Disabled",
			},
			expectedError: false,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkDisabled,
		},
		{
			// Missing ProvisioningOSDownloadURL
			name: "InvalidDisabled",
			spec: metal3iov1alpha1.ProvisioningSpec{
				ProvisioningInterface:     "",
				ProvisioningIP:            "172.30.20.3",
				ProvisioningNetworkCIDR:   "172.30.20.0/24",
				ProvisioningOSDownloadURL: "",
				ProvisioningNetwork:       "Disabled",
			},
			expectedError: true,
			expectedMode:  metal3iov1alpha1.ProvisioningNetworkDisabled,
			expectedMsg:   "ProvisioningOSDownloadURL",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			baremetalCR.Spec = tc.spec
			err := validateBaremetalProvisioningConfig(baremetalCR)
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

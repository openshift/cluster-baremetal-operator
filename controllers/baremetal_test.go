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
	"testing"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateManagedProvisioningConfig(t *testing.T) {
	tCases := []struct {
		name          string
		baremetalCR   *metal3iov1alpha1.Provisioning
		expectedError bool
		expectedMode  string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "ValidManaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "ensp0",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningNetwork:       "Managed",
				},
			},
			expectedError: false,
			expectedMode:  "Managed",
		},
		{
			// ProvisioningNetwork is not specified but ProvisioningDHCPExternal is.
			name: "ImpliedManaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "eth0",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningDHCPExternal:  false,
				},
			},
			expectedError: false,
			expectedMode:  "Managed",
		},
		{
			// Verifying default behavior where both ProvisioningNetwork and ProvisioningDHCPExternal are not specified.
			name: "DefaultManaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "eth0",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
				},
			},
			expectedError: false,
			expectedMode:  "Managed",
		},
		{
			// ProvisioningInterface is not specified.
			name: "InvalidManaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningDHCPRange:     "172.30.20.11, 172.30.20.101",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningNetwork:       "Managed",
				},
			},
			expectedError: true,
			expectedMode:  "Managed",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			err := validateBaremetalProvisioningConfig(tc.baremetalCR)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, tc.baremetalCR.Spec.ProvisioningNetwork, "enabled results did not match")
			return
		})
	}
}

func TestValidateUnmanagedProvisioningConfig(t *testing.T) {
	tCases := []struct {
		name          string
		baremetalCR   *metal3iov1alpha1.Provisioning
		expectedError bool
		expectedMode  string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "ValidUnmanaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "ensp0",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningNetwork:       "Unmanaged",
				},
			},
			expectedError: false,
			expectedMode:  "Unmanaged",
		},
		{
			//ProvisioningDHCPExternal is true and ProvisioningNetwork missing
			name: "ImpliedUnmanaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "ensp0",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningDHCPExternal:  true,
				},
			},
			expectedError: false,
			expectedMode:  "Unmanaged",
		},
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "InvalidUnmanaged",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningNetwork:       "Unmanaged",
				},
			},
			expectedError: true,
			expectedMode:  "Unmanaged",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			err := validateBaremetalProvisioningConfig(tc.baremetalCR)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, tc.baremetalCR.Spec.ProvisioningNetwork, "enabled results did not match")
			return
		})
	}
}

func TestValidateDisabledProvisioningConfig(t *testing.T) {
	tCases := []struct {
		name          string
		baremetalCR   *metal3iov1alpha1.Provisioning
		expectedError bool
		expectedMode  string
	}{
		{
			// All fields are specified as they should including the ProvisioningNetwork
			name: "ValidDisabled",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningOSDownloadURL: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
					ProvisioningNetwork:       "Disabled",
				},
			},
			expectedError: false,
			expectedMode:  "Disabled",
		},
		{
			// Missing ProvisioningOSDownloadURL
			name: "InvalidDisabled",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: baremetalProvisioningCR,
				},
				Spec: metal3iov1alpha1.ProvisioningSpec{
					ProvisioningInterface:     "",
					ProvisioningIP:            "172.30.20.3",
					ProvisioningNetworkCIDR:   "172.30.20.0/24",
					ProvisioningOSDownloadURL: "",
					ProvisioningNetwork:       "Disabled",
				},
			},
			expectedError: true,
			expectedMode:  "Disabled",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			err := validateBaremetalProvisioningConfig(tc.baremetalCR)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedMode, tc.baremetalCR.Spec.ProvisioningNetwork, "enabled results did not match")
			return
		})
	}
}

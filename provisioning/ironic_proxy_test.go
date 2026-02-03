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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestUseIronicProxy(t *testing.T) {
	tCases := []struct {
		name     string
		info     *ProvisioningInfo
		expected bool
	}{
		{
			name: "ProvisioningNetwork Disabled",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				IsHyperShift: false,
			},
			expected: true,
		},
		{
			name: "VirtualMediaViaExternalNetwork enabled",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
				},
				IsHyperShift: false,
			},
			expected: true,
		},
		{
			name: "Managed provisioning without VirtualMediaViaExternalNetwork",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *managedProvisioning().build(),
				},
				IsHyperShift: false,
			},
			expected: false,
		},
		{
			name: "HyperShift mode - ironic proxy disabled even with disabled network",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				IsHyperShift: true,
			},
			expected: false,
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := UseIronicProxy(tc.info)
			assert.Equal(t, tc.expected, actual, "UseIronicProxy result mismatch")
		})
	}
}

func TestNewIronicProxyService(t *testing.T) {
	info := &ProvisioningInfo{
		ProvConfig: &metal3iov1alpha1.Provisioning{
			Spec: *disabledProvisioning().build(),
		},
		Namespace: "openshift-machine-api",
	}

	svc := newIronicProxyService(info)

	// Verify metadata
	assert.Equal(t, ironicProxyService, svc.Name, "Service name mismatch")
	assert.Equal(t, "openshift-machine-api", svc.Namespace, "Namespace mismatch")
	assert.Equal(t, ironicProxyService, svc.Labels[cboLabelName], "Label mismatch")

	// Verify spec
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type, "Service type should be ClusterIP")
	assert.Equal(t, corev1.ClusterIPNone, svc.Spec.ClusterIP, "ClusterIP should be None (headless)")
	assert.Equal(t, ironicProxyService, svc.Spec.Selector[cboLabelName], "Selector mismatch")

	// Verify ports
	assert.Len(t, svc.Spec.Ports, 1, "Should have exactly one port")
	assert.Equal(t, "ironic-api", svc.Spec.Ports[0].Name, "Port name mismatch")
	assert.Equal(t, int32(baremetalIronicPort), svc.Spec.Ports[0].Port, "Port should be 6385")
}

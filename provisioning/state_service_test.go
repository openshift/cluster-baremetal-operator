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

func TestNewMetal3StateService(t *testing.T) {
	tCases := []struct {
		name                    string
		info                    *ProvisioningInfo
		expectedIronicPort      int32
		expectedIronicAPIPort   int32
		expectedIronicAPITarget int32
		hasIronicAPIPort        bool
	}{
		{
			name: "Managed provisioning - no ironic-proxy",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *managedProvisioning().build(),
				},
				Namespace:    "openshift-machine-api",
				IsHyperShift: false,
			},
			expectedIronicPort: int32(baremetalIronicPort), // 6385
			hasIronicAPIPort:   false,                      // No separate ironic-api port when ironic-proxy is disabled
		},
		{
			name: "Disabled provisioning - with ironic-proxy",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				Namespace:    "openshift-machine-api",
				IsHyperShift: false,
			},
			expectedIronicPort:      int32(ironicPrivatePort), // 6388
			hasIronicAPIPort:        true,
			expectedIronicAPIPort:   int32(baremetalIronicPort), // 6385
			expectedIronicAPITarget: int32(ironicPrivatePort),   // 6388 (routes to the private port)
		},
		{
			name: "VirtualMediaViaExternalNetwork enabled - with ironic-proxy",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
				},
				Namespace:    "openshift-machine-api",
				IsHyperShift: false,
			},
			expectedIronicPort:      int32(ironicPrivatePort), // 6388
			hasIronicAPIPort:        true,
			expectedIronicAPIPort:   int32(baremetalIronicPort), // 6385
			expectedIronicAPITarget: int32(ironicPrivatePort),   // 6388 (routes to the private port)
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newMetal3StateService(tc.info)

			// Verify service metadata
			assert.Equal(t, stateService, svc.Name, "Service name mismatch")
			assert.Equal(t, tc.info.Namespace, svc.Namespace, "Namespace mismatch")
			assert.Equal(t, stateService, svc.Labels[cboLabelName], "Label mismatch")

			// Verify selector
			assert.Equal(t, stateService, svc.Spec.Selector[cboLabelName], "Selector mismatch")

			// Find the ironic port
			var ironicPort *corev1.ServicePort
			var ironicAPIPort *corev1.ServicePort
			for i := range svc.Spec.Ports {
				switch svc.Spec.Ports[i].Name {
				case "ironic":
					ironicPort = &svc.Spec.Ports[i]
				case "ironic-api":
					ironicAPIPort = &svc.Spec.Ports[i]
				}
			}

			// Verify the main ironic port
			assert.NotNil(t, ironicPort, "ironic port should exist")
			assert.Equal(t, tc.expectedIronicPort, ironicPort.Port, "ironic port value mismatch")

			// Verify the ironic-api port when ironic-proxy is enabled
			if tc.hasIronicAPIPort {
				assert.NotNil(t, ironicAPIPort, "ironic-api port should exist when ironic-proxy is enabled")
				assert.Equal(t, tc.expectedIronicAPIPort, ironicAPIPort.Port, "ironic-api port value mismatch")
				assert.Equal(t, tc.expectedIronicAPITarget, ironicAPIPort.TargetPort.IntVal, "ironic-api targetPort should route to private port (6388)")
			} else {
				assert.Nil(t, ironicAPIPort, "ironic-api port should not exist when ironic-proxy is disabled")
			}
		})
	}
}

func TestMetal3StateServiceSelector(t *testing.T) {
	// Verify that the metal3-state service selects metal3-state pods, not ironic-proxy pods
	info := &ProvisioningInfo{
		ProvConfig: &metal3iov1alpha1.Provisioning{
			Spec: *disabledProvisioning().build(),
		},
		Namespace: "openshift-machine-api",
	}

	svc := newMetal3StateService(info)

	// The selector should be for metal3-state (stateService), NOT ironicProxyService
	assert.Equal(t, stateService, svc.Spec.Selector[cboLabelName],
		"metal3-state service should select metal3-state pods, not ironic-proxy pods")
	assert.NotEqual(t, ironicProxyService, svc.Spec.Selector[cboLabelName],
		"metal3-state service should NOT select ironic-proxy pods")
}

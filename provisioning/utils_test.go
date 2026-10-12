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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"

	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestGetIronicIPs(t *testing.T) {
	metal3Pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "openshift-machine-api",
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: stateService,
			},
		},
		Status: corev1.PodStatus{
			PodIPs: []corev1.PodIP{
				{IP: "192.168.111.22"},
			},
		},
	}

	tCases := []struct {
		name        string
		info        *ProvisioningInfo
		expectedIPs []string
		expectError bool
	}{
		{
			name: "ExternalIPs set, returns externalIPs directly",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().ExternalIPs([]string{"1.2.3.4"}).build(),
				},
				Namespace: "openshift-machine-api",
				Client:    fakekube.NewSimpleClientset(metal3Pod),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type:    osconfigv1.VSpherePlatformType,
								VSphere: &osconfigv1.VSpherePlatformStatus{APIServerInternalIPs: []string{}},
							},
						},
					}),
			},
			expectedIPs: []string{"1.2.3.4"},
		},
		{
			name: "VSphere with empty apiServerInternalIPs falls back to pod IPs",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				Namespace: "openshift-machine-api",
				Client:    fakekube.NewSimpleClientset(metal3Pod),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Infrastructure",
							APIVersion: "config.openshift.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type:    osconfigv1.VSpherePlatformType,
								VSphere: &osconfigv1.VSpherePlatformStatus{APIServerInternalIPs: []string{}},
							},
						},
					}),
			},
			expectedIPs: []string{"192.168.111.22"},
		},
		{
			name: "VSphere with nil platformStatus falls back to pod IPs",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				Namespace: "openshift-machine-api",
				Client:    fakekube.NewSimpleClientset(metal3Pod),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Infrastructure",
							APIVersion: "config.openshift.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type:    osconfigv1.VSpherePlatformType,
								VSphere: nil,
							},
						},
					}),
			},
			expectedIPs: []string{"192.168.111.22"},
		},
		{
			name: "BareMetal with populated apiServerInternalIPs uses those IPs",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				Namespace: "openshift-machine-api",
				Client:    fakekube.NewSimpleClientset(metal3Pod),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Infrastructure",
							APIVersion: "config.openshift.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type: osconfigv1.BareMetalPlatformType,
								BareMetal: &osconfigv1.BareMetalPlatformStatus{
									APIServerInternalIPs: []string{"192.168.1.1"},
								},
							},
						},
					}),
			},
			expectedIPs: []string{"192.168.1.1"},
		},
		{
			name: "BareMetal with empty apiServerInternalIPs falls back to pod IPs",
			info: &ProvisioningInfo{
				ProvConfig: &metal3iov1alpha1.Provisioning{
					Spec: *disabledProvisioning().build(),
				},
				Namespace: "openshift-machine-api",
				Client:    fakekube.NewSimpleClientset(metal3Pod),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Infrastructure",
							APIVersion: "config.openshift.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type: osconfigv1.BareMetalPlatformType,
								BareMetal: &osconfigv1.BareMetalPlatformStatus{
									APIServerInternalIPs: []string{},
								},
							},
						},
					}),
			},
			expectedIPs: []string{"192.168.111.22"},
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := GetIronicIPs(tc.info)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedIPs, ips)
			}
		})
	}
}

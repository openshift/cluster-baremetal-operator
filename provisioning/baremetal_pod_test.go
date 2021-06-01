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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/openshift/api/config/v1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

var nodeMacAddresses = []string{"34:b3:2d:81:f8:fb", "34:b3:2d:81:f8:fc", "34:b3:2d:81:f8:fd"}

func TestBuildEnvVar(t *testing.T) {
	tCases := []struct {
		name           string
		configName     string
		spec           *metal3iov1alpha1.ProvisioningSpec
		expectedEnvVar corev1.EnvVar
	}{
		{
			name:       "Managed ProvisioningIPCIDR",
			configName: provisioningIP,
			spec:       managedProvisioning().build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  provisioningIP,
				Value: "172.30.20.3/24",
			},
		},
		{
			name:       "Unmanaged ProvisioningInterface",
			configName: provisioningInterface,
			spec:       unmanagedProvisioning().build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  provisioningInterface,
				Value: "ensp0",
			},
		},
		{
			name:       "Disabled MachineOsUrl",
			configName: machineImageUrl,
			spec:       disabledProvisioning().build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  machineImageUrl,
				Value: "http://172.22.0.1/images/rhcos-44.81.202001171431.0-openstack.x86_64.qcow2.gz?sha256=e98f83a2b9d4043719664a2be75fe8134dc6ca1fdbde807996622f8cc7ecd234",
			},
		},
		{
			name:       "Disabled ProvisioningInterface",
			configName: provisioningInterface,
			spec:       disabledProvisioning().build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  provisioningInterface,
				Value: "",
			},
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			actualEnvVar := buildEnvVar(tc.configName, tc.spec)
			assert.Equal(t, tc.expectedEnvVar, actualEnvVar, fmt.Sprintf("%s : Expected : %s Actual : %s", tc.configName, tc.expectedEnvVar, actualEnvVar))
			return
		})
	}
}

func TestNewMetal3InitContainers(t *testing.T) {
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		IronicInspector:     expectedIronicInspector,
		IpaDownloader:       expectedIronicIpaDownloader,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}
	tCases := []struct {
		name               string
		config             *metal3iov1alpha1.ProvisioningSpec
		expectedContainers []corev1.Container
	}{
		{
			name:   "valid config",
			config: managedProvisioning().build(),
			expectedContainers: []corev1.Container{
				{
					Name:  "metal3-ipa-downloader",
					Image: images.IpaDownloader,
				},
				{
					Name:  "metal3-machine-os-downloader",
					Image: images.MachineOsDownloader,
				},
				{
					Name:  "metal3-static-ip-set",
					Image: images.StaticIpManager,
				},
			},
		},
		{
			name:   "disabled without provisioning ip",
			config: disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedContainers: []corev1.Container{
				{
					Name:  "metal3-ipa-downloader",
					Image: images.IpaDownloader,
				},
				{
					Name:  "metal3-machine-os-downloader",
					Image: images.MachineOsDownloader,
				},
			},
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			info := &ProvisioningInfo{Images: &images, ProvConfig: &metal3iov1alpha1.Provisioning{Spec: *tc.config}}
			actualContainers := newMetal3InitContainers(info)
			assert.Equal(t, len(tc.expectedContainers), len(actualContainers), fmt.Sprintf("%s : Expected number of Init Containers : %d Actual number of Init Containers : %d", tc.name, len(tc.expectedContainers), len(actualContainers)))
		})
	}
}

func TestNewMetal3Containers(t *testing.T) {
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		IronicInspector:     expectedIronicInspector,
		IpaDownloader:       expectedIronicIpaDownloader,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}
	tCases := []struct {
		name               string
		config             *metal3iov1alpha1.ProvisioningSpec
		expectedContainers int
	}{
		{
			name:               "ManagedSpec",
			config:             managedProvisioning().build(),
			expectedContainers: 10,
		},
		{
			name:               "UnmanagedSpec",
			config:             unmanagedProvisioning().build(),
			expectedContainers: 10,
		},
		{
			name:               "DisabledSpec",
			config:             disabledProvisioning().build(),
			expectedContainers: 8,
		},
		{
			name:               "DisabledSpecWithoutProvisioningIP",
			config:             disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedContainers: 8,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			info := &ProvisioningInfo{
				Images:             &images,
				ProvConfig:         &metal3iov1alpha1.Provisioning{Spec: *tc.config},
				MasterMacAddresses: nodeMacAddresses,
			}
			actualContainers := newMetal3Containers(info)
			assert.Equal(t, tc.expectedContainers, len(actualContainers), fmt.Sprintf("%s : Expected number of Containers : %d Actual number of Containers : %d", tc.name, tc.expectedContainers, len(actualContainers)))
		})
	}
}

func TestProxyAndCAInjection(t *testing.T) {
	info := &ProvisioningInfo{
		Images: &Images{
			BaremetalOperator:   expectedBaremetalOperator,
			Ironic:              expectedIronic,
			IronicInspector:     expectedIronicInspector,
			IpaDownloader:       expectedIronicIpaDownloader,
			MachineOsDownloader: expectedMachineOsDownloader,
			StaticIpManager:     expectedIronicStaticIpManager,
		},
		ProvConfig: &metal3iov1alpha1.Provisioning{Spec: *managedProvisioning().build()},
		Proxy: &v1.Proxy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Status: v1.ProxyStatus{
				HTTPProxy:  "https://172.2.0.1:3128",
				HTTPSProxy: "https://172.2.0.1:3128",
				NoProxy:    ".example.com",
			},
		},
	}

	tCases := []struct {
		name       string
		containers []corev1.Container
	}{
		{
			name:       "init containers have proxy and CA information",
			containers: newMetal3InitContainers(info),
		},
		{
			name:       "metal3 containers have proxy and CA information",
			containers: newMetal3Containers(info),
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			for _, container := range tc.containers {
				assert.Contains(t, container.Env, corev1.EnvVar{Name: "HTTP_PROXY", Value: "https://172.2.0.1:3128"})
				assert.Contains(t, container.Env, corev1.EnvVar{Name: "HTTPS_PROXY", Value: "https://172.2.0.1:3128"})
				assert.Contains(t, container.Env, corev1.EnvVar{Name: "NO_PROXY", Value: ".example.com"})

				assert.Contains(t, container.VolumeMounts, corev1.VolumeMount{
					MountPath: "/etc/pki/ca-trust/extracted/pem",
					Name:      "trusted-ca",
					ReadOnly:  true},
				)
			}
		})
	}
}

func TestIPOptionForMachineOsDownloader(t *testing.T) {
	tests := []struct {
		ns   NetworkStackType
		want string
	}{
		{
			ns:   NetworkStackV4,
			want: "ip=dhcp",
		},
		{
			ns:   NetworkStackV6,
			want: "ip=dhcp6",
		},
		{
			ns:   NetworkStackDual,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := ipOptionForMachineOsDownloader(&ProvisioningInfo{NetworkStack: tt.ns}); got != tt.want {
				t.Errorf("ipOptionForMachineOsDownloader() = %v, want %v", got, tt.want)
			}
		})
	}
}

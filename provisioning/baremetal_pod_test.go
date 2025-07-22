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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"

	fakekube "k8s.io/client-go/kubernetes/fake"
)

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
			name:       "Unanaged ProvisioningIPCIDR",
			configName: provisioningIP,
			spec:       unmanagedProvisioning().build(),
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
		})
	}
}

func TestNewMetal3InitContainers(t *testing.T) {
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
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
					Name:  "metal3-static-ip-set",
					Image: images.StaticIpManager,
				},
				{
					Name:  "machine-os-images",
					Image: images.MachineOSImages,
				},
				{
					Name:  "metal3-machine-os-downloader",
					Image: images.MachineOsDownloader,
				},
			},
		},
		{
			name:   "disabled without provisioning ip",
			config: disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedContainers: []corev1.Container{
				{
					Name:  "machine-os-images",
					Image: images.MachineOSImages,
				},
				{
					Name:  "metal3-machine-os-downloader",
					Image: images.MachineOsDownloader,
				},
			},
		},
		{
			name:   "disabled with provisioning ip",
			config: disabledProvisioning().ProvisioningIP("1.2.3.4").ProvisioningNetworkCIDR("").build(),
			expectedContainers: []corev1.Container{
				{
					Name:  "machine-os-images",
					Image: images.MachineOSImages,
				},
				{
					Name:  "metal3-machine-os-downloader",
					Image: images.MachineOsDownloader,
				},
			},
		},
		{
			name:   "valid config with pre provisioning os download urls set",
			config: configWithPreProvisioningOSDownloadURLs().build(),
			expectedContainers: []corev1.Container{
				{
					Name:  "metal3-static-ip-set",
					Image: images.StaticIpManager,
				},
				{
					Name:  "machine-os-images",
					Image: images.MachineOSImages,
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
	envWithValue := func(name, value string) corev1.EnvVar {
		return corev1.EnvVar{Name: name, Value: value}
	}
	sshkey := envWithValue("IRONIC_RAMDISK_SSH_KEY", "sshkey")
	envWithFieldValue := func(name, fieldPath string) corev1.EnvVar {
		return corev1.EnvVar{
			Name:  name,
			Value: "",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: fieldPath,
				},
			},
		}
	}
	containers := map[string]corev1.Container{
		"metal3-httpd": {
			Name: "metal3-httpd",
			Env: []corev1.EnvVar{
				{Name: "HTTP_PORT", Value: "6180"},
				{Name: "PROVISIONING_IP", Value: "172.30.20.3/24"},
				{Name: "PROVISIONING_INTERFACE", Value: "eth0"},
				{Name: "IRONIC_RAMDISK_SSH_KEY"},
				{Name: "PROVISIONING_MACS", Value: "34:b3:2d:81:f8:fb,34:b3:2d:81:f8:fc,34:b3:2d:81:f8:fd"},
				{Name: "VMEDIA_TLS_PORT", Value: "6183"},
				{Name: "IRONIC_REVERSE_PROXY_SETUP", Value: "true"},
				{Name: "IRONIC_PRIVATE_PORT", Value: "unix"},
				{Name: "IRONIC_LISTEN_PORT", Value: "6385"},
			},
			VolumeMounts: []corev1.VolumeMount{
				sharedVolumeMount,
				ironicCredentialsMount,
				imageVolumeMount,
				ironicTlsMount,
			},
		},
		"metal3-ironic": {
			Name: "metal3-ironic",
			Env: []corev1.EnvVar{
				{Name: "IRONIC_INSECURE", Value: "true"},
				{Name: "IRONIC_KERNEL_PARAMS", Value: "rd.net.timeout.carrier=30 ip=dhcp"},
				{Name: "IRONIC_REVERSE_PROXY_SETUP", Value: "true"},
				{Name: "IRONIC_PRIVATE_PORT", Value: "unix"},
				{Name: "HTTP_PORT", Value: "6180"},
				{Name: "PROVISIONING_IP", Value: "172.30.20.3/24"},
				{Name: "PROVISIONING_INTERFACE", Value: "eth0"},
				{Name: "IRONIC_RAMDISK_SSH_KEY"},
				{Name: "IRONIC_EXTERNAL_IP"},
				{Name: "PROVISIONING_MACS", Value: "34:b3:2d:81:f8:fb,34:b3:2d:81:f8:fc,34:b3:2d:81:f8:fd"},
				{Name: "VMEDIA_TLS_PORT", Value: "6183"},
			},
			VolumeMounts: []corev1.VolumeMount{
				sharedVolumeMount,
				imageVolumeMount,
				ironicTlsMount,
			},
		},
		"metal3-ramdisk-logs": {
			Name:         "metal3-ramdisk-logs",
			Env:          []corev1.EnvVar{},
			VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		},
		"metal3-static-ip-manager": {
			Name: "metal3-static-ip-manager",
			Env: []corev1.EnvVar{
				{Name: "PROVISIONING_IP", Value: "172.30.20.3/24"},
				{Name: "PROVISIONING_INTERFACE", Value: "eth0"},
				{Name: "PROVISIONING_MACS", Value: "34:b3:2d:81:f8:fb,34:b3:2d:81:f8:fc,34:b3:2d:81:f8:fd"},
			},
		},
		"metal3-dnsmasq": {
			Name: "metal3-dnsmasq",
			Env: []corev1.EnvVar{
				{Name: "HTTP_PORT", Value: "6180"},
				{Name: "PROVISIONING_INTERFACE", Value: "eth0"},
				{Name: "DHCP_RANGE", Value: "172.30.20.11,172.30.20.101,24"},
				{Name: "PROVISIONING_MACS", Value: "34:b3:2d:81:f8:fb,34:b3:2d:81:f8:fc,34:b3:2d:81:f8:fd"},
			},
			VolumeMounts: []corev1.VolumeMount{
				sharedVolumeMount,
				imageVolumeMount,
			},
		},
	}
	withEnv := func(c corev1.Container, ne ...corev1.EnvVar) corev1.Container {
		newMap := map[string]corev1.EnvVar{}
		for _, n := range ne {
			newMap[n.Name] = n
		}

		new := []corev1.EnvVar{}
		for _, existing := range c.Env {
			override, haveOverride := newMap[existing.Name]
			if haveOverride {
				new = append(new, override)
				delete(newMap, existing.Name)
			} else {
				new = append(new, existing)
			}
		}
		// Make sure new variables also end up in the final list.
		// Append them to the end in the same order.
		for _, value := range ne {
			if _, exist := newMap[value.Name]; exist {
				new = append(new, value)
			}
		}
		c.Env = new
		return c
	}
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}
	// withTLSEnv wraps withEnv and appends TLS env vars at the end,
	// matching the order in which createContainerMetal3Httpd/Ironic append them.
	withTLSEnv := func(c corev1.Container, ne ...corev1.EnvVar) corev1.Container {
		c = withEnv(c, ne...)
		c.Env = append(c.Env,
			envWithValue("IRONIC_SSL_PROTOCOL", "-ALL +TLSv1.2 +TLSv1.3"),
			envWithValue("IRONIC_VMEDIA_SSL_PROTOCOL", "-ALL +TLSv1.2 +TLSv1.3"),
		)
		return c
	}
	tCases := []struct {
		name               string
		config             *metal3iov1alpha1.ProvisioningSpec
		sshkey             string
		expectedContainers []corev1.Container
	}{
		{
			name:   "ManagedSpec",
			config: managedProvisioning().build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with DNS",
			config: managedProvisioning().ProvisioningDNS(true).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				withEnv(
					containers["metal3-dnsmasq"],
					envWithValue("DNS_IP", "provisioning"),
				),
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with Gateway",
			config: managedProvisioning().ProvisioningNetworkGateway("192.0.2.1").build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				withEnv(
					containers["metal3-dnsmasq"],
					envWithValue("GATEWAY_IP", "192.0.2.1"),
				),
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with virtualmedia",
			config: managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(
					containers["metal3-httpd"],
					sshkey,
					envWithValue("IRONIC_LISTEN_PORT", "6388"),
				),
				withTLSEnv(containers["metal3-ironic"], sshkey, envWithFieldValue("IRONIC_EXTERNAL_IP", "status.hostIP")),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with sensor metrics enabled",
			config: managedProvisioning().PrometheusExporter(true, 60).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey,
					envWithValue("SEND_SENSOR_DATA", "true"),
					envWithValue("OS_SENSOR_DATA__INTERVAL", "60")),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
				{
					Name:         "metal3-ironic-prometheus-exporter",
					Env:          []corev1.EnvVar{},
					VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
				},
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with sensor metrics disabled",
			config: managedProvisioning().PrometheusExporter(false, 60).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with custom sensor metrics interval",
			config: managedProvisioning().PrometheusExporter(true, 120).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey,
					envWithValue("SEND_SENSOR_DATA", "true"),
					envWithValue("OS_SENSOR_DATA__INTERVAL", "120")),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
				{
					Name:         "metal3-ironic-prometheus-exporter",
					Env:          []corev1.EnvVar{},
					VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
				},
			},
			sshkey: "sshkey",
		},
		{
			name:   "UnmanagedSpec",
			config: unmanagedProvisioning().build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], envWithValue("PROVISIONING_INTERFACE", "ensp0"), envWithValue("PROVISIONING_IP", "172.30.20.3/24")),
				withTLSEnv(containers["metal3-ironic"], envWithValue("PROVISIONING_INTERFACE", "ensp0"), envWithValue("PROVISIONING_IP", "172.30.20.3/24")),
				containers["metal3-ramdisk-logs"],
				withEnv(containers["metal3-static-ip-manager"], envWithValue("PROVISIONING_INTERFACE", "ensp0"), envWithValue("PROVISIONING_IP", "172.30.20.3/24")),
				withEnv(containers["metal3-dnsmasq"], envWithValue("PROVISIONING_INTERFACE", "ensp0"), envWithValue("DHCP_RANGE", "")),
			},
			sshkey: "",
		},
		{
			name:   "DisabledSpec",
			config: disabledProvisioning().build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(
					containers["metal3-httpd"],
					envWithValue("PROVISIONING_INTERFACE", ""),
					envWithValue("IRONIC_LISTEN_PORT", "6388"),
					envWithValue("PROVISIONING_IP", "172.30.20.3"),
				),
				withTLSEnv(
					containers["metal3-ironic"],
					envWithValue("PROVISIONING_INTERFACE", ""),
					envWithValue("IRONIC_KERNEL_PARAMS", "rd.net.timeout.carrier=30 ip=dhcp6"),
					envWithValue("PROVISIONING_IP", "172.30.20.3"),
				),
				containers["metal3-ramdisk-logs"],
			},
			sshkey: "",
		},
		{
			name:   "DisabledSpecWithoutProvisioningIP",
			config: disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(
					containers["metal3-httpd"],
					envWithValue("PROVISIONING_INTERFACE", ""),
					envWithFieldValue("PROVISIONING_IP", "status.hostIP"),
					envWithValue("IRONIC_LISTEN_PORT", "6388"),
				),
				withTLSEnv(
					containers["metal3-ironic"],
					envWithValue("PROVISIONING_INTERFACE", ""),
					envWithFieldValue("PROVISIONING_IP", "status.hostIP"),
					envWithValue("IRONIC_KERNEL_PARAMS", "rd.net.timeout.carrier=30 ip=dhcp6"),
				),
				containers["metal3-ramdisk-logs"],
			},
			sshkey: "",
		},
		{
			name:   "DisabledSpecWithProvisioningInterface",
			config: disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").ProvisioningInterface("em1").build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(
					containers["metal3-httpd"],
					envWithValue("PROVISIONING_INTERFACE", "em1"),
					envWithValue("PROVISIONING_IP", ""),
					envWithValue("IRONIC_LISTEN_PORT", "6388"),
				),
				withTLSEnv(
					containers["metal3-ironic"],
					envWithValue("PROVISIONING_INTERFACE", "em1"),
					envWithValue("PROVISIONING_IP", ""),
					envWithValue("IRONIC_KERNEL_PARAMS", "rd.net.timeout.carrier=30 ip=dhcp6"),
				),
				containers["metal3-ramdisk-logs"],
			},
			sshkey: "",
		},
		{
			name:   "ManagedSpecWithIronicNetworking",
			config: managedProvisioning().SwitchManagementEnabled(true).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(containers["metal3-httpd"], sshkey),
				withTLSEnv(containers["metal3-ironic"], sshkey,
					envWithValue("IRONIC_NETWORKING_ENABLED", "true"),
					envWithValue("IRONIC_FORCE_DHCP", "true"),
					envWithValue("IRONIC_DEFAULT_NETWORK_INTERFACE", "ironic-networking"),
					envWithValue("IRONIC_NETWORKING_JSON_RPC_HOST", "metal3-ironic-networking-service.openshift-machine-api.svc.cluster.local"),
					envWithValue("IRONIC_NETWORKING_JSON_RPC_PORT", "6190"),
				),
				containers["metal3-ramdisk-logs"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
			},
			sshkey: "sshkey",
		},
		{
			name:   "DisabledSpecWithIronicNetworking",
			config: disabledProvisioning().SwitchManagementEnabled(true).build(),
			expectedContainers: []corev1.Container{
				withTLSEnv(
					containers["metal3-httpd"],
					envWithValue("PROVISIONING_INTERFACE", ""),
					envWithValue("IRONIC_LISTEN_PORT", "6388"),
					envWithValue("PROVISIONING_IP", "172.30.20.3"),
				),
				withTLSEnv(
					containers["metal3-ironic"],
					envWithValue("PROVISIONING_INTERFACE", ""),
					envWithValue("IRONIC_KERNEL_PARAMS", "rd.net.timeout.carrier=30 ip=dhcp6"),
					envWithValue("PROVISIONING_IP", "172.30.20.3"),
					envWithValue("IRONIC_NETWORKING_ENABLED", "true"),
					envWithValue("IRONIC_FORCE_DHCP", "true"),
					envWithValue("IRONIC_DEFAULT_NETWORK_INTERFACE", "ironic-networking"),
					envWithValue("IRONIC_NETWORKING_JSON_RPC_HOST", "metal3-ironic-networking-service.openshift-machine-api.svc.cluster.local"),
					envWithValue("IRONIC_NETWORKING_JSON_RPC_PORT", "6190"),
				),
				containers["metal3-ramdisk-logs"],
			},
			sshkey: "",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			info := &ProvisioningInfo{
				Images:         &images,
				Namespace:      "openshift-machine-api",
				ProvConfig:     &metal3iov1alpha1.Provisioning{Spec: *tc.config},
				SSHKey:         tc.sshkey,
				TLSProfileSpec: &osconfigv1.TLSProfileSpec{},
				NetworkStack:   NetworkStackV6,
				Client: fakekube.NewSimpleClientset(&corev1.Pod{
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
							{IP: "fd2e:6f44:5dd8:c956::16"},
						},
					}}),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Infrastructure",
							APIVersion: "config.openshift.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster",
						},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type: osconfigv1.BareMetalPlatformType,
								BareMetal: &osconfigv1.BareMetalPlatformStatus{
									APIServerInternalIPs: []string{
										"192.168.1.1",
										"fd2e:6f44:5dd8:c956::16",
									},
								},
							},
						},
					}),
			}
			actualContainers := newMetal3Containers(info)
			assert.Equal(t, len(tc.expectedContainers), len(actualContainers), fmt.Sprintf("%s : Expected number of Containers : %d Actual number of Containers : %d", tc.name, len(tc.expectedContainers), len(actualContainers)))
			for i, container := range actualContainers {
				assert.Equal(t, tc.expectedContainers[i].Name, actualContainers[i].Name)
				assert.Equal(t, len(tc.expectedContainers[i].Env), len(actualContainers[i].Env), "container name: ", tc.expectedContainers[i].Name)
				for e := range container.Env {
					assert.EqualValues(t, tc.expectedContainers[i].Env[e], actualContainers[i].Env[e], "container name: ", tc.expectedContainers[i].Name)
				}
			}
		})
	}
}

func TestNewMetal3ContainersNoTLSProfile(t *testing.T) {
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}
	info := &ProvisioningInfo{
		Images:       &images,
		ProvConfig:   &metal3iov1alpha1.Provisioning{Spec: *managedProvisioning().build()},
		SSHKey:       "sshkey",
		NetworkStack: NetworkStackV6,
		Client: fakekube.NewSimpleClientset(&corev1.Pod{
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
					{IP: "fd2e:6f44:5dd8:c956::16"},
				},
			}}),
		OSClient: fakeconfigclientset.NewSimpleClientset(
			&osconfigv1.Infrastructure{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Infrastructure",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: osconfigv1.InfrastructureStatus{
					PlatformStatus: &osconfigv1.PlatformStatus{
						Type: osconfigv1.BareMetalPlatformType,
						BareMetal: &osconfigv1.BareMetalPlatformStatus{
							APIServerInternalIPs: []string{
								"192.168.1.1",
								"fd2e:6f44:5dd8:c956::16",
							},
						},
					},
				},
			}),
	}

	containers := newMetal3Containers(info)

	tlsEnvNames := []string{
		"IRONIC_SSL_PROTOCOL",
		"IRONIC_VMEDIA_SSL_PROTOCOL",
		"IRONIC_TLS_12_CIPHERS",
		"IRONIC_TLS_13_CIPHERS",
	}
	for _, container := range containers {
		for _, envName := range tlsEnvNames {
			for _, env := range container.Env {
				assert.NotEqual(t, envName, env.Name,
					"container %s should not have TLS env var %s when TLSProfileSpec is nil",
					container.Name, envName)
			}
		}
	}
}

func TestProxyAndCAInjection(t *testing.T) {
	info := &ProvisioningInfo{
		Images: &Images{
			BaremetalOperator:   expectedBaremetalOperator,
			Ironic:              expectedIronic,
			MachineOsDownloader: expectedMachineOsDownloader,
			StaticIpManager:     expectedIronicStaticIpManager,
		},
		ProvConfig: &metal3iov1alpha1.Provisioning{Spec: *managedProvisioning().build()},
		Proxy: &osconfigv1.Proxy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Status: osconfigv1.ProxyStatus{
				HTTPProxy:  "https://172.2.0.1:3128",
				HTTPSProxy: "https://172.2.0.1:3128",
				NoProxy:    ".example.com",
			},
		},
		OSClient: fakeconfigclientset.NewSimpleClientset(
			&osconfigv1.Infrastructure{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Infrastructure",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: osconfigv1.InfrastructureStatus{
					PlatformStatus: &osconfigv1.PlatformStatus{
						Type: osconfigv1.BareMetalPlatformType,
						BareMetal: &osconfigv1.BareMetalPlatformStatus{
							APIServerInternalIPs: []string{
								"192.168.1.1",
								"fd2e:6f44:5dd8:c956::16",
							},
						},
					},
				},
			}),
	}

	containers := newMetal3Containers(info)
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
			containers: containers,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			for _, container := range tc.containers {
				assert.Contains(t, container.Env, corev1.EnvVar{Name: "HTTP_PROXY", Value: "https://172.2.0.1:3128"})
				assert.Contains(t, container.Env, corev1.EnvVar{Name: "HTTPS_PROXY", Value: "https://172.2.0.1:3128"})
				assert.Contains(t, container.Env, corev1.EnvVar{Name: "NO_PROXY", Value: ".example.com,"})

				assert.Contains(t, container.VolumeMounts, corev1.VolumeMount{
					MountPath: "/etc/pki/ca-trust/extracted/pem",
					Name:      "trusted-ca",
					ReadOnly:  true},
				)
			}
		})
	}
}

func TestSetIronicExternalIp(t *testing.T) {
	tCases := []struct {
		name           string
		envVarName     string
		spec           *metal3iov1alpha1.ProvisioningSpec
		expectedEnvVar corev1.EnvVar
	}{
		{
			name:       "ExternalIP is set",
			envVarName: externalIpEnvVar,
			spec:       managedProvisioning().ExternalIPs([]string{"192.168.1.100"}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalIpEnvVar,
				Value: "192.168.1.100",
			},
		},
		{
			name:       "ExternalIP is set with IPv6",
			envVarName: externalIpEnvVar,
			spec:       managedIPv6Provisioning().ExternalIPs([]string{"fd2e:6f44:5dd8:b856::100"}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalIpEnvVar,
				Value: "fd2e:6f44:5dd8:b856::100",
			},
		},
		{
			name:       "ExternalIP takes precedence over VirtualMediaViaExternalNetwork",
			envVarName: externalIpEnvVar,
			spec:       managedProvisioning().ExternalIPs([]string{"192.168.1.100"}).VirtualMediaViaExternalNetwork(true).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalIpEnvVar,
				Value: "192.168.1.100",
			},
		},
		{
			name:       "VirtualMediaViaExternalNetwork with Managed provisioning",
			envVarName: externalIpEnvVar,
			spec:       managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalIpEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
		},
		{
			name:       "VirtualMediaViaExternalNetwork with Unmanaged provisioning",
			envVarName: externalIpEnvVar,
			spec:       unmanagedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalIpEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
		},
		{
			name:       "VirtualMediaViaExternalNetwork false with Managed provisioning",
			envVarName: externalIpEnvVar,
			spec:       managedProvisioning().VirtualMediaViaExternalNetwork(false).build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalIpEnvVar,
			},
		},
		{
			name:       "Disabled provisioning network ignores VirtualMediaViaExternalNetwork",
			envVarName: externalIpEnvVar,
			spec:       disabledProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalIpEnvVar,
			},
		},
		{
			name:       "Default case with no ExternalIP and no VirtualMediaViaExternalNetwork",
			envVarName: externalIpEnvVar,
			spec:       managedProvisioning().build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalIpEnvVar,
			},
		},
		{
			name:       "Empty ExternalIP treated as not set",
			envVarName: externalIpEnvVar,
			spec:       managedProvisioning().ExternalIPs([]string{""}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalIpEnvVar,
			},
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			actualEnvVar := setIronicExternalIp(tc.envVarName, tc.spec)
			assert.Equal(t, tc.expectedEnvVar, actualEnvVar, fmt.Sprintf("%s : Expected : %v Actual : %v", tc.name, tc.expectedEnvVar, actualEnvVar))
		})
	}
}

func TestSetIronicExternalIPv6(t *testing.T) {
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}

	tCases := []struct {
		name           string
		spec           *metal3iov1alpha1.ProvisioningSpec
		expectedEnvVar corev1.EnvVar
		expectError    bool
	}{
		{
			name: "ExternalIPs set with IPv6 address and TLS enabled",
			spec: managedProvisioning().ExternalIPs([]string{"fd2e:6f44:5dd8:b856::100"}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalUrlEnvVar,
				Value: "https://[fd2e:6f44:5dd8:b856::100]:6183",
			},
			expectError: false,
		},
		{
			name: "ExternalIPs set with IPv6 address and TLS disabled",
			spec: managedProvisioning().ExternalIPs([]string{"fd2e:6f44:5dd8:b856::100"}).DisableVirtualMediaTLS(true).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalUrlEnvVar,
				Value: "http://[fd2e:6f44:5dd8:b856::100]:6180",
			},
			expectError: false,
		},
		{
			name: "ExternalIPs set with multiple IPs including IPv6",
			spec: managedProvisioning().ExternalIPs([]string{"192.168.1.100", "fd2e:6f44:5dd8:b856::100"}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalUrlEnvVar,
				Value: "https://[fd2e:6f44:5dd8:b856::100]:6183",
			},
			expectError: false,
		},
		{
			name: "ExternalIPs set with multiple IPv6 addresses",
			spec: managedProvisioning().ExternalIPs([]string{"fd2e:6f44:5dd8:b856::100", "fd2e:6f44:5dd8:b856::200"}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalUrlEnvVar,
				Value: "https://[fd2e:6f44:5dd8:b856::100]:6183",
			},
			expectError: false,
		},
		{
			name: "ExternalIPs set with only IPv4 address",
			spec: managedProvisioning().ExternalIPs([]string{"192.168.1.100"}).build(),
			expectedEnvVar: corev1.EnvVar{
				Name: externalUrlEnvVar,
			},
			expectError: false,
		},
		{
			name: "ExternalIPs not set - falls back to pod IPs",
			spec: disabledProvisioning().build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalUrlEnvVar,
				Value: "https://[fd2e:6f44:5dd8:c956::16]:6183",
			},
			expectError: false,
		},
		{
			name: "ExternalIPs not set with VirtualMediaViaExternalNetwork - falls back to pod IPs",
			spec: managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedEnvVar: corev1.EnvVar{
				Name:  externalUrlEnvVar,
				Value: "https://[fd2e:6f44:5dd8:c956::16]:6183",
			},
			expectError: false,
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			info := &ProvisioningInfo{
				Images:       &images,
				ProvConfig:   &metal3iov1alpha1.Provisioning{Spec: *tc.spec},
				SSHKey:       "testkey",
				NetworkStack: NetworkStackV6,
				Namespace:    "openshift-machine-api",
				Client: fakekube.NewSimpleClientset(&corev1.Pod{
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
							{IP: "fd2e:6f44:5dd8:c956::16"},
						},
					}}),
				OSClient: fakeconfigclientset.NewSimpleClientset(
					&osconfigv1.Infrastructure{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Infrastructure",
							APIVersion: "config.openshift.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "cluster",
						},
						Status: osconfigv1.InfrastructureStatus{
							PlatformStatus: &osconfigv1.PlatformStatus{
								Type: osconfigv1.BareMetalPlatformType,
								BareMetal: &osconfigv1.BareMetalPlatformStatus{
									APIServerInternalIPs: []string{
										"192.168.1.1",
										"fd2e:6f44:5dd8:c956::16",
									},
								},
							},
						},
					}),
			}

			actualEnvVar, err := setIronicExternalIPv6(info)

			if tc.expectError {
				assert.Error(t, err, fmt.Sprintf("%s : Expected error but got none", tc.name))
			} else {
				assert.NoError(t, err, fmt.Sprintf("%s : Unexpected error: %v", tc.name, err))
				assert.Equal(t, tc.expectedEnvVar, actualEnvVar, fmt.Sprintf("%s : Expected : %v Actual : %v", tc.name, tc.expectedEnvVar, actualEnvVar))
			}
		})
	}
}

func TestCreateInitContainerMachineOsDownloader(t *testing.T) {
	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}

	tCases := []struct {
		name                  string
		useLiveImages         bool
		expectedCommand       string
		expectedContainerName string
	}{
		{
			name:                  "get-resource script",
			useLiveImages:         false,
			expectedCommand:       "/usr/local/bin/get-resource.sh",
			expectedContainerName: "metal3-machine-os-downloader",
		},
		{
			name:                  "get-live-images script",
			useLiveImages:         true,
			expectedCommand:       "/usr/local/bin/get-live-images.sh",
			expectedContainerName: "metal3-machine-os-downloader-live-images",
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			info := &ProvisioningInfo{
				Images:       &images,
				ProvConfig:   &metal3iov1alpha1.Provisioning{Spec: *managedProvisioning().build()},
				NetworkStack: NetworkStackV4,
			}

			container := createInitContainerMachineOsDownloader(info, "http://example.com/image.qcow2", tc.useLiveImages, false)

			assert.Equal(t, tc.expectedContainerName, container.Name)
			assert.Equal(t, []string{tc.expectedCommand}, container.Command)

			// Verify that imageVolumeMount, sharedVolumeMount, and ironicTmpMount are present
			// This is critical because the scripts need to write to /shared/tmp
			// while having ReadOnlyRootFilesystem enabled. ironicTmpMount provides
			// a writable /tmp for libguestfs tools.
			assert.Contains(t, container.VolumeMounts, imageVolumeMount,
				"imageVolumeMount should be present for /shared/html/images")
			assert.Contains(t, container.VolumeMounts, sharedVolumeMount,
				"sharedVolumeMount should be present for /shared/tmp writes")
			assert.Contains(t, container.VolumeMounts, ironicTmpMount,
				"ironicTmpMount should be present for /tmp writes (required by libguestfs)")

			// Verify ReadOnlyRootFilesystem is enabled
			assert.NotNil(t, container.SecurityContext)
			assert.NotNil(t, container.SecurityContext.ReadOnlyRootFilesystem)
			assert.True(t, *container.SecurityContext.ReadOnlyRootFilesystem,
				"ReadOnlyRootFilesystem should be true")
		})
	}
}

func TestOperandImagesMatch(t *testing.T) {
	const namespace = "openshift-machine-api"

	desiredImages := &Images{
		Ironic:            "registry.example.com/ironic:v2",
		StaticIpManager:   "registry.example.com/static-ip:v2",
		BaremetalOperator: "registry.example.com/bmo:v2",
	}

	metal3Deployment := func(ironicImage, staticIPImage string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalDeploymentName,
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "metal3-ironic", Image: ironicImage},
							{Name: "metal3-static-ip-manager", Image: staticIPImage},
						},
					},
				},
			},
		}
	}

	bmoDeployment := func(bmoImage string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bmoDeploymentName,
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "metal3-baremetal-operator", Image: bmoImage},
						},
					},
				},
			},
		}
	}

	tCases := []struct {
		name     string
		objects  []*appsv1.Deployment
		expected bool
	}{
		{
			name:     "no deployments exist",
			objects:  nil,
			expected: false,
		},
		{
			name: "all images match",
			objects: []*appsv1.Deployment{
				metal3Deployment(desiredImages.Ironic, desiredImages.StaticIpManager),
				bmoDeployment(desiredImages.BaremetalOperator),
			},
			expected: true,
		},
		{
			name: "ironic image differs",
			objects: []*appsv1.Deployment{
				metal3Deployment("registry.example.com/ironic:v1", desiredImages.StaticIpManager),
				bmoDeployment(desiredImages.BaremetalOperator),
			},
			expected: false,
		},
		{
			name: "static-ip-manager image differs",
			objects: []*appsv1.Deployment{
				metal3Deployment(desiredImages.Ironic, "registry.example.com/static-ip:v1"),
				bmoDeployment(desiredImages.BaremetalOperator),
			},
			expected: false,
		},
		{
			name: "bmo image differs",
			objects: []*appsv1.Deployment{
				metal3Deployment(desiredImages.Ironic, desiredImages.StaticIpManager),
				bmoDeployment("registry.example.com/bmo:v1"),
			},
			expected: false,
		},
		{
			name: "only metal3 exists and matches",
			objects: []*appsv1.Deployment{
				metal3Deployment(desiredImages.Ironic, desiredImages.StaticIpManager),
			},
			expected: false,
		},
		{
			name: "only bmo exists and differs",
			objects: []*appsv1.Deployment{
				bmoDeployment("registry.example.com/bmo:v1"),
			},
			expected: false,
		},
		{
			name: "metal3 missing required container",
			objects: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      baremetalDeploymentName,
						Namespace: namespace,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "metal3-ironic", Image: desiredImages.Ironic},
								},
							},
						},
					},
				},
				bmoDeployment(desiredImages.BaremetalOperator),
			},
			expected: false,
		},
		{
			name: "bmo missing required container",
			objects: []*appsv1.Deployment{
				metal3Deployment(desiredImages.Ironic, desiredImages.StaticIpManager),
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bmoDeploymentName,
						Namespace: namespace,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "unrelated-container", Image: "some-image"},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			var kubeObjects []runtime.Object
			for _, d := range tc.objects {
				kubeObjects = append(kubeObjects, d)
			}
			kubeClient := fakekube.NewSimpleClientset(kubeObjects...)
			result, err := OperandImagesMatch(kubeClient.AppsV1(), namespace, desiredImages)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSwitchVolumesNotInStaticMetal3Volumes(t *testing.T) {
	// Switch volumes should NOT be in the static metal3Volumes slice;
	// they are conditionally added in newMetal3PodTemplateSpec.
	for _, volume := range metal3Volumes {
		assert.NotEqual(t, switchConfigsVolume, volume.Name, "Switch configs volume should not be in static metal3Volumes")
		assert.NotEqual(t, switchCredentialsVolume, volume.Name, "Switch credentials volume should not be in static metal3Volumes")
	}
}

func TestSwitchVolumesNotInMetal3Pod(t *testing.T) {
	images := Images{
		Ironic:              expectedIronic,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}

	// Switch volumes should not be in the metal3 pod template regardless
	// of networking flag, since the networking container now runs in its
	// own deployment.
	for _, networkingEnabled := range []bool{true, false} {
		info := &ProvisioningInfo{
			Images: &images,
			ProvConfig: &metal3iov1alpha1.Provisioning{
				Spec: *managedProvisioning().SwitchManagementEnabled(networkingEnabled).build(),
			},
			NetworkStack: NetworkStackV6,
			Client: fakekube.NewSimpleClientset(&corev1.Pod{
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
			}),
		}
		labels := map[string]string{"k8s-app": metal3AppName}
		template := newMetal3PodTemplateSpec(info, &labels)
		for _, volume := range template.Spec.Volumes {
			assert.NotEqual(t, switchConfigsVolume, volume.Name, "Switch configs volume should not be in metal3 pod (networking=%v)", networkingEnabled)
			assert.NotEqual(t, switchCredentialsVolume, volume.Name, "Switch credentials volume should not be in metal3 pod (networking=%v)", networkingEnabled)
		}
	}
}

func TestIronicNetworkingEnvVarInIronicContainer(t *testing.T) {
	images := Images{
		Ironic: expectedIronic,
	}

	// Test with SwitchManagement enabled (default network driver)
	info := &ProvisioningInfo{
		Images:    &images,
		Namespace: "openshift-machine-api",
		ProvConfig: &metal3iov1alpha1.Provisioning{
			Spec: metal3iov1alpha1.ProvisioningSpec{
				SwitchManagement: &metal3iov1alpha1.SwitchManagement{Enabled: true},
			},
		},
		NetworkStack: NetworkStackV4,
	}

	config := &info.ProvConfig.Spec
	sshKey := "test-ssh-key"

	container := createContainerMetal3Ironic(&images, info, config, sshKey)

	assertEnvVar(t, container.Env, ironicNetworkingEnabledEnvVar, "true")
	assertEnvVar(t, container.Env, "IRONIC_DEFAULT_NETWORK_INTERFACE", "ironic-networking")
	assertEnvVar(t, container.Env, "IRONIC_NETWORKING_JSON_RPC_HOST", "metal3-ironic-networking-service.openshift-machine-api.svc.cluster.local")
	assertEnvVar(t, container.Env, "IRONIC_NETWORKING_JSON_RPC_PORT", "6190")
	assert.Contains(t, container.VolumeMounts, ironicRPCCredentialsMount,
		"ironicRPCCredentialsMount should be present when SwitchManagement is enabled")

	// Test with SwitchManagement disabled
	info.ProvConfig.Spec.SwitchManagement = nil
	container = createContainerMetal3Ironic(&images, info, config, sshKey)

	assertNoEnvVar(t, container.Env, ironicNetworkingEnabledEnvVar)
	assertNoEnvVar(t, container.Env, "IRONIC_DEFAULT_NETWORK_INTERFACE")
	assertNoEnvVar(t, container.Env, "IRONIC_NETWORKING_JSON_RPC_HOST")
	assertNoEnvVar(t, container.Env, "IRONIC_NETWORKING_JSON_RPC_PORT")
	assert.NotContains(t, container.VolumeMounts, ironicRPCCredentialsMount,
		"ironicRPCCredentialsMount should be absent when SwitchManagement is disabled")
}

func TestBuildNetworkEnvVarValue(t *testing.T) {
	tCases := []struct {
		name     string
		cfg      *metal3iov1alpha1.ProviderNetworkConfig
		expected string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: "",
		},
		{
			name: "access mode",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:       metal3iov1alpha1.SwitchPortModeAccess,
				NativeVLAN: 100,
			},
			expected: "access/native_vlan=100",
		},
		{
			name: "trunk mode with allowed VLANs",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:         metal3iov1alpha1.SwitchPortModeTrunk,
				NativeVLAN:   100,
				AllowedVLANs: []string{"200", "300", "400"},
			},
			expected: "trunk/native_vlan=100/allowed_vlans=200,300,400",
		},
		{
			name: "hybrid mode with allowed VLANs",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:         metal3iov1alpha1.SwitchPortModeHybrid,
				NativeVLAN:   50,
				AllowedVLANs: []string{"10"},
			},
			expected: "hybrid/native_vlan=50/allowed_vlans=10",
		},
		{
			name: "trunk mode with empty allowed VLANs",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:         metal3iov1alpha1.SwitchPortModeTrunk,
				NativeVLAN:   100,
				AllowedVLANs: []string{},
			},
			expected: "trunk/native_vlan=100",
		},
		{
			name: "access mode ignores allowed VLANs",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:         "access",
				NativeVLAN:   100,
				AllowedVLANs: []string{"200", "300"},
			},
			expected: "access/native_vlan=100",
		},
		{
			name: "access mode with MTU",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:       metal3iov1alpha1.SwitchPortModeAccess,
				NativeVLAN: 100,
				MTU:        9000,
			},
			expected: "access/native_vlan=100/mtu=9000",
		},
		{
			name: "trunk mode with allowed VLANs and MTU",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:         metal3iov1alpha1.SwitchPortModeTrunk,
				NativeVLAN:   100,
				AllowedVLANs: []string{"200", "300"},
				MTU:          1500,
			},
			expected: "trunk/native_vlan=100/allowed_vlans=200,300/mtu=1500",
		},
		{
			name: "trunk mode with VLAN ranges",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:         metal3iov1alpha1.SwitchPortModeTrunk,
				NativeVLAN:   100,
				AllowedVLANs: []string{"200-210", "300", "400-500"},
			},
			expected: "trunk/native_vlan=100/allowed_vlans=200-210,300,400-500",
		},
		{
			name: "MTU zero is omitted",
			cfg: &metal3iov1alpha1.ProviderNetworkConfig{
				Mode:       metal3iov1alpha1.SwitchPortModeAccess,
				NativeVLAN: 100,
				MTU:        0,
			},
			expected: "access/native_vlan=100",
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildProviderNetworkEnvVarValue(tc.cfg)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSetupSwitchConfigsEnvVars(t *testing.T) {
	tCases := []struct {
		name           string
		networks       []metal3iov1alpha1.ProviderNetworkConfig
		existingEnvs   []corev1.EnvVar
		expectedEnvs   int
		expectedNames  []string
		expectedValues []string
	}{
		{
			name:         "nil config returns existing envs unchanged",
			networks:     nil,
			existingEnvs: []corev1.EnvVar{{Name: "EXISTING", Value: "val"}},
			expectedEnvs: 1,
		},
		{
			name: "derives env var name from Type field",
			networks: []metal3iov1alpha1.ProviderNetworkConfig{
				{
					Type:       "idle",
					Mode:       metal3iov1alpha1.SwitchPortModeAccess,
					NativeVLAN: 100,
				},
				{
					Type:       "inspection",
					Mode:       metal3iov1alpha1.SwitchPortModeAccess,
					NativeVLAN: 100,
				},
			},
			existingEnvs:   []corev1.EnvVar{},
			expectedEnvs:   2,
			expectedNames:  []string{"IRONIC_NETWORKING_IDLE_NETWORK", "IRONIC_NETWORKING_INSPECTION_NETWORK"},
			expectedValues: []string{"access/native_vlan=100", "access/native_vlan=100"},
		},
		{
			name: "each network gets its own config",
			networks: []metal3iov1alpha1.ProviderNetworkConfig{
				{
					Type:       "idle",
					Mode:       metal3iov1alpha1.SwitchPortModeAccess,
					NativeVLAN: 100,
				},
				{
					Type:       "cleaning",
					Mode:       metal3iov1alpha1.SwitchPortModeTrunk,
					NativeVLAN: 50,
				},
			},
			existingEnvs:   []corev1.EnvVar{{Name: "EXISTING", Value: "val"}},
			expectedEnvs:   3,
			expectedNames:  []string{"IRONIC_NETWORKING_IDLE_NETWORK", "IRONIC_NETWORKING_CLEANING_NETWORK"},
			expectedValues: []string{"access/native_vlan=100", "trunk/native_vlan=50"},
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			result := appendProviderNetworkConfigEnvVars(tc.networks, tc.existingEnvs)
			assert.Len(t, result, tc.expectedEnvs)
			for _, existing := range tc.existingEnvs {
				assert.Contains(t, result, existing)
			}
			for i, name := range tc.expectedNames {
				found := false
				for _, env := range result {
					if env.Name == name {
						found = true
						assert.Equal(t, tc.expectedValues[i], env.Value)
						break
					}
				}
				assert.True(t, found, "Expected env var %s not found", name)
			}
		})
	}
}

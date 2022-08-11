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
	envWithSecret := func(name, secret, key string) corev1.EnvVar {
		return corev1.EnvVar{
			Name: name,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret,
					},
					Key: key,
				},
			},
		}
	}
	containers := map[string]corev1.Container{
		"metal3-baremetal-operator": {
			Name: "metal3-baremetal-operator",
			Env: []corev1.EnvVar{
				envWithFieldValue("WATCH_NAMESPACE", "metadata.namespace"),
				envWithFieldValue("POD_NAMESPACE", "metadata.namespace"),
				envWithFieldValue("POD_NAME", "metadata.name"),
				{Name: "OPERATOR_NAME", Value: "baremetal-operator"},
				{Name: "IRONIC_CACERT_FILE", Value: "/certs/ironic/tls.crt"},
				{Name: "IRONIC_INSECURE", Value: "true"},
				{Name: "DEPLOY_KERNEL_URL", Value: "http://localhost:6180/images/ironic-python-agent.kernel"},
				{Name: "IRONIC_ENDPOINT", Value: "https://localhost:6385/v1/"},
				{Name: "IRONIC_INSPECTOR_ENDPOINT", Value: "https://localhost:5050/v1/"},
				{Name: "LIVE_ISO_FORCE_PERSISTENT_BOOT_DEVICE", Value: "Never"},
				{Name: "METAL3_AUTH_ROOT_DIR", Value: "/auth"},
			},
		},
		"metal3-httpd": {
			Name: "metal3-httpd",
			Env: []corev1.EnvVar{
				{Name: "HTTP_PORT", Value: "6180"},
				{Name: "PROVISIONING_IP", Value: "172.30.20.3/24"},
				{Name: "PROVISIONING_INTERFACE", Value: "eth0"},
				{Name: "IRONIC_RAMDISK_SSH_KEY"},
				{Name: "PROVISIONING_MACS", Value: "34:b3:2d:81:f8:fb,34:b3:2d:81:f8:fc,34:b3:2d:81:f8:fd"},
				{Name: "VMEDIA_TLS_PORT", Value: "6183"},
				envWithSecret("IRONIC_HTPASSWD", "metal3-ironic-password", "htpasswd"),
				envWithSecret("INSPECTOR_HTPASSWD", "metal3-ironic-inspector-password", "htpasswd"),
				{Name: "IRONIC_REVERSE_PROXY_SETUP", Value: "true"},
				{Name: "INSPECTOR_REVERSE_PROXY_SETUP", Value: "true"},
				{Name: "IRONIC_PRIVATE_PORT", Value: "unix"},
				{Name: "IRONIC_INSPECTOR_PRIVATE_PORT", Value: "unix"},
			},
		},
		"metal3-ironic": {
			Name: "metal3-ironic",
			Env: []corev1.EnvVar{
				{Name: "IRONIC_INSECURE", Value: "true"},
				{Name: "IRONIC_INSPECTOR_INSECURE", Value: "true"},
				{Name: "IRONIC_KERNEL_PARAMS", Value: "ip=dhcp"},
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
		},
		"metal3-ramdisk-logs": {
			Name: "metal3-ramdisk-logs",
			Env:  []corev1.EnvVar{},
		},
		"metal3-ironic-inspector": {
			Name: "metal3-ironic-inspector",
			Env: []corev1.EnvVar{
				{Name: "IRONIC_INSECURE", Value: "true"},
				{Name: "IRONIC_KERNEL_PARAMS", Value: "ip=dhcp"},
				{Name: "INSPECTOR_REVERSE_PROXY_SETUP", Value: "true"},
				{Name: "IRONIC_INSPECTOR_PRIVATE_PORT", Value: "unix"},
				{Name: "PROVISIONING_IP", Value: "172.30.20.3/24"},
				{Name: "PROVISIONING_INTERFACE", Value: "eth0"},
				{Name: "PROVISIONING_MACS", Value: "34:b3:2d:81:f8:fb,34:b3:2d:81:f8:fc,34:b3:2d:81:f8:fd"},
			},
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
			} else {
				new = append(new, existing)
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
				containers["metal3-baremetal-operator"],
				withEnv(containers["metal3-httpd"], sshkey),
				withEnv(containers["metal3-ironic"], sshkey),
				containers["metal3-ramdisk-logs"],
				containers["metal3-ironic-inspector"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with virtualmedia",
			config: managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedContainers: []corev1.Container{
				containers["metal3-baremetal-operator"],
				withEnv(containers["metal3-httpd"], sshkey),
				withEnv(containers["metal3-ironic"], sshkey, envWithFieldValue("IRONIC_EXTERNAL_IP", "status.hostIP")),
				containers["metal3-ramdisk-logs"],
				containers["metal3-ironic-inspector"],
				containers["metal3-static-ip-manager"],
				containers["metal3-dnsmasq"],
			},
			sshkey: "sshkey",
		},
		{
			name:   "UnmanagedSpec",
			config: unmanagedProvisioning().build(),
			expectedContainers: []corev1.Container{
				containers["metal3-baremetal-operator"],
				withEnv(containers["metal3-httpd"], envWithValue("PROVISIONING_INTERFACE", "ensp0")),
				withEnv(containers["metal3-ironic"], envWithValue("PROVISIONING_INTERFACE", "ensp0")),
				containers["metal3-ramdisk-logs"],
				withEnv(containers["metal3-ironic-inspector"], envWithValue("PROVISIONING_INTERFACE", "ensp0")),
				withEnv(containers["metal3-static-ip-manager"], envWithValue("PROVISIONING_INTERFACE", "ensp0")),
				withEnv(containers["metal3-dnsmasq"], envWithValue("PROVISIONING_INTERFACE", "ensp0"), envWithValue("DHCP_RANGE", "")),
			},
			sshkey: "",
		},
		{
			name:   "DisabledSpec",
			config: disabledProvisioning().build(),
			expectedContainers: []corev1.Container{
				containers["metal3-baremetal-operator"],
				withEnv(containers["metal3-httpd"], envWithValue("PROVISIONING_INTERFACE", "")),
				withEnv(containers["metal3-ironic"], envWithValue("PROVISIONING_INTERFACE", ""), envWithValue("IRONIC_KERNEL_PARAMS", "ip=dhcp6")),
				containers["metal3-ramdisk-logs"],
				withEnv(containers["metal3-ironic-inspector"], envWithValue("PROVISIONING_INTERFACE", ""), envWithValue("IRONIC_KERNEL_PARAMS", "ip=dhcp6")),
			},
			sshkey: "",
		},
		{
			name:   "DisabledSpecWithoutProvisioningIP",
			config: disabledProvisioning().ProvisioningIP("").ProvisioningNetworkCIDR("").build(),
			expectedContainers: []corev1.Container{
				containers["metal3-baremetal-operator"],
				withEnv(containers["metal3-httpd"], envWithValue("PROVISIONING_INTERFACE", ""), envWithFieldValue("PROVISIONING_IP", "status.hostIP")),
				withEnv(containers["metal3-ironic"], envWithValue("PROVISIONING_INTERFACE", ""), envWithFieldValue("PROVISIONING_IP", "status.hostIP"), envWithValue("IRONIC_KERNEL_PARAMS", "ip=dhcp6")),
				containers["metal3-ramdisk-logs"],
				withEnv(containers["metal3-ironic-inspector"], envWithValue("PROVISIONING_INTERFACE", ""), envWithFieldValue("PROVISIONING_IP", "status.hostIP"), envWithValue("IRONIC_KERNEL_PARAMS", "ip=dhcp6")),
			},
			sshkey: "",
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			info := &ProvisioningInfo{
				Images:       &images,
				ProvConfig:   &metal3iov1alpha1.Provisioning{Spec: *tc.config},
				SSHKey:       tc.sshkey,
				NetworkStack: NetworkStackV6,
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

func TestProxyAndCAInjection(t *testing.T) {
	info := &ProvisioningInfo{
		Images: &Images{
			BaremetalOperator:   expectedBaremetalOperator,
			Ironic:              expectedIronic,
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

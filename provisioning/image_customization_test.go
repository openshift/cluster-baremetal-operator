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

	v1 "github.com/openshift/api/config/v1"
)

func TestNewImageCustomizationContainer(t *testing.T) {
	testProxy := &v1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Status: v1.ProxyStatus{
			HTTPProxy:  "https://172.2.0.1:3128",
			HTTPSProxy: "https://172.2.0.1:3128",
			NoProxy:    ".example.com",
		},
	}

	images := Images{
		BaremetalOperator:   expectedBaremetalOperator,
		Ironic:              expectedIronic,
		IronicAgent:         expectedIronicAgent,
		MachineOsDownloader: expectedMachineOsDownloader,
		StaticIpManager:     expectedIronicStaticIpManager,
	}
	ironicIP := "192.168.0.2"

	container1 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "HTTP_PROXY", Value: "https://172.2.0.1:3128"},
			{Name: "HTTPS_PROXY", Value: "https://172.2.0.1:3128"},
			{Name: "NO_PROXY", Value: ".example.com"},
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2"},
			{Name: "IRONIC_INSPECTOR_BASE_URL", Value: "https://192.168.0.2"},
			{Name: "IRONIC_AGENT_IMAGE", Value: "registry.ci.openshift.org/openshift:ironic-agent"},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
			pullSecret,
		},
	}

	container2 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2"},
			{Name: "IRONIC_INSPECTOR_BASE_URL", Value: "https://192.168.0.3"},
			{Name: "IRONIC_AGENT_IMAGE", Value: "registry.ci.openshift.org/openshift:ironic-agent"},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
			pullSecret,
		},
	}

	tCases := []struct {
		name              string
		inspectorIP       string
		proxy             *v1.Proxy
		expectedContainer corev1.Container
	}{
		{
			name:              "image customization containe with proxy",
			inspectorIP:       ironicIP,
			proxy:             testProxy,
			expectedContainer: container1,
		},
		{
			name:              "image customization containe without proxy",
			inspectorIP:       "192.168.0.3",
			proxy:             nil,
			expectedContainer: container2,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			info := &ProvisioningInfo{
				SSHKey:       "sshkey",
				NetworkStack: NetworkStackV4,
				Proxy:        tc.proxy,
			}
			actualContainer := createImageCustomizationContainer(&images, info, ironicIP, tc.inspectorIP)
			for e := range actualContainer.Env {
				assert.EqualValues(t, tc.expectedContainer.Env[e], actualContainer.Env[e])
			}
		})
	}
}

func TestGetUrlFromIP(t *testing.T) {
	tests := []struct {
		ipAddr string
		want   string
	}{
		{
			ipAddr: "0:0:0:0:0:0:0:1",
			want:   "https://[0:0:0:0:0:0:0:1]",
		},
		{
			ipAddr: "127.0.0.1",
			want:   "https://127.0.0.1",
		},
		{
			ipAddr: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := getUrlFromIP(tt.ipAddr); got != tt.want {
				t.Errorf("getUrlFromIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

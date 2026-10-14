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
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
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
	ironicIP6 := "2001:db8::2"

	readOnlyImageVolumeMount := imageVolumeMount
	readOnlyImageVolumeMount.ReadOnly = true
	expectedVolumeMounts := []corev1.VolumeMount{
		imageRegistriesVolumeMount,
		readOnlyImageVolumeMount,
		ironicAgentPullSecretMount,
		ironicTlsMount,
		caTrustDirVolumeMount,
	}

	ntpServers := []string{"192.168.1.252", "192.168.1.253"}
	watchNamespaceEnv := corev1.EnvVar{
		Name: "WATCH_NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
		},
	}

	container1 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "HTTP_PROXY", Value: "https://172.2.0.1:3128"},
			{Name: "HTTPS_PROXY", Value: "https://172.2.0.1:3128"},
			{Name: "NO_PROXY", Value: ".example.com,192.168.0.2"},
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			watchNamespaceEnv,
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "DEPLOY_KERNEL", Value: "/shared/html/images/ironic-python-agent.kernel"},
			{Name: "IMAGE_SHARED_DIR", Value: "/shared/html/images"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2:6385"},
			{Name: "IRONIC_AGENT_IMAGE", Value: "registry.ci.openshift.org/openshift:ironic-agent"},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "ADDITIONAL_NTP_SERVERS", Value: ""},
			{Name: "CA_BUNDLE", Value: "/etc/pki/ca-trust/source/anchors/openshift-config-user-ca-bundle.crt"},
			{Name: "IRONIC_CA_BUNDLE", Value: "/certs/ironic/ca.crt"},
			{Name: "IRONIC_ROOTFS_URL", Value: "http://192.168.0.2:6180/images/ironic-python-agent.rootfs"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
		},
		VolumeMounts: expectedVolumeMounts,
	}
	secret1 := map[string]string{
		"IRONIC_BASE_URL":        "https://192.168.0.2:6385",
		"IRONIC_AGENT_IMAGE":     "registry.ci.openshift.org/openshift:ironic-agent",
		"IRONIC_RAMDISK_SSH_KEY": "sshkey",
	}

	container2 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			watchNamespaceEnv,
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "DEPLOY_KERNEL", Value: "/shared/html/images/ironic-python-agent.kernel"},
			{Name: "IMAGE_SHARED_DIR", Value: "/shared/html/images"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2:6385"},
			{Name: "IRONIC_AGENT_IMAGE", Value: "registry.ci.openshift.org/openshift:ironic-agent"},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "ADDITIONAL_NTP_SERVERS", Value: ""},
			{Name: "CA_BUNDLE", Value: "/etc/pki/ca-trust/source/anchors/openshift-config-user-ca-bundle.crt"},
			{Name: "IRONIC_CA_BUNDLE", Value: "/certs/ironic/ca.crt"},
			{Name: "IRONIC_ROOTFS_URL", Value: "http://192.168.0.2:6180/images/ironic-python-agent.rootfs"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
		},
		VolumeMounts: expectedVolumeMounts,
	}
	secret2 := map[string]string{
		"IRONIC_BASE_URL":        "https://192.168.0.2:6385",
		"IRONIC_AGENT_IMAGE":     "registry.ci.openshift.org/openshift:ironic-agent",
		"IRONIC_RAMDISK_SSH_KEY": "sshkey",
	}

	container3 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "HTTP_PROXY", Value: "https://172.2.0.1:3128"},
			{Name: "HTTPS_PROXY", Value: "https://172.2.0.1:3128"},
			{Name: "NO_PROXY", Value: ".example.com,192.168.0.2,2001:db8::2"},
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			watchNamespaceEnv,
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "DEPLOY_KERNEL", Value: "/shared/html/images/ironic-python-agent.kernel"},
			{Name: "IMAGE_SHARED_DIR", Value: "/shared/html/images"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2:6385,https://[2001:db8::2]:6385"},
			{Name: "IRONIC_AGENT_IMAGE", Value: "registry.ci.openshift.org/openshift:ironic-agent"},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "ADDITIONAL_NTP_SERVERS", Value: ""},
			{Name: "CA_BUNDLE", Value: "/etc/pki/ca-trust/source/anchors/openshift-config-user-ca-bundle.crt"},
			{Name: "IRONIC_CA_BUNDLE", Value: "/certs/ironic/ca.crt"},
			{Name: "IRONIC_ROOTFS_URL", Value: "http://192.168.0.2:6180/images/ironic-python-agent.rootfs"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
		},
		VolumeMounts: expectedVolumeMounts,
	}
	secret3 := map[string]string{
		"IRONIC_BASE_URL":        "https://192.168.0.2:6385,https://[2001:db8::2]:6385",
		"IRONIC_AGENT_IMAGE":     "registry.ci.openshift.org/openshift:ironic-agent",
		"IRONIC_RAMDISK_SSH_KEY": "sshkey",
	}

	container4 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			watchNamespaceEnv,
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "DEPLOY_KERNEL", Value: "/shared/html/images/ironic-python-agent.kernel"},
			{Name: "IMAGE_SHARED_DIR", Value: "/shared/html/images"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2:6385"},
			{Name: "IRONIC_AGENT_IMAGE", Value: "registry.ci.openshift.org/openshift:ironic-agent"},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "ADDITIONAL_NTP_SERVERS", Value: "192.168.1.252,192.168.1.253"},
			{Name: "CA_BUNDLE", Value: "/etc/pki/ca-trust/source/anchors/openshift-config-user-ca-bundle.crt"},
			{Name: "IRONIC_CA_BUNDLE", Value: "/certs/ironic/ca.crt"},
			{Name: "IRONIC_ROOTFS_URL", Value: "http://192.168.0.2:6180/images/ironic-python-agent.rootfs"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
		},
		VolumeMounts: expectedVolumeMounts,
	}
	secret4 := map[string]string{
		"IRONIC_BASE_URL":        "https://192.168.0.2:6385",
		"IRONIC_AGENT_IMAGE":     "registry.ci.openshift.org/openshift:ironic-agent",
		"IRONIC_RAMDISK_SSH_KEY": "sshkey",
	}

	imageOverride := "example.com/image:latest"
	container5 := corev1.Container{
		Name: "image-customization-controller",
		Env: []corev1.EnvVar{
			{Name: "DEPLOY_ISO", Value: "/shared/html/images/ironic-python-agent.iso"},
			watchNamespaceEnv,
			{Name: "DEPLOY_INITRD", Value: "/shared/html/images/ironic-python-agent.initramfs"},
			{Name: "DEPLOY_KERNEL", Value: "/shared/html/images/ironic-python-agent.kernel"},
			{Name: "IMAGE_SHARED_DIR", Value: "/shared/html/images"},
			{Name: "IRONIC_BASE_URL", Value: "https://192.168.0.2:6385"},
			{Name: "IRONIC_AGENT_IMAGE", Value: imageOverride},
			{Name: "REGISTRIES_CONF_PATH", Value: "/etc/containers/registries.conf"},
			{Name: "IP_OPTIONS", Value: "ip=dhcp"},
			{Name: "ADDITIONAL_NTP_SERVERS", Value: ""},
			{Name: "CA_BUNDLE", Value: "/etc/pki/ca-trust/source/anchors/openshift-config-user-ca-bundle.crt"},
			{Name: "IRONIC_CA_BUNDLE", Value: "/certs/ironic/ca.crt"},
			{Name: "IRONIC_ROOTFS_URL", Value: "http://192.168.0.2:6180/images/ironic-python-agent.rootfs"},
			{Name: "IRONIC_RAMDISK_SSH_KEY", Value: "sshkey"},
		},
		VolumeMounts: expectedVolumeMounts,
	}
	secret5 := secret2

	tCases := []struct {
		name              string
		ironicIPs         []string
		proxy             *v1.Proxy
		expectedContainer corev1.Container
		expectedSecret    map[string]string
		ntpServers        []string
		ironicAgentImage  string
	}{
		{
			name:              "image customization container with proxy",
			ironicIPs:         []string{ironicIP},
			proxy:             testProxy,
			expectedContainer: container1,
			expectedSecret:    secret1,
			ntpServers:        []string{},
		},
		{
			name:              "image customization container without proxy",
			ironicIPs:         []string{ironicIP},
			proxy:             nil,
			expectedContainer: container2,
			expectedSecret:    secret2,
			ntpServers:        []string{},
		},
		{
			name:              "image customization container with proxy",
			ironicIPs:         []string{ironicIP, ironicIP6},
			proxy:             testProxy,
			expectedContainer: container3,
			expectedSecret:    secret3,
			ntpServers:        []string{},
		},
		{
			name:              "image customization container with additional NTP servers",
			ironicIPs:         []string{ironicIP},
			proxy:             nil,
			expectedContainer: container4,
			expectedSecret:    secret4,
			ntpServers:        ntpServers,
		},
		{
			name:              "image customization container with agent override",
			ironicIPs:         []string{ironicIP},
			expectedContainer: container5,
			expectedSecret:    secret5,
			ironicAgentImage:  imageOverride,
		},
	}
	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)
			var overrides *v1alpha1.UnsupportedConfigOverrides
			if tc.ironicAgentImage != "" {
				overrides = &v1alpha1.UnsupportedConfigOverrides{
					IronicAgentImage: tc.ironicAgentImage,
				}
			}
			info := &ProvisioningInfo{
				Images:       &images,
				SSHKey:       "sshkey",
				NetworkStack: NetworkStackV4,
				Proxy:        tc.proxy,
				ProvConfig: &v1alpha1.Provisioning{
					Spec: v1alpha1.ProvisioningSpec{
						AdditionalNTPServers:       tc.ntpServers,
						UnsupportedConfigOverrides: overrides,
					},
				},
			}
			actualContainer := createImageCustomizationContainer(&images, info, tc.ironicIPs)
			assert.Equal(t, tc.expectedContainer.Env, actualContainer.Env)
			actualSecret := newImageCustomizationConfig(info, tc.ironicIPs)
			assert.Equal(t, tc.expectedSecret, actualSecret.StringData)
			assert.Equal(t, tc.expectedContainer.VolumeMounts, actualContainer.VolumeMounts)
		})
	}
}

func TestImageCustomizationTLSResources(t *testing.T) {
	info := &ProvisioningInfo{
		Namespace:    "openshift-machine-api",
		Images:       &Images{},
		ProvConfig:   &v1alpha1.Provisioning{},
		NetworkStack: NetworkStackV4,
		TlsCertHash:  "initial-cert-hash",
	}
	deployment := newImageCustomizationDeployment(info, []string{"192.0.2.1"})
	require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, []string{
		"/machine-image-customization-controller",
		"-images-bind-addr", ":8084",
		"-images-publish-addr", "https://metal3-image-customization-service.openshift-machine-api.svc.cluster.local/",
		"-images-tls-cert-file", "/certs/ironic/tls.crt",
		"-images-tls-key-file", "/certs/ironic/tls.key",
	}, container.Command)
	assert.Contains(t, container.Env, corev1.EnvVar{Name: "IRONIC_CA_BUNDLE", Value: "/certs/ironic/ca.crt"})
	assert.Contains(t, container.VolumeMounts, corev1.VolumeMount{
		Name:      "metal3-ironic-tls",
		MountPath: "/certs/ironic",
		ReadOnly:  true,
	})
	assert.Contains(t, deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "metal3-ironic-tls",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: "metal3-ironic-tls"},
		},
	})
	assert.Equal(t, []corev1.ContainerPort{{Name: "https", ContainerPort: 8084}}, container.Ports)

	service := newImageCustomizationService(info.Namespace)
	assert.Equal(t, "metal3-image-customization-service", service.Name)
	assert.Equal(t, info.Namespace, service.Namespace)
	assert.Equal(t, []corev1.ServicePort{{
		Name:       "https",
		Port:       443,
		TargetPort: intstr.FromInt(8084),
	}}, service.Spec.Ports)
	require.NotEmpty(t, service.Spec.Selector)
	for key, value := range service.Spec.Selector {
		assert.Equal(t, value, deployment.Spec.Template.Labels[key], "Service must select the ICC pods")
	}

	const hashAnnotation = "baremetal.openshift.io/ironic-tls-cert-hash"
	assert.Equal(t, "initial-cert-hash", deployment.Spec.Template.Annotations[hashAnnotation])
	info.TlsCertHash = "rotated-cert-hash"
	rotated := newImageCustomizationDeployment(info, []string{"192.0.2.1"})
	assert.Equal(t, "rotated-cert-hash", rotated.Spec.Template.Annotations[hashAnnotation])
	assert.NotEqual(t, deployment.Spec.Template, rotated.Spec.Template, "certificate rotation must trigger an ICC rollout")

	info.TlsCertHash = ""
	withoutHash := newImageCustomizationDeployment(info, []string{"192.0.2.1"})
	assert.NotContains(t, withoutHash.Spec.Template.Annotations, hashAnnotation)
}

func TestGetUrlFromIP(t *testing.T) {
	tests := []struct {
		ipAddr []string
		want   string
	}{
		{
			ipAddr: []string{"0:0:0:0:0:0:0:1"},
			want:   "https://[0:0:0:0:0:0:0:1]:6385",
		},
		{
			ipAddr: []string{"127.0.0.1"},
			want:   "https://127.0.0.1:6385",
		},
		{
			ipAddr: []string{"2001:db8::1", "192.0.2.1"},
			want:   "https://[2001:db8::1]:6385,https://192.0.2.1:6385",
		},
		{
			ipAddr: nil,
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := getUrlFromIP(tt.ipAddr, 6385); got != tt.want {
				t.Errorf("getUrlFromIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

package provisioning

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"

	osconfigv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestNewBMOContainers(t *testing.T) {
	envWithValue := func(name, value string) corev1.EnvVar {
		return corev1.EnvVar{Name: name, Value: value}
	}
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
	primaryIP := "192.168.111.1"
	realIP := "192.168.111.22"
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
				{Name: "DEPLOY_KERNEL_URL", Value: "file:///shared/html/images/ironic-python-agent.kernel"},
				{Name: "IRONIC_ENDPOINT", Value: fmt.Sprintf("https://%s:6385/v1/", primaryIP)},
				{Name: "IRONIC_INSPECTOR_ENDPOINT", Value: fmt.Sprintf("https://%s:5050/v1/", primaryIP)},
				{Name: "LIVE_ISO_FORCE_PERSISTENT_BOOT_DEVICE", Value: "Never"},
				{Name: "METAL3_AUTH_ROOT_DIR", Value: "/auth"},
				{Name: "IRONIC_EXTERNAL_IP", Value: ""},
				{Name: "IRONIC_EXTERNAL_URL_V6", Value: ""},
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
		for _, value := range newMap {
			new = append(new, value)
		}
		c.Env = new
		return c
	}
	images := Images{
		BaremetalOperator: expectedBaremetalOperator,
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
				withEnv(
					containers["metal3-baremetal-operator"],
					envWithValue("IRONIC_EXTERNAL_URL_V6", "https://[fd2e:6f44:5dd8:c956::16]:6183"),
					envWithValue(
						"IRONIC_ENDPOINT",
						fmt.Sprintf("https://%s:6385/v1/", realIP),
					),
					envWithValue(
						"IRONIC_INSPECTOR_ENDPOINT",
						fmt.Sprintf("https://%s:5050/v1/", realIP),
					),
				),
			},
			sshkey: "sshkey",
		},
		{
			name:   "ManagedSpec with virtualmedia",
			config: managedProvisioning().VirtualMediaViaExternalNetwork(true).build(),
			expectedContainers: []corev1.Container{
				withEnv(
					containers["metal3-baremetal-operator"],
					envWithFieldValue("IRONIC_EXTERNAL_IP", "status.hostIP"),
					envWithValue("IRONIC_EXTERNAL_URL_V6", "https://[fd2e:6f44:5dd8:c956::16]:6183"),
				),
			},
			sshkey: "sshkey",
		},
		{
			name:   "DisabledSpec",
			config: disabledProvisioning().build(),
			expectedContainers: []corev1.Container{
				withEnv(
					containers["metal3-baremetal-operator"],
					envWithValue("IRONIC_EXTERNAL_IP", ""),
					envWithValue("IRONIC_EXTERNAL_URL_V6", "https://[fd2e:6f44:5dd8:c956::16]:6183"),
				),
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
				Client: fakekube.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "openshift-machine-api",
						Labels: map[string]string{
							"k8s-app":    metal3AppName,
							cboLabelName: stateService,
						},
					},
					Status: corev1.PodStatus{
						HostIP: realIP,
						PodIPs: []corev1.PodIP{
							{IP: realIP},
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
						Status: v1.InfrastructureStatus{
							PlatformStatus: &v1.PlatformStatus{
								Type: v1.BareMetalPlatformType,
								BareMetal: &v1.BareMetalPlatformStatus{
									APIServerInternalIPs: []string{
										primaryIP,
										"fd2e:6f44:5dd8:c956::16",
									},
								},
							},
						},
					}),
			}
			templateSpec, err := newBMOPodTemplateSpec(info, &map[string]string{})
			assert.NoError(t, err)
			actualContainers := templateSpec.Spec.Containers

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

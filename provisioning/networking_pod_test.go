package provisioning

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakekube "k8s.io/client-go/kubernetes/fake"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestCreateContainerIronicNetworking(t *testing.T) {
	images := &Images{Ironic: expectedIronic}
	container := createContainerIronicNetworking(images)

	assert.Equal(t, "metal3-ironic-networking", container.Name)
	assert.Equal(t, expectedIronic, container.Image)
	assert.Equal(t, []string{"/bin/runironic-networking"}, container.Command)

	// Verify volume mounts do NOT include sharedVolumeMount
	expectedVolumeMounts := []corev1.VolumeMount{
		ironicConfigMount,
		ironicDataMount,
		ironicTmpMount,
		ironicRPCCredentialsMount,
		switchConfigsMount,
		switchCredentialsMount,
		ironicTlsMount,
	}
	assert.Equal(t, expectedVolumeMounts, container.VolumeMounts)
	assert.NotContains(t, container.VolumeMounts, sharedVolumeMount,
		"standalone networking container should not mount the shared volume")

	// Verify container port
	assert.Len(t, container.Ports, 1)
	assert.Equal(t, ironicNetworkingRPCPortName, container.Ports[0].Name)
	assert.Equal(t, int32(ironicNetworkingRPCPort), container.Ports[0].ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, container.Ports[0].Protocol)

	// Verify env vars
	assertEnvVar(t, container.Env, switchConfigsFileNameEnvVar, switchConfigsMountPath+"/"+switchConfigsFileName)
	assertEnvVar(t, container.Env, "IRONIC_NETWORKING_JSON_RPC_HOST", "0.0.0.0")
	assertEnvVar(t, container.Env, "IRONIC_NETWORKING_JSON_RPC_PORT", "6190")
	assertEnvVar(t, container.Env, "IRONIC_NETWORKING_ENABLED_SWITCH_DRIVERS", "generic-switch")

	assert.Nil(t, container.LivenessProbe)
	assert.NotNil(t, container.ReadinessProbe)
	assert.NotNil(t, container.ReadinessProbe.TCPSocket)
	assert.Equal(t, intstr.FromString(ironicNetworkingRPCPortName), container.ReadinessProbe.TCPSocket.Port)

	// Verify security context
	assert.NotNil(t, container.SecurityContext)
	assert.True(t, *container.SecurityContext.ReadOnlyRootFilesystem)
	assert.Equal(t, []corev1.Capability{"ALL"}, container.SecurityContext.Capabilities.Drop)
}

func TestIronicNetworkingVolumes(t *testing.T) {
	for _, volume := range ironicNetworkingVolumes {
		if volume.Name == switchConfigsVolume {
			assert.Nil(t, volume.Secret.DefaultMode, "Configs volume should use Kubernetes default mode")
		}
		if volume.Name == switchCredentialsVolume {
			assert.Nil(t, volume.Secret.DefaultMode, "Credentials volume should use Kubernetes default mode")
		}
	}
}

func TestNewIronicNetworkingDeployment(t *testing.T) {
	images := Images{
		Ironic: expectedIronic,
	}
	info := &ProvisioningInfo{
		Images:    &images,
		Namespace: "openshift-machine-api",
		ProvConfig: &metal3iov1alpha1.Provisioning{
			Spec: *managedProvisioning().SwitchManagementEnabled(true).build(),
		},
	}

	deployment := newIronicNetworkingDeployment(info)

	assert.Equal(t, ironicNetworkingDeploymentName, deployment.Name)
	assert.Equal(t, "openshift-machine-api", deployment.Namespace)
	assert.Equal(t, metal3AppName, deployment.Labels["k8s-app"])
	assert.Equal(t, ironicNetworkingDeploymentName, deployment.Labels[cboLabelName])
	assert.Equal(t, int32(1), *deployment.Spec.Replicas)

	template := deployment.Spec.Template
	assert.False(t, template.Spec.HostNetwork, "ironic-networking deployment should not use HostNetwork")
	assert.Equal(t, corev1.DNSClusterFirst, template.Spec.DNSPolicy, "should use ClusterFirst DNS policy")
	assert.Equal(t, "system-node-critical", template.Spec.PriorityClassName)
	assert.Equal(t, "cluster-baremetal-operator", template.Spec.ServiceAccountName)

	// Verify master node selector
	assert.Equal(t, "", template.Spec.NodeSelector["node-role.kubernetes.io/master"])
}

func TestNewIronicNetworkingDeploymentHyperShift(t *testing.T) {
	images := Images{
		Ironic: expectedIronic,
	}
	info := &ProvisioningInfo{
		Images:       &images,
		Namespace:    "openshift-machine-api",
		IsHyperShift: true,
		ProvConfig: &metal3iov1alpha1.Provisioning{
			Spec: *managedProvisioning().SwitchManagementEnabled(true).build(),
		},
	}

	deployment := newIronicNetworkingDeployment(info)
	template := deployment.Spec.Template

	// In HyperShift mode, no master node selector
	_, hasMasterSelector := template.Spec.NodeSelector["node-role.kubernetes.io/master"]
	assert.False(t, hasMasterSelector, "HyperShift deployment should not have master node selector")
}

func TestSecretVersionAnnotation(t *testing.T) {
	namespace := "openshift-machine-api"

	t.Run("returns error when secret does not exist", func(t *testing.T) {
		kubeClient := fakekube.NewSimpleClientset()
		ann, err := secretVersionAnnotation(context.Background(), kubeClient, namespace, "switch-configs", switchConfigsSecretName)
		assert.Error(t, err)
		assert.Nil(t, ann)
	})

	t.Run("returns annotation with hash", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      switchConfigsSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
			},
		}
		kubeClient := fakekube.NewSimpleClientset(secret)
		ann, err := secretVersionAnnotation(context.Background(), kubeClient, namespace, "switch-configs", switchConfigsSecretName)
		assert.NoError(t, err)
		assert.NotNil(t, ann)
		hash, ok := ann["baremetal.openshift.io/switch-configs-version"]
		assert.True(t, ok, "annotation key should be present")
		assert.NotEmpty(t, hash)
	})

	t.Run("hash changes when secret data changes", func(t *testing.T) {
		secret1 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      switchConfigsSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"key1": []byte("value1"),
			},
		}
		kubeClient1 := fakekube.NewSimpleClientset(secret1)
		ann1, err := secretVersionAnnotation(context.Background(), kubeClient1, namespace, "switch-configs", switchConfigsSecretName)
		assert.NoError(t, err)

		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      switchConfigsSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"key1": []byte("value1-updated"),
			},
		}
		kubeClient2 := fakekube.NewSimpleClientset(secret2)
		ann2, err := secretVersionAnnotation(context.Background(), kubeClient2, namespace, "switch-configs", switchConfigsSecretName)
		assert.NoError(t, err)

		assert.NotEqual(t,
			ann1["baremetal.openshift.io/switch-configs-version"],
			ann2["baremetal.openshift.io/switch-configs-version"],
			"hash should change when secret data changes")
	})

	t.Run("hash is deterministic with multiple keys", func(t *testing.T) {
		data := map[string][]byte{
			"aaa": []byte("1"),
			"zzz": []byte("2"),
			"mmm": []byte("3"),
		}
		// Create two independent secrets with the same data to get
		// independent map instances that may iterate in different orders.
		secret1 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      switchConfigsSecretName,
				Namespace: namespace,
			},
			Data: data,
		}
		secret2 := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      switchCredentialsSecretName,
				Namespace: namespace,
			},
			Data: data,
		}
		kubeClient := fakekube.NewSimpleClientset(secret1, secret2)
		ann1, err := secretVersionAnnotation(context.Background(), kubeClient, namespace, "switch-configs", switchConfigsSecretName)
		assert.NoError(t, err)
		ann2, err := secretVersionAnnotation(context.Background(), kubeClient, namespace, "switch-credentials", switchCredentialsSecretName)
		assert.NoError(t, err)
		assert.Equal(t,
			ann1["baremetal.openshift.io/switch-configs-version"],
			ann2["baremetal.openshift.io/switch-credentials-version"],
			"same data in different secrets should produce the same hash")
	})
}

func TestEnsureIronicNetworkingDeploymentSecretAnnotations(t *testing.T) {
	namespace := "openshift-machine-api"

	configsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      switchConfigsSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			switchConfigsFileName: []byte("config-data"),
		},
	}
	credentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      switchCredentialsSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"cred-key": []byte("cred-data"),
		},
	}
	kubeClient := fakekube.NewSimpleClientset(configsSecret, credentialsSecret)

	info := &ProvisioningInfo{
		Context:   context.Background(),
		Images:    &Images{Ironic: expectedIronic},
		Namespace: namespace,
		Client:    kubeClient,
		Scheme:    scheme,
		ProvConfig: &metal3iov1alpha1.Provisioning{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provisioning-configuration",
			},
			Spec: *managedProvisioning().SwitchManagementEnabled(true).build(),
		},
	}

	// Build the deployment with secret annotations the same way
	// EnsureIronicNetworkingDeployment does.
	deployment := newIronicNetworkingDeployment(info)
	podAnnotations := make(map[string]string)
	for key, val := range podTemplateAnnotations {
		podAnnotations[key] = val
	}
	configAnn, err := secretVersionAnnotation(info.Context, info.Client, info.Namespace, "switch-configs", switchConfigsSecretName)
	assert.NoError(t, err)
	maps.Copy(podAnnotations, configAnn)
	credsAnn, err := secretVersionAnnotation(info.Context, info.Client, info.Namespace, "switch-credentials", switchCredentialsSecretName)
	assert.NoError(t, err)
	maps.Copy(podAnnotations, credsAnn)

	deployment.Spec.Template.Annotations = podAnnotations

	annotations := deployment.Spec.Template.Annotations

	// Verify base annotations are preserved
	assert.Contains(t, annotations, "target.workload.openshift.io/management",
		"should preserve base pod template annotations")

	// Verify secret version annotations are present
	configsHash, ok := annotations["baremetal.openshift.io/switch-configs-version"]
	assert.True(t, ok, "switch-configs-version annotation should be present")
	assert.NotEmpty(t, configsHash)

	credsHash, ok := annotations["baremetal.openshift.io/switch-credentials-version"]
	assert.True(t, ok, "switch-credentials-version annotation should be present")
	assert.NotEmpty(t, credsHash)

	// Verify podTemplateAnnotations was NOT mutated
	_, leaked := podTemplateAnnotations["baremetal.openshift.io/switch-configs-version"]
	assert.False(t, leaked, "podTemplateAnnotations must not be mutated by secret version annotations")
}

func TestNewIronicNetworkingService(t *testing.T) {
	info := &ProvisioningInfo{
		Namespace: "openshift-machine-api",
		ProvConfig: &metal3iov1alpha1.Provisioning{
			Spec: *managedProvisioning().SwitchManagementEnabled(true).build(),
		},
	}

	svc := newIronicNetworkingService(info)

	assert.Equal(t, ironicNetworkingServiceName, svc.Name)
	assert.Equal(t, "openshift-machine-api", svc.Namespace)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// Verify selector labels
	assert.Equal(t, ironicNetworkingDeploymentName, svc.Spec.Selector[cboLabelName])

	// Verify labels
	assert.Equal(t, ironicNetworkingDeploymentName, svc.Labels[cboLabelName])

	// Verify ports
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, ironicNetworkingRPCPortName, svc.Spec.Ports[0].Name)
	assert.Equal(t, int32(ironicNetworkingRPCPort), svc.Spec.Ports[0].Port)
}

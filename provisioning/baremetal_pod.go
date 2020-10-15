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
	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	baremetalDeploymentName    = "metal3"
	baremetalSharedVolume      = "metal3-shared"
	metal3AuthRootDir          = "/auth"
	ironicCredentialsVolume    = "metal3-ironic-basic-auth"
	inspectorCredentialsVolume = "metal3-inspector-basic-auth"
	htpasswdEnvVar             = "HTTP_BASIC_HTPASSWD" // #nosec
	mariadbPwdEnvVar           = "MARIADB_PASSWORD"    // #nosec
	cboOwnedAnnotation         = "baremetal.openshift.io/owned"
)

var sharedVolumeMount = corev1.VolumeMount{
	Name:      baremetalSharedVolume,
	MountPath: "/shared",
}

var ironicCredentialsMount = corev1.VolumeMount{
	Name:      ironicCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic",
	ReadOnly:  true,
}

var inspectorCredentialsMount = corev1.VolumeMount{
	Name:      inspectorCredentialsVolume,
	MountPath: metal3AuthRootDir + "/ironic-inspector",
	ReadOnly:  true,
}

func buildEnvVar(name string, baremetalProvisioningConfig *metal3iov1alpha1.ProvisioningSpec) corev1.EnvVar {
	value := getMetal3DeploymentConfig(name, baremetalProvisioningConfig)
	if value != nil {
		return corev1.EnvVar{
			Name:  name,
			Value: *value,
		}
	} else {
		return corev1.EnvVar{
			Name: name,
		}
	}
}

func setMariadbPassword() corev1.EnvVar {
	return corev1.EnvVar{
		Name: mariadbPwdEnvVar,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: baremetalSecretName,
				},
				Key: baremetalSecretKey,
			},
		},
	}
}

func setIronicHtpasswdHash(name string, secretName string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: ironicHtpasswdKey,
			},
		},
	}
}

func newMetal3InitContainers(images *Images, config *metal3iov1alpha1.ProvisioningSpec) []corev1.Container {
	initContainers := []corev1.Container{
		createInitContainerIpaDownloader(images),
		createInitContainerMachineOsDownloader(images, config),
		createInitContainerStaticIpSet(images, config),
	}
	return initContainers
}

func createInitContainerIpaDownloader(images *Images) corev1.Container {
	initContainer := corev1.Container{
		Name:            "metal3-ipa-downloader",
		Image:           images.IpaDownloader,
		Command:         []string{"/usr/local/bin/get-resource.sh"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env:          []corev1.EnvVar{},
	}
	return initContainer
}

func createInitContainerMachineOsDownloader(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	initContainer := corev1.Container{
		Name:            "metal3-machine-os-downloader",
		Image:           images.MachineOsDownloader,
		Command:         []string{"/usr/local/bin/get-resource.sh"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env: []corev1.EnvVar{
			buildEnvVar(machineImageUrl, config),
		},
	}
	return initContainer
}

func createInitContainerStaticIpSet(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	initContainer := corev1.Container{
		Name:            "metal3-static-ip-set",
		Image:           images.StaticIpManager,
		Command:         []string{"/set-static-ip"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
	}
	return initContainer
}

func newMetal3Containers(images *Images, config *metal3iov1alpha1.ProvisioningSpec) []corev1.Container {
	containers := []corev1.Container{
		createContainerMetal3BaremetalOperator(images, config),
		createContainerMetal3Mariadb(images),
		createContainerMetal3Httpd(images, config),
		createContainerMetal3IronicConductor(images, config),
		createContainerMetal3IronicApi(images, config),
		createContainerMetal3IronicInspector(images, config),
		createContainerMetal3StaticIpManager(images, config),
	}
	if config.ProvisioningNetwork != metal3iov1alpha1.ProvisioningNetworkDisabled {
		containers = append(containers, createContainerMetal3Dnsmasq(images, config))
	}
	return containers
}

func createContainerMetal3BaremetalOperator(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:  "metal3-baremetal-operator",
		Image: images.BaremetalOperator,
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 60000,
			},
		},
		Command:         []string{"/baremetal-operator"},
		ImagePullPolicy: "IfNotPresent",
		VolumeMounts: []corev1.VolumeMount{
			ironicCredentialsMount,
			inspectorCredentialsMount,
		},
		Env: []corev1.EnvVar{
			{
				Name: "WATCH_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "OPERATOR_NAME",
				Value: "baremetal-operator",
			},
			buildEnvVar(deployKernelUrl, config),
			buildEnvVar(deployRamdiskUrl, config),
			buildEnvVar(ironicEndpoint, config),
			buildEnvVar(ironicInspectorEndpoint, config),
			{
				Name:  "METAL3_AUTH_ROOT_DIR",
				Value: metal3AuthRootDir,
			},
		},
	}
	return container
}

func createContainerMetal3Dnsmasq(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-dnsmasq",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/rundnsmasq"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env: []corev1.EnvVar{
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningInterface, config),
			buildEnvVar(dhcpRange, config),
		},
	}
	return container
}

func createContainerMetal3Mariadb(images *Images) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-mariadb",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runmariadb"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env: []corev1.EnvVar{
			setMariadbPassword(),
		},
	}
	return container
}

func createContainerMetal3Httpd(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-httpd",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runhttpd"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env: []corev1.EnvVar{
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
	}
	return container
}

func createContainerMetal3IronicConductor(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-ironic-conductor",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command: []string{"/bin/runironic-conductor"},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			inspectorCredentialsMount,
		},
		Env: []corev1.EnvVar{
			setMariadbPassword(),
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
	}
	return container
}

func createContainerMetal3IronicApi(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-ironic-api",
		Image:           images.Ironic,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Command:      []string{"/bin/runironic-api"},
		VolumeMounts: []corev1.VolumeMount{sharedVolumeMount},
		Env: []corev1.EnvVar{
			setMariadbPassword(),
			buildEnvVar(httpPort, config),
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			setIronicHtpasswdHash(htpasswdEnvVar, ironicSecretName),
		},
	}
	return container
}

func createContainerMetal3IronicInspector(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
	container := corev1.Container{
		Name:            "metal3-ironic-inspector",
		Image:           images.IronicInspector,
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			sharedVolumeMount,
			ironicCredentialsMount,
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
			setIronicHtpasswdHash(htpasswdEnvVar, inspectorSecretName),
		},
	}
	return container
}

func createContainerMetal3StaticIpManager(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {

	container := corev1.Container{
		Name:            "metal3-static-ip-manager",
		Image:           images.StaticIpManager,
		Command:         []string{"/refresh-static-ip"},
		ImagePullPolicy: "IfNotPresent",
		SecurityContext: &corev1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
		Env: []corev1.EnvVar{
			buildEnvVar(provisioningIP, config),
			buildEnvVar(provisioningInterface, config),
		},
	}
	return container
}

func newMetal3Volumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: baremetalSharedVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: ironicCredentialsVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: ironicSecretName,
					Items: []corev1.KeyToPath{
						{Key: ironicUsernameKey, Path: ironicUsernameKey},
						{Key: ironicPasswordKey, Path: ironicPasswordKey},
						{Key: ironicConfigKey, Path: ironicConfigKey},
					},
				},
			},
		},
		{
			Name: inspectorCredentialsVolume,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: inspectorSecretName,
					Items: []corev1.KeyToPath{
						{Key: ironicUsernameKey, Path: ironicUsernameKey},
						{Key: ironicPasswordKey, Path: ironicPasswordKey},
						{Key: ironicConfigKey, Path: ironicConfigKey},
					},
				},
			},
		},
	}
	return volumes
}

func newMetal3PodTemplateSpec(images *Images, config *metal3iov1alpha1.ProvisioningSpec) *corev1.PodTemplateSpec {
	initContainers := newMetal3InitContainers(images, config)
	containers := newMetal3Containers(images, config)
	volumes := newMetal3Volumes()
	tolerations := []corev1.Toleration{
		{
			Key:    "node-role.kubernetes.io/master",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "CriticalAddonsOnly",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:               "node.kubernetes.io/not-ready",
			Effect:            corev1.TaintEffectNoExecute,
			Operator:          corev1.TolerationOpExists,
			TolerationSeconds: pointer.Int64Ptr(120),
		},
		{
			Key:               "node.kubernetes.io/unreachable",
			Effect:            corev1.TaintEffectNoExecute,
			Operator:          corev1.TolerationOpExists,
			TolerationSeconds: pointer.Int64Ptr(120),
		},
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"api":     "clusterapi",
				"k8s-app": "controller",
			},
		},
		Spec: corev1.PodSpec{
			Volumes:           volumes,
			InitContainers:    initContainers,
			Containers:        containers,
			HostNetwork:       true,
			PriorityClassName: "system-node-critical",
			NodeSelector:      map[string]string{"node-role.kubernetes.io/master": ""},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: pointer.BoolPtr(false),
			},
			ServiceAccountName: "cluster-baremetal-operator",
			Tolerations:        tolerations,
		},
	}
}

func NewMetal3Deployment(targetNamespace string, images *Images, config *metal3iov1alpha1.ProvisioningSpec) *appsv1.Deployment {
	replicas := int32(1)
	template := newMetal3PodTemplateSpec(images, config)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baremetalDeploymentName,
			Namespace: targetNamespace,
			Annotations: map[string]string{
				cboOwnedAnnotation: "",
			},
			Labels: map[string]string{
				"k8s-app": "controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "controller",
				},
			},
			Template: *template,
		},
	}
}

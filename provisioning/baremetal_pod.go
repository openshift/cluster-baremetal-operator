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

	corev1 "k8s.io/api/core/v1"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	baremetalSharedVolume = "metal3-shared"
)

var sharedVolumeMount = corev1.VolumeMount{
	Name:      baremetalSharedVolume,
	MountPath: "/shared",
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

func newMetal3InitContainers(images *Images, config *metal3iov1alpha1.ProvisioningSpec) []corev1.Container {
	initContainers := []corev1.Container{
		createInitContainerIpaDownloader(images, config),
		createInitContainerMachineOsDownloader(images, config),
		createInitContainerStaticIpSet(images, config),
	}
	return initContainers
}

func createInitContainerIpaDownloader(images *Images, config *metal3iov1alpha1.ProvisioningSpec) corev1.Container {
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

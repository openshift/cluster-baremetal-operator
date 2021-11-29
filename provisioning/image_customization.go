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
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	ironicBaseUrl    = "IRONIC_BASE_URL"
	ironicAgentImage = "IRONIC_AGENT_IMAGE"
)

func imageRegistriesVolume() corev1.Volume {
	// TODO: Should this be corev1.HostPathFile?
	volType := corev1.HostPathFileOrCreate

	return corev1.Volume{
		Name: imageCustomizationVolume,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: containerRegistriesConfPath,
				Type: &volType,
			},
		},
	}
}

func hostForIP(ipAddr string) string {
	if strings.Contains(ipAddr, ":") {
		return fmt.Sprintf("[%s]", ipAddr)
	}
	return ipAddr
}

func setIronicBaseUrl(name string, info *ProvisioningInfo) corev1.EnvVar {
	config := info.ProvConfig.Spec
	if config.ProvisioningNetwork != metal3iov1alpha1.ProvisioningNetworkDisabled && !config.VirtualMediaViaExternalNetwork {
		return corev1.EnvVar{
			Name:  name,
			Value: "https://" + hostForIP(config.ProvisioningIP),
		}
	} else {
		hostIP, err := getPodHostIP(info.Client.CoreV1(), info.Namespace)
		if err == nil && hostIP != "" {
			return corev1.EnvVar{
				Name:  name,
				Value: "https://" + hostForIP(hostIP),
			}
		}
		return corev1.EnvVar{
			Name: name,
		}
	}
}

func createImageCustomizationContainer(images *Images, info *ProvisioningInfo) corev1.Container {
	container := corev1.Container{
		Name:            "image-customization-controller",
		Image:           images.ImageCustomizationController,
		Command:         []string{"/image-customization-controller"},
		ImagePullPolicy: "IfNotPresent",
		Env: []corev1.EnvVar{
			setIronicBaseUrl(ironicBaseUrl, info),
			{
				Name:  ironicAgentImage,
				Value: images.IronicAgent,
			},
			buildSSHKeyEnvVar(info.SSHKey),
			pullSecret,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
	}
	return container
}

func newImageCustomizationPodTemplateSpec(info *ProvisioningInfo, labels *map[string]string) *corev1.PodTemplateSpec {
	containers := []corev1.Container{
		createImageCustomizationContainer(info.Images, info),
	}

	tolerations := []corev1.Toleration{
		{
			Key:      "node-role.kubernetes.io/master",
			Effect:   corev1.TaintEffectNoSchedule,
			Operator: corev1.TolerationOpExists,
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
			Annotations: podTemplateAnnotations,
			Labels:      *labels,
		},
		Spec: corev1.PodSpec{
			Containers:         containers,
			HostNetwork:        false,
			DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
			PriorityClassName:  "system-node-critical",
			NodeSelector:       map[string]string{"node-role.kubernetes.io/master": ""},
			ServiceAccountName: "cluster-baremetal-operator",
			Tolerations:        tolerations,
		},
	}
}

func newImageCustomizationDeployment(info *ProvisioningInfo) *appsv1.Deployment {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"k8s-app":    metal3AppName,
			cboLabelName: imageCustomizationService,
		},
	}
	podSpecLabels := map[string]string{
		"k8s-app":    metal3AppName,
		cboLabelName: imageCustomizationService,
	}
	template := newImageCustomizationPodTemplateSpec(info, &podSpecLabels)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "metal3ImageCustomization",
			Namespace:   info.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				"k8s-app":    metal3AppName,
				cboLabelName: imageCustomizationService,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: selector,
			Template: *template,
		},
	}
}

func EnsureImageCustomizationDeployment(info *ProvisioningInfo) (updated bool, err error) {
	_ = newImageCustomizationDeployment(info)
	return false, nil
}

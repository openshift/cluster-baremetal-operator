package provisioning

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func createInitContainerMachineOSImages(info *ProvisioningInfo, whichImages string, dest corev1.VolumeMount, destPath string) corev1.Container {
	ipOptionValue := ipOptionForExternal(info)
	if !info.ProvConfig.Spec.VirtualMediaViaExternalNetwork {
		ipOptionValue = ipOptionForProvisioning(info)
	}

	container := corev1.Container{
		Name:    "machine-os-images",
		Image:   info.Images.MachineOSImages,
		Command: []string{"/bin/copy-metal", whichImages, destPath},
		VolumeMounts: []corev1.VolumeMount{
			dest,
		},
		ImagePullPolicy: "IfNotPresent",
		Env: []corev1.EnvVar{
			{
				Name:  ipOptions,
				Value: ipOptionValue,
			},
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

package provisioning

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	imageCacheSharedVolume = "metal3-shared-image-cache"
)

func imageVolume() corev1.Volume {
	volType := corev1.HostPathDirectoryOrCreate
	return corev1.Volume{
		Name: imageCacheSharedVolume,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/lib/metal3/images",
				Type: &volType,
			},
		},
	}
}

var imageVolumeMount = corev1.VolumeMount{
	Name:      imageCacheSharedVolume,
	MountPath: "/shared/html/images",
}

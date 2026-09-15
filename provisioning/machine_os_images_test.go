package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

func TestCreateInitContainerMachineOSImagesReleaseImage(t *testing.T) {
	releaseImage := "quay.io/openshift-release-dev/ocp-release@sha256:deadbeef"
	info := &ProvisioningInfo{
		Images:       &Images{MachineOSImages: expectedMachineOSImages},
		ProvConfig:   &metal3iov1alpha1.Provisioning{},
		NetworkStack: NetworkStackV4,
		ReleaseImage: releaseImage,
	}

	c := createInitContainerMachineOSImages(info, "--pxe", imageVolumeMount, imageSharedDir)

	assert.Equal(t, "machine-os-images", c.Name)
	assert.Contains(t, c.Env, corev1.EnvVar{Name: "MACHINE_OS_IMAGES_IMAGE", Value: expectedMachineOSImages})
	assert.Contains(t, c.Env, corev1.EnvVar{Name: "RELEASE_IMAGE", Value: releaseImage})
}

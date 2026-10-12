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
)

func TestGetImageVolumes(t *testing.T) {
	volumes := getImageVolumes()

	// Verify the expected volumes are present
	expectedVolumeNames := []string{
		imageCacheSharedVolume, // hostPath for /shared/html/images
		"trusted-ca",
		ironicConfigVolume,
		ironicDataVolume,
		baremetalSharedVolume, // emptyDir for /shared (needed for /shared/tmp)
		ironicTmpVolume,       // emptyDir for /tmp (needed by libguestfs)
		ironicTlsVolume,       // secret for TLS CA cert
	}

	actualVolumeNames := make([]string, len(volumes))
	for i, vol := range volumes {
		actualVolumeNames[i] = vol.Name
	}

	for _, expectedName := range expectedVolumeNames {
		assert.Contains(t, actualVolumeNames, expectedName,
			"Volume %s should be present in getImageVolumes()", expectedName)
	}

	// Verify baremetalSharedVolume is an emptyDir (required for writable /shared)
	var sharedVolume *corev1.Volume
	for i := range volumes {
		if volumes[i].Name == baremetalSharedVolume {
			sharedVolume = &volumes[i]
			break
		}
	}
	assert.NotNil(t, sharedVolume, "baremetalSharedVolume should exist")
	assert.NotNil(t, sharedVolume.EmptyDir,
		"baremetalSharedVolume should be an EmptyDir volume for writable /shared")
}

func TestCreateContainerImageCache(t *testing.T) {
	images := &Images{
		Ironic: "test-ironic-image:latest",
	}

	container := createContainerImageCache(images)

	assert.Equal(t, "metal3-httpd", container.Name)
	assert.Equal(t, images.Ironic, container.Image)

	// Verify that sharedVolumeMount is present
	// This is critical because runhttpd needs access to /shared
	assert.Contains(t, container.VolumeMounts, sharedVolumeMount,
		"sharedVolumeMount should be present for /shared access")
	assert.Contains(t, container.VolumeMounts, imageVolumeMount,
		"imageVolumeMount should be present for /shared/html/images")

	// Verify ReadOnlyRootFilesystem is enabled
	assert.NotNil(t, container.SecurityContext)
	assert.NotNil(t, container.SecurityContext.ReadOnlyRootFilesystem)
	assert.True(t, *container.SecurityContext.ReadOnlyRootFilesystem,
		"ReadOnlyRootFilesystem should be true")
}

func TestTransformURL(t *testing.T) {
	testCases := []struct {
		name          string
		namespace     string
		inputURL      string
		expectedURL   string
		expectedError bool
	}{
		{
			name:        "RHCOS image URL",
			namespace:   "openshift-machine-api",
			inputURL:    "https://releases-art-rhcos.svc.ci.openshift.org/art/storage/releases/rhcos-4.2/42.80.20190725.1/rhcos-42.80.20190725.1-openstack.qcow2.gz",
			expectedURL: "https://metal3-state.openshift-machine-api.svc.cluster.local:6388/images/rhcos-42.80.20190725.1-openstack.qcow2/rhcos-42.80.20190725.1-openstack.qcow2",
		},
		{
			name:        "RHCOS image URL with different namespace",
			namespace:   "test-namespace",
			inputURL:    "https://example.com/rhcos-9.8.20260403-0-openstack.x86_64.qcow2",
			expectedURL: "https://metal3-state.test-namespace.svc.cluster.local:6388/images/rhcos-9.8.20260403-0-openstack.x86_64.qcow2/rhcos-9.8.20260403-0-openstack.x86_64.qcow2",
		},
		{
			name:          "Invalid URL",
			namespace:     "openshift-machine-api",
			inputURL:      "://invalid-url",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := transformURL(tc.namespace, tc.inputURL)

			if tc.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedURL, result)
		})
	}
}

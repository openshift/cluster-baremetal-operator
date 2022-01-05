package provisioning

import (
	"testing"
)

var (
	TestImagesFile                       = "sample_images.json"
	expectedBaremetalOperator            = "registry.ci.openshift.org/openshift:baremetal-operator"
	expectedIronic                       = "registry.ci.openshift.org/openshift:ironic"
	expectedMachineOsDownloader          = "registry.ci.openshift.org/openshift:ironic-machine-os-downloader"
	expectedIronicStaticIpManager        = "registry.ci.openshift.org/openshift:ironic-static-ip-manager"
	expectedIronicAgent                  = "registry.ci.openshift.org/openshift:ironic-agent"
	expectedImageCustomizationController = "registry.ci.openshift.org/openshift:image-customization-controller"
	expectedMachineOSImages              = "registry.ci.openshift.org/openshift:machine-os-images"
)

func TestGetContainerImages(t *testing.T) {
	testCases := []struct {
		name           string
		imagesFile     string
		expectedError  bool
		expectedImages bool
	}{
		{
			name:           "Valid Images File",
			imagesFile:     TestImagesFile,
			expectedError:  false,
			expectedImages: true,
		},
		{
			name:           "Invalid Images File",
			imagesFile:     "invalid.json",
			expectedError:  true,
			expectedImages: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var containerImages Images

			err := GetContainerImages(&containerImages, tc.imagesFile)
			if tc.expectedError != (err != nil) {
				t.Errorf("ExpectedError: %v, got: %v", tc.expectedError, err)
			}
			if tc.expectedImages {
				if containerImages.BaremetalOperator != expectedBaremetalOperator ||
					containerImages.Ironic != expectedIronic ||
					containerImages.MachineOsDownloader != expectedMachineOsDownloader ||
					containerImages.StaticIpManager != expectedIronicStaticIpManager ||
					containerImages.IronicAgent != expectedIronicAgent ||
					containerImages.ImageCustomizationController != expectedImageCustomizationController ||
					containerImages.MachineOSImages != expectedMachineOSImages {
					t.Errorf("failed GetContainerImages. One or more Baremetal container images do not match the expected images.")
				}
			}
		})
	}
}

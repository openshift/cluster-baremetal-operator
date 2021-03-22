package provisioning

import (
	"testing"
)

var (
	TestImagesFile                = "sample_images.json"
	expectedBaremetalOperator     = "registry.svc.ci.openshift.org/openshift:baremetal-operator"
	expectedIronic                = "registry.svc.ci.openshift.org/openshift:ironic"
	expectedIronicInspector       = "registry.svc.ci.openshift.org/openshift:ironic-inspector"
	expectedIronicIpaDownloader   = "registry.svc.ci.openshift.org/openshift:ironic-ipa-downloader"
	expectedMachineOsDownloader   = "registry.svc.ci.openshift.org/openshift:ironic-machine-os-downloader"
	expectedIronicStaticIpManager = "registry.svc.ci.openshift.org/openshift:ironic-static-ip-manager"
	expectedKubeRBACProxy         = "registry.svc.ci.openshift.org/openshift:kube-rbac-proxy"
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
					containerImages.IronicInspector != expectedIronicInspector ||
					containerImages.IpaDownloader != expectedIronicIpaDownloader ||
					containerImages.MachineOsDownloader != expectedMachineOsDownloader ||
					containerImages.StaticIpManager != expectedIronicStaticIpManager {
					t.Errorf("failed GetContainerImages. One or more Baremetal container images do not match the expected images.")
				}
			}
		})
	}
}

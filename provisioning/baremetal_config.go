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
	"fmt"
	"net"
	"strings"

	"k8s.io/utils/pointer"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

var (
	baremetalHttpPort              = "6180"
	baremetalWebhookPort           = "9447"
	baremetalIronicPort            = "6385"
	baremetalIronicInspectorPort   = "5050"
	baremetalKernelUrlSubPath      = "images/ironic-python-agent.kernel"
	baremetalRamdiskUrlSubPath     = "images/ironic-python-agent.initramfs"
	baremetalIronicEndpointSubpath = "v1/"
	provisioningIP                 = "PROVISIONING_IP"
	provisioningInterface          = "PROVISIONING_INTERFACE"
	provisioningMacAddresses       = "PROVISIONING_MACS"
	deployKernelUrl                = "DEPLOY_KERNEL_URL"
	deployRamdiskUrl               = "DEPLOY_RAMDISK_URL"
	ironicEndpoint                 = "IRONIC_ENDPOINT"
	ironicInspectorEndpoint        = "IRONIC_INSPECTOR_ENDPOINT"
	httpPort                       = "HTTP_PORT"
	dhcpRange                      = "DHCP_RANGE"
	machineImageUrl                = "RHCOS_IMAGE_URL"
	ipOptions                      = "IP_OPTIONS"
	bootIsoSource                  = "IRONIC_BOOT_ISO_SOURCE"
)

func getDHCPRange(config *metal3iov1alpha1.ProvisioningSpec) *string {
	var dhcpRange string
	if config.ProvisioningDHCPRange != "" {
		_, net, err := net.ParseCIDR(config.ProvisioningNetworkCIDR)
		if err == nil {
			cidr, _ := net.Mask.Size()
			dhcpRange = fmt.Sprintf("%s,%d", config.ProvisioningDHCPRange, cidr)
		}
	}
	return &dhcpRange
}

func getProvisioningIPCIDR(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningNetworkCIDR != "" && config.ProvisioningIP != "" {
		_, net, err := net.ParseCIDR(config.ProvisioningNetworkCIDR)
		if err == nil {
			cidr, _ := net.Mask.Size()
			ipCIDR := fmt.Sprintf("%s/%d", config.ProvisioningIP, cidr)
			return &ipCIDR
		}
	}
	return nil
}

func getDeployKernelUrl() *string {
	deployKernelUrl := fmt.Sprintf("http://localhost:%d/%s", imageCachePort, baremetalKernelUrlSubPath)
	return &deployKernelUrl
}

func getDeployRamdiskUrl() *string {
	deployRamdiskUrl := fmt.Sprintf("http://localhost:%d/%s", imageCachePort, baremetalRamdiskUrlSubPath)
	return &deployRamdiskUrl
}

func getIronicEndpoint() *string {
	ironicEndpoint := fmt.Sprintf("https://localhost:%s/%s", baremetalIronicPort, baremetalIronicEndpointSubpath)
	return &ironicEndpoint
}

func getIronicInspectorEndpoint() *string {
	ironicInspectorEndpoint := fmt.Sprintf("https://localhost:%s/%s", baremetalIronicInspectorPort, baremetalIronicEndpointSubpath)
	return &ironicInspectorEndpoint
}

func getProvisioningOSDownloadURL(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningOSDownloadURL != "" {
		return &(config.ProvisioningOSDownloadURL)
	}
	return nil
}

// Check whether the PreProvisionOSDownloadURLs are set. If yes, we
// construct a comma-separated list of RHCOS live images and return it
func getPreProvisioningOSDownloadURLs(config *metal3iov1alpha1.ProvisioningSpec) []string {
	var liveURLs []string
	if config.PreProvisioningOSDownloadURLs.IsoURL != "" {
		liveURLs = append(liveURLs, config.PreProvisioningOSDownloadURLs.IsoURL)
	}
	if isCoreOSIPAAvailable(config) {
		liveURLs = append(liveURLs, config.PreProvisioningOSDownloadURLs.InitramfsURL)
		liveURLs = append(liveURLs, config.PreProvisioningOSDownloadURLs.KernelURL)
		liveURLs = append(liveURLs, config.PreProvisioningOSDownloadURLs.RootfsURL)
	}

	return liveURLs
}

// isCoreOSIPAAvailable is a helper to check whether the CoreOS based IPA URLs are available.
// Only return true when kernel, rootfs and initramfs URLs are present
func isCoreOSIPAAvailable(config *metal3iov1alpha1.ProvisioningSpec) bool {
	if config.PreProvisioningOSDownloadURLs.KernelURL != "" &&
		config.PreProvisioningOSDownloadURLs.RootfsURL != "" &&
		config.PreProvisioningOSDownloadURLs.InitramfsURL != "" {
		return true
	}
	return false
}

func getBootIsoSource(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.BootIsoSource != "" {
		return (*string)(&config.BootIsoSource)
	}
	return nil
}

func getMetal3DeploymentConfig(name string, baremetalConfig *metal3iov1alpha1.ProvisioningSpec) *string {
	switch name {
	case provisioningIP:
		return getProvisioningIPCIDR(baremetalConfig)
	case provisioningInterface:
		return &baremetalConfig.ProvisioningInterface
	case provisioningMacAddresses:
		return pointer.StringPtr(strings.Join(baremetalConfig.ProvisioningMacAddresses, ","))
	case deployKernelUrl:
		return getDeployKernelUrl()
	case deployRamdiskUrl:
		return getDeployRamdiskUrl()
	case ironicEndpoint:
		return getIronicEndpoint()
	case ironicInspectorEndpoint:
		return getIronicInspectorEndpoint()
	case httpPort:
		return pointer.StringPtr(baremetalHttpPort)
	case dhcpRange:
		return getDHCPRange(baremetalConfig)
	case machineImageUrl:
		return getProvisioningOSDownloadURL(baremetalConfig)
	case bootIsoSource:
		return getBootIsoSource(baremetalConfig)
	}
	return nil
}

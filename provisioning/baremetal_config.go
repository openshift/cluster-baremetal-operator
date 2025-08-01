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

const (
	baremetalHttpPort              = "6180"
	baremetalVmediaHttpsPort       = "6183"
	baremetalWebhookPort           = "9447"
	baremetalIronicPort            = 6385
	baremetalKernelSubPath         = "ironic-python-agent.kernel"
	baremetalIronicEndpointSubpath = "v1/"
	provisioningIP                 = "PROVISIONING_IP"
	provisioningInterface          = "PROVISIONING_INTERFACE"
	provisioningMacAddresses       = "PROVISIONING_MACS"
	deployKernelUrl                = "DEPLOY_KERNEL_URL"
	ironicEndpoint                 = "IRONIC_ENDPOINT"
	httpPort                       = "HTTP_PORT"
	vmediaHttpsPort                = "VMEDIA_TLS_PORT"
	dnsIP                          = "DNS_IP"
	dhcpRange                      = "DHCP_RANGE"
	machineImageUrl                = "RHCOS_IMAGE_URL"
	ipOptions                      = "IP_OPTIONS"
	bootIsoSource                  = "IRONIC_BOOT_ISO_SOURCE"
	useUnixSocket                  = "unix"
	useProvisioningDNS             = "provisioning"
)

func getDHCPRange(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningNetwork != metal3iov1alpha1.ProvisioningNetworkManaged {
		return nil
	}
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

func getProvisioningIPWithPrefix(config *metal3iov1alpha1.ProvisioningSpec) *string {
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

func getProvisioningIP(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningNetwork == metal3iov1alpha1.ProvisioningNetworkManaged {
		return getProvisioningIPWithPrefix(config)
	}

	if config.ProvisioningIP != "" {
		return &config.ProvisioningIP
	}
	return nil
}

func getDeployKernelUrl() *string {
	deployKernelUrl := fmt.Sprintf("file://%s/%s", imageSharedDir, baremetalKernelSubPath)
	return &deployKernelUrl
}

func getControlPlanePort(info *ProvisioningInfo) (ironicPort int) {
	ironicPort = baremetalIronicPort
	if UseIronicProxy(info) {
		// Direct access to real services behind the proxy.
		ironicPort = ironicPrivatePort
	}
	return
}

func getControlPlaneEndpoint(info *ProvisioningInfo) (ironicEndpoint string) {
	ironicPort := getControlPlanePort(info)
	ironicEndpoint = fmt.Sprintf("https://%s.%s.svc.cluster.local:%d/%s", stateService, info.Namespace, ironicPort, baremetalIronicEndpointSubpath)
	return
}

func getProvisioningOSDownloadURL(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningOSDownloadURL != "" {
		return &(config.ProvisioningOSDownloadURL)
	}
	return nil
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
		return getProvisioningIP(baremetalConfig)
	case provisioningInterface:
		return &baremetalConfig.ProvisioningInterface
	case provisioningMacAddresses:
		return pointer.StringPtr(strings.Join(baremetalConfig.ProvisioningMacAddresses, ","))
	case deployKernelUrl:
		return getDeployKernelUrl()
	case httpPort:
		return pointer.StringPtr(baremetalHttpPort)
	case vmediaHttpsPort:
		return pointer.StringPtr(baremetalVmediaHttpsPort)
	case dhcpRange:
		return getDHCPRange(baremetalConfig)
	case machineImageUrl:
		return getProvisioningOSDownloadURL(baremetalConfig)
	case bootIsoSource:
		return getBootIsoSource(baremetalConfig)
	}
	return nil
}

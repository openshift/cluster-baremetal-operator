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
	"bytes"
	"fmt"
	"net"
	"net/url"
	"strings"

	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

var (
	log                            = ctrl.Log.WithName("provisioning")
	baremetalHttpPort              = "6180"
	baremetalIronicPort            = "6385"
	baremetalIronicInspectorPort   = "5050"
	baremetalKernelUrlSubPath      = "images/ironic-python-agent.kernel"
	baremetalRamdiskUrlSubPath     = "images/ironic-python-agent.initramfs"
	baremetalIronicEndpointSubpath = "v1/"
	provisioningIP                 = "PROVISIONING_IP"
	provisioningInterface          = "PROVISIONING_INTERFACE"
	deployKernelUrl                = "DEPLOY_KERNEL_URL"
	deployRamdiskUrl               = "DEPLOY_RAMDISK_URL"
	ironicEndpoint                 = "IRONIC_ENDPOINT"
	ironicInspectorEndpoint        = "IRONIC_INSPECTOR_ENDPOINT"
	httpPort                       = "HTTP_PORT"
	dhcpRange                      = "DHCP_RANGE"
	machineImageUrl                = "RHCOS_IMAGE_URL"
)

// ValidateBaremetalProvisioningConfig validates the contents of the provisioning resource
func ValidateBaremetalProvisioningConfig(prov *metal3iov1alpha1.Provisioning) []error {
	provisioningNetworkMode := getProvisioningNetworkMode(prov)
	log.V(1).Info("provisioning network", "mode", provisioningNetworkMode)

	/*
	   Managed:
	   "ProvisioningInterface"
	   "ProvisioningIP"
	   "ProvisioningNetworkCIDR"
	   "ProvisioningDHCPRange"
	   "ProvisioningOSDownloadURL"

	   Unmanaged:
	   "ProvisioningInterface"
	   "ProvisioningIP"
	   "ProvisioningNetworkCIDR"
	   "ProvisioningOSDownloadURL"

	   Disabled:
	   "ProvisioningIP"
	   "ProvisioningNetworkCIDR"
	   "ProvisioningOSDownloadURL"
	*/

	var errs []error

	// They all use provisioningOSDownloadURL
	if err := validateProvisioningOSDownloadURL(prov.Spec.ProvisioningOSDownloadURL); err != nil {
		errs = append(errs, err...)
	}

	// Ignore DHCPRange in all but Managed mode.
	dhcpRange := prov.Spec.ProvisioningDHCPRange
	if provisioningNetworkMode != metal3iov1alpha1.ProvisioningNetworkManaged {
		dhcpRange = ""
	}

	// They all use the provisioning network settings, except DHCPRange.  We'll
	// verify that below
	if err := validateProvisioningNetworkSettings(prov.Spec.ProvisioningIP, prov.Spec.ProvisioningNetworkCIDR, dhcpRange); err != nil {
		errs = append(errs, err...)
	}

	if provisioningNetworkMode == metal3iov1alpha1.ProvisioningNetworkManaged || provisioningNetworkMode == metal3iov1alpha1.ProvisioningNetworkUnmanaged {
		if err := validateProvisioningInterface(prov.Spec.ProvisioningInterface); err != nil {
			errs = append(errs, err...)
		}
	}

	if provisioningNetworkMode == metal3iov1alpha1.ProvisioningNetworkManaged {
		if prov.Spec.ProvisioningDHCPRange == "" {
			errs = append(errs, fmt.Errorf("provisioningDHCPRange is required in Managed mode but is not set"))
		}
	}

	return errs
}

func getProvisioningNetworkMode(prov *metal3iov1alpha1.Provisioning) metal3iov1alpha1.ProvisioningNetwork {
	provisioningNetworkMode := prov.Spec.ProvisioningNetwork
	if provisioningNetworkMode == "" {
		// Set it to the default Managed mode
		provisioningNetworkMode = metal3iov1alpha1.ProvisioningNetworkManaged
		if prov.Spec.ProvisioningDHCPExternal {
			log.V(1).Info("provisioningDHCPExternal is deprecated and will be removed in the next release. Use provisioningNetwork instead.")
			provisioningNetworkMode = metal3iov1alpha1.ProvisioningNetworkUnmanaged
		} else {
			log.V(1).Info("provisioningNetwork and provisioningDHCPExternal not set, defaulting to managed network")
		}
	}
	return provisioningNetworkMode
}

func validateProvisioningOSDownloadURL(uri string) []error {
	var errs []error

	if uri == "" {
		errs = append(errs, fmt.Errorf("provisioningOSDownloadURL is required but is empty"))
		return errs
	}

	parsedURL, err := url.ParseRequestURI(uri)
	if err != nil {
		errs = append(errs, fmt.Errorf("the provisioningOSDownloadURL provided: %q is invalid", uri))
		// If it's not a valid URI lets just return.
		return errs
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		errs = append(errs, fmt.Errorf("unsupported scheme %q in provisioningOSDownloadURL %s", parsedURL.Scheme, uri))
		// Again it's not worth it if it's not http(s)
		return errs
	}
	var sha256Checksum string
	if sha256Checksums, ok := parsedURL.Query()["sha256"]; ok {
		sha256Checksum = sha256Checksums[0]
	}
	if sha256Checksum == "" {
		errs = append(errs, fmt.Errorf("the sha256 parameter in the provisioningOSDownloadURL %q is missing", uri))
	}
	if len(sha256Checksum) != 64 {
		errs = append(errs, fmt.Errorf("the sha256 parameter in the provisioningOSDownloadURL %q is invalid", uri))
	}
	if !strings.HasSuffix(parsedURL.Path, ".qcow2.gz") && !strings.HasSuffix(parsedURL.Path, ".qcow2.xz") {
		errs = append(errs, fmt.Errorf("the provisioningOSDownloadURL provided: %q is an OS image and must end in .qcow2.gz or .qcow2.xz", uri))
	}

	return errs
}

func validateProvisioningInterface(iface string) []error {
	// Unfortunately there seems to be nothing you can really do to verify
	// the ProvisioningInterface as it's not necessarily on this machine
	// and could be almost anything.
	var errs []error

	if iface == "" {
		errs = append(errs, fmt.Errorf("provisioningInterface is required but is not set"))
	}
	return errs
}

func validateProvisioningNetworkSettings(ip string, cidr string, dhcpRange string) []error {
	// provisioningIP and networkCIDR are always set.  DHCP range is optional
	// depending on mode.
	var errs []error

	// Verify provisioning ip and get it into net format for future tests.
	provisioningIP := net.ParseIP(ip)
	if provisioningIP == nil {
		errs = append(errs, fmt.Errorf("could not parse provisioningIP %q", ip))
		return errs
	}

	// Verify Network CIDR
	_, provisioningCIDR, err := net.ParseCIDR(cidr)
	if err != nil {
		errs = append(errs, fmt.Errorf("could not parse provisioningNetworkCIDR %q", cidr))
		// Similar thing.. need this to be valid for further tests.
		return errs
	}

	// Ensure provisioning IP is in the network CIDR
	if !provisioningCIDR.Contains(provisioningIP) {
		errs = append(errs, fmt.Errorf("provisioningIP %q is not in the range defined by the provisioningNetworkCIDR %q", ip, cidr))
	}

	// DHCP Range might not be set in which case we're done here.
	if dhcpRange == "" {
		return errs
	}

	// We want to allow a space after the ',' if the user likes it.
	dhcpRange = strings.ReplaceAll(dhcpRange, ", ", ",")

	// Test DHCP Range.
	dhcpRangeSplit := strings.Split(dhcpRange, ",")
	if len(dhcpRangeSplit) != 2 {
		errs = append(errs, fmt.Errorf("%q is not a valid provisioningDHCPRange.  DHCP range format: start_ip,end_ip", dhcpRange))
		return errs
	}

	for _, ip := range dhcpRangeSplit {
		// Ensure IP is valid
		dhcpIP := net.ParseIP(ip)
		if dhcpIP == nil {
			errs = append(errs, fmt.Errorf("could not parse provisioningDHCPRange, %q is not a valid IP", ip))
			// Can't really do further tests without valid IPs
			return errs
		}

		// Validate IP is in the provisioning network
		if !provisioningCIDR.Contains(dhcpIP) {
			errs = append(errs, fmt.Errorf("invalid provisioningDHCPRange, IP %q is not part of the provisioningNetworkCIDR %q", dhcpIP, cidr))
		}
	}

	// Ensure provisioning IP is not in the DHCP range
	start := net.ParseIP(dhcpRangeSplit[0])
	end := net.ParseIP(dhcpRangeSplit[1])

	if start != nil && end != nil {
		if bytes.Compare(provisioningIP, start) >= 0 && bytes.Compare(provisioningIP, end) <= 0 {
			errs = append(errs, fmt.Errorf("invalid provisioningIP %q, value must be outside of the provisioningDHCPRange %q", provisioningIP, dhcpRange))
		}
	}

	return errs
}

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
	ironicEndpoint := fmt.Sprintf("http://localhost:%s/%s", baremetalIronicPort, baremetalIronicEndpointSubpath)
	return &ironicEndpoint
}

func getIronicInspectorEndpoint() *string {
	ironicInspectorEndpoint := fmt.Sprintf("http://localhost:%s/%s", baremetalIronicInspectorPort, baremetalIronicEndpointSubpath)
	return &ironicInspectorEndpoint
}

func getProvisioningOSDownloadURL(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningOSDownloadURL != "" {
		return &(config.ProvisioningOSDownloadURL)
	}
	return nil
}

func getMetal3DeploymentConfig(name string, baremetalConfig *metal3iov1alpha1.ProvisioningSpec) *string {
	switch name {
	case provisioningIP:
		return getProvisioningIPCIDR(baremetalConfig)
	case provisioningInterface:
		return &baremetalConfig.ProvisioningInterface
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
	}
	return nil
}

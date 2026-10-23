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

package v1alpha1

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"k8s.io/apimachinery/pkg/util/errors"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
)

type EnabledFeatures struct {
	ProvisioningNetwork map[ProvisioningNetwork]bool
}

var (
	log = ctrl.Log.WithName("provisioning_validation")

	// Linux interface names are at most IFNAMSIZ-1 (15) characters and should
	// look like a device name: letters, digits, dots, underscores, and hyphens.
	provisioningInterfaceRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,14}$`)
)

// ValidateBaremetalProvisioningConfig validates the contents of the provisioning resource
func (prov *Provisioning) ValidateBaremetalProvisioningConfig(enabledFeatures EnabledFeatures) error {
	provisioningNetworkMode := prov.getProvisioningNetworkMode()
	log.V(1).Info("provisioning network", "mode", provisioningNetworkMode)

	/*
	   Managed:
	   "ProvisioningIP"
	   "ProvisioningNetworkCIDR"
	   "ProvisioningDHCPRange"
	   "ProvisioningOSDownloadURL"

	   Unmanaged:
	   "ProvisioningIP"
	   "ProvisioningNetworkCIDR"
	   "ProvisioningOSDownloadURL"

	   Disabled:
	   "ProvisioningOSDownloadURL"

	   And optionally when Disabled, both:
	   "ProvisioningIP"
	   "ProvisioningNetworkCIDR"
	*/

	var errs []error

	if !enabledFeatures.ProvisioningNetwork[provisioningNetworkMode] {
		return errors.NewAggregate(append(errs, fmt.Errorf("ProvisioningNetwork %s is not supported", provisioningNetworkMode)))
	}

	// Sanity-check fields that are passed through to containers as environment
	// variables (OCPBUGS-115068). These checks always apply, including when
	// network settings are omitted in Disabled mode.
	if err := validateProvisioningInterface(prov.Spec.ProvisioningInterface); err != nil {
		errs = append(errs, err)
	}
	if err := validateAdditionalNTPServers(prov.Spec.AdditionalNTPServers); err != nil {
		errs = append(errs, err...)
	}
	if err := validateProvisioningMacAddresses(prov.Spec.ProvisioningMacAddresses); err != nil {
		errs = append(errs, err...)
	}
	if err := validateExternalIPs(prov.Spec.ExternalIPs); err != nil {
		errs = append(errs, err...)
	}
	if err := validatePreProvisioningOSDownloadURLs(prov.Spec.PreProvisioningOSDownloadURLs); err != nil {
		errs = append(errs, err...)
	}
	if prov.Spec.UnsupportedConfigOverrides != nil {
		if err := validateNoUnsafeCharacters("unsupportedConfigOverrides.ironicAgentImage", prov.Spec.UnsupportedConfigOverrides.IronicAgentImage); err != nil {
			errs = append(errs, err)
		}
	}

	// They all use provisioningOSDownloadURL
	if err := validateProvisioningOSDownloadURL(prov.Spec.ProvisioningOSDownloadURL); err != nil {
		errs = append(errs, err...)
	}

	if provisioningNetworkMode == ProvisioningNetworkDisabled {
		// Only check network settings in Disabled mode if it's set.
		if prov.Spec.ProvisioningNetworkCIDR == "" && prov.Spec.ProvisioningIP == "" {
			return errors.NewAggregate(errs)
		}
	}

	// Only force check of dhcpRange and gatewayIP if in managed mode.
	dhcpRange := prov.Spec.ProvisioningDHCPRange
	gatewayIP := prov.Spec.ProvisioningNetworkGateway

	// Validate that gateway IP is only set for Managed provisioning network
	if provisioningNetworkMode != ProvisioningNetworkManaged {
		if prov.Spec.ProvisioningNetworkGateway != "" {
			errs = append(errs, fmt.Errorf("provisioningNetworkGateway is only supported for Managed provisioning network, but provisioning network is set to %s", provisioningNetworkMode))
		}
		dhcpRange = ""
		gatewayIP = ""
	}

	if err := validateProvisioningNetworkSettings(prov.Spec.ProvisioningIP, prov.Spec.ProvisioningNetworkCIDR, dhcpRange, gatewayIP, prov.getProvisioningNetworkMode()); err != nil {
		errs = append(errs, err...)
	}

	// We need to check this here because we've designed validateProvisioningNetworkSettings() to allow an empty DHCP Range.
	if provisioningNetworkMode == ProvisioningNetworkManaged {
		if prov.Spec.ProvisioningDHCPRange == "" {
			errs = append(errs, fmt.Errorf("provisioningDHCPRange is required in Managed mode but is not set"))
		}
	}

	return errors.NewAggregate(errs)
}

// isNetworkOrBroadcastAddress checks if the IP is a network or broadcast address
// For IPv4 with masks narrower than /31, network and broadcast addresses are not usable
func isNetworkOrBroadcastAddress(ip net.IP, cidr *net.IPNet) bool {
	ones, bits := cidr.Mask.Size()
	// Only apply network/broadcast checks to IPv4.
	if bits != 32 {
		return false
	}
	// For IPv4 /31 and /32, all addresses are usable.
	if ones >= 31 {
		return false
	}
	// Regular IPv4 → check network/broadcast
	networkAddr := cidr.IP.Mask(cidr.Mask)
	if ip.Equal(networkAddr) {
		return true
	}
	// IPv4 broadcast address check (all host bits are 1).
	broadcast := make(net.IP, len(networkAddr))
	copy(broadcast, networkAddr)
	for i := range broadcast {
		broadcast[i] |= ^cidr.Mask[i]
	}
	return ip.Equal(broadcast)
}

func (prov *Provisioning) getProvisioningNetworkMode() ProvisioningNetwork {
	provisioningNetworkMode := prov.Spec.ProvisioningNetwork
	if provisioningNetworkMode == "" {
		// Set it to the default Managed mode
		provisioningNetworkMode = ProvisioningNetworkManaged
		if prov.Spec.ProvisioningDHCPExternal {
			log.V(1).Info("provisioningDHCPExternal is deprecated and will be removed in the next release. Use provisioningNetwork instead.")
			provisioningNetworkMode = ProvisioningNetworkUnmanaged
		} else {
			log.V(1).Info("provisioningNetwork and provisioningDHCPExternal not set, defaulting to managed network")
		}
	}
	return provisioningNetworkMode
}

// validateNoUnsafeCharacters rejects spaces, newlines, and other control
// characters that should not appear in values copied into container env vars.
func validateNoUnsafeCharacters(field, value string) error {
	if value == "" {
		return nil
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f || unicode.IsSpace(r) {
			return fmt.Errorf("%s %q contains invalid whitespace or control characters", field, value)
		}
	}
	return nil
}

func validateProvisioningInterface(name string) error {
	if name == "" {
		return nil
	}
	if err := validateNoUnsafeCharacters("provisioningInterface", name); err != nil {
		return err
	}
	if !provisioningInterfaceRegexp.MatchString(name) {
		return fmt.Errorf("provisioningInterface %q is not a valid interface name (letters, numbers, dots, underscores, and hyphens; max 15 characters)", name)
	}
	return nil
}

func validateAdditionalNTPServers(servers []string) []error {
	var errs []error
	for i, server := range servers {
		field := fmt.Sprintf("additionalNTPServers[%d]", i)
		if server == "" {
			errs = append(errs, fmt.Errorf("%s must not be empty", field))
			continue
		}
		if err := validateNoUnsafeCharacters(field, server); err != nil {
			errs = append(errs, err)
			continue
		}
		if net.ParseIP(server) != nil {
			continue
		}
		if msgs := k8svalidation.IsDNS1123Subdomain(strings.ToLower(server)); len(msgs) > 0 {
			errs = append(errs, fmt.Errorf("%s %q is not a valid hostname or IP address", field, server))
		}
	}
	return errs
}

func validateProvisioningMacAddresses(macs []string) []error {
	var errs []error
	for i, mac := range macs {
		field := fmt.Sprintf("provisioningMacAddresses[%d]", i)
		if mac == "" {
			errs = append(errs, fmt.Errorf("%s must not be empty", field))
			continue
		}
		if err := validateNoUnsafeCharacters(field, mac); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := net.ParseMAC(mac); err != nil {
			errs = append(errs, fmt.Errorf("%s %q is not a valid MAC address", field, mac))
		}
	}
	return errs
}

func validateExternalIPs(ips []string) []error {
	var errs []error
	for i, ip := range ips {
		field := fmt.Sprintf("externalIPs[%d]", i)
		if ip == "" {
			errs = append(errs, fmt.Errorf("%s must not be empty", field))
			continue
		}
		if err := validateNoUnsafeCharacters(field, ip); err != nil {
			errs = append(errs, err)
			continue
		}
		if net.ParseIP(ip) == nil {
			errs = append(errs, fmt.Errorf("%s %q is not a valid IP address", field, ip))
		}
	}
	return errs
}

func validateHTTPURLHost(field string, parsedURL *url.URL) error {
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q in %s", parsedURL.Scheme, field)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("%s must include a host", field)
	}
	return nil
}

func validatePreProvisioningOSDownloadURLs(urls PreProvisioningOSDownloadURLs) []error {
	var errs []error
	fields := []struct {
		name string
		uri  string
	}{
		{"preProvisioningOSDownloadURLs.isoURL", urls.IsoURL},
		{"preProvisioningOSDownloadURLs.kernelURL", urls.KernelURL},
		{"preProvisioningOSDownloadURLs.initramfsURL", urls.InitramfsURL},
		{"preProvisioningOSDownloadURLs.rootfsURL", urls.RootfsURL},
	}
	for _, f := range fields {
		if f.uri == "" {
			continue
		}
		if err := validateNoUnsafeCharacters(f.name, f.uri); err != nil {
			errs = append(errs, err)
			continue
		}
		parsedURL, err := url.ParseRequestURI(f.uri)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %q is not a valid URL", f.name, f.uri))
			continue
		}
		if err := validateHTTPURLHost(f.name, parsedURL); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateProvisioningOSDownloadURL(uri string) []error {
	var errs []error

	if uri == "" {
		return errs
	}

	if err := validateNoUnsafeCharacters("provisioningOSDownloadURL", uri); err != nil {
		errs = append(errs, err)
		return errs
	}

	parsedURL, err := url.ParseRequestURI(uri)
	if err != nil {
		errs = append(errs, fmt.Errorf("the provisioningOSDownloadURL provided: %q is invalid", uri))
		// If it's not a valid URI lets just return.
		return errs
	}
	if err := validateHTTPURLHost("provisioningOSDownloadURL", parsedURL); err != nil {
		errs = append(errs, err)
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

func validateProvisioningNetworkSettings(ip string, cidr string, dhcpRange string, gatewayIP string, provisioningNetworkMode ProvisioningNetwork) []error {
	// provisioningIP and networkCIDR are always set.  DHCP range is optional
	// depending on mode.
	var errs []error

	// Verify provisioning ip and get it into net format for future tests.
	provisioningIP := net.ParseIP(ip)
	if provisioningIP == nil {
		errs = append(errs, fmt.Errorf("could not parse provisioningIP %q", ip))
		return errs
	}

	if provisioningNetworkMode == ProvisioningNetworkDisabled {
		return errs
	}
	// Verify Network CIDR
	_, provisioningCIDR, err := net.ParseCIDR(cidr)
	if err != nil {
		errs = append(errs, fmt.Errorf("could not parse provisioningNetworkCIDR %q", cidr))
		return errs
	}

	// We cannot have managed ipv6 provisioning networks larger than a /64 due
	// to a limitation in dnsmasq
	cidrSize, _ := provisioningCIDR.Mask.Size()
	if cidrSize < 64 && provisioningCIDR.IP.To4() == nil && provisioningCIDR.IP.To16() != nil && provisioningNetworkMode == ProvisioningNetworkManaged {
		errs = append(errs, fmt.Errorf("provisioningNetworkCIDR mask must be greater than or equal to 64 for managed IPv6 networks"))
	}

	// Ensure provisioning IP is in the network CIDR
	if !provisioningCIDR.Contains(provisioningIP) {
		errs = append(errs, fmt.Errorf("provisioningIP %q is not in the range defined by the provisioningNetworkCIDR %q", ip, cidr))
	}

	// Validate gateway IP if provided
	if gatewayIP != "" {
		gateway := net.ParseIP(gatewayIP)
		if gateway == nil {
			errs = append(errs, fmt.Errorf("could not parse provisioningNetworkGateway %q", gatewayIP))
			return errs
		}
		// Ensure gateway IP is in the network CIDR
		if !provisioningCIDR.Contains(gateway) {
			errs = append(errs, fmt.Errorf("provisioningNetworkGateway %q is not in the range defined by the provisioningNetworkCIDR %q", gatewayIP, cidr))
		}
		// Ensure gateway IP is not the same as provisioning IP
		if gateway.Equal(provisioningIP) {
			errs = append(errs, fmt.Errorf("provisioningNetworkGateway %q cannot be the same as provisioningIP %q", gatewayIP, ip))
		}
		// Ensure gateway is a usable host address (not network or broadcast)
		if isNetworkOrBroadcastAddress(gateway, provisioningCIDR) {
			errs = append(errs, fmt.Errorf("provisioningNetworkGateway %q is not a usable host address (network or broadcast address)", gatewayIP))
		}
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

		// Ensure gateway IP is not in the DHCP range if provided
		if gatewayIP != "" {
			gateway := net.ParseIP(gatewayIP)
			if gateway != nil && bytes.Compare(gateway, start) >= 0 && bytes.Compare(gateway, end) <= 0 {
				errs = append(errs, fmt.Errorf("invalid provisioningNetworkGateway %q, value must be outside of the provisioningDHCPRange %q", gatewayIP, dhcpRange))
			}
		}
	}

	return errs
}

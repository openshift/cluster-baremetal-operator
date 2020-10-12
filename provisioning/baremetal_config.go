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
func ValidateBaremetalProvisioningConfig(prov *metal3iov1alpha1.Provisioning) error {
	provisioningNetworkMode := getProvisioningNetworkMode(prov)
	log.V(1).Info("provisioning network", "mode", provisioningNetworkMode)
	switch provisioningNetworkMode {
	case metal3iov1alpha1.ProvisioningNetworkManaged:
		return validateManagedConfig(prov)
	case metal3iov1alpha1.ProvisioningNetworkUnmanaged:
		return validateUnmanagedConfig(prov)
	case metal3iov1alpha1.ProvisioningNetworkDisabled:
		return validateDisabledConfig(prov)
	}
	return nil
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

func validateManagedConfig(prov *metal3iov1alpha1.Provisioning) error {
	for _, toTest := range []struct {
		Name  string
		Value string
	}{

		{Name: "ProvisioningInterface", Value: prov.Spec.ProvisioningInterface},
		{Name: "ProvisioningIP", Value: prov.Spec.ProvisioningIP},
		{Name: "ProvisioningNetworkCIDR", Value: prov.Spec.ProvisioningNetworkCIDR},
		{Name: "ProvisioningDHCPRange", Value: prov.Spec.ProvisioningDHCPRange},
		{Name: "ProvisioningOSDownloadURL", Value: prov.Spec.ProvisioningOSDownloadURL},
	} {
		if toTest.Value == "" {
			return fmt.Errorf("%s is required but is empty", toTest.Name)
		}
	}
	return nil
}

func validateUnmanagedConfig(prov *metal3iov1alpha1.Provisioning) error {
	for _, toTest := range []struct {
		Name  string
		Value string
	}{

		{Name: "ProvisioningInterface", Value: prov.Spec.ProvisioningInterface},
		{Name: "ProvisioningIP", Value: prov.Spec.ProvisioningIP},
		{Name: "ProvisioningNetworkCIDR", Value: prov.Spec.ProvisioningNetworkCIDR},
		{Name: "ProvisioningOSDownloadURL", Value: prov.Spec.ProvisioningOSDownloadURL},
	} {
		if toTest.Value == "" {
			return fmt.Errorf("%s is required but is empty", toTest.Name)
		}
	}
	return nil
}

func validateDisabledConfig(prov *metal3iov1alpha1.Provisioning) error {
	for _, toTest := range []struct {
		Name  string
		Value string
	}{

		{Name: "ProvisioningIP", Value: prov.Spec.ProvisioningIP},
		{Name: "ProvisioningNetworkCIDR", Value: prov.Spec.ProvisioningNetworkCIDR},
		{Name: "ProvisioningOSDownloadURL", Value: prov.Spec.ProvisioningOSDownloadURL},
	} {
		if toTest.Value == "" {
			return fmt.Errorf("%s is required but is empty", toTest.Name)
		}
	}
	return nil
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

func getDeployKernelUrl(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningIP != "" {
		deployKernelUrl := fmt.Sprintf("http://%s/%s", net.JoinHostPort(config.ProvisioningIP, baremetalHttpPort), baremetalKernelUrlSubPath)
		return &deployKernelUrl
	}
	return nil
}

func getDeployRamdiskUrl(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningIP != "" {
		deployRamdiskUrl := fmt.Sprintf("http://%s/%s", net.JoinHostPort(config.ProvisioningIP, baremetalHttpPort), baremetalRamdiskUrlSubPath)
		return &deployRamdiskUrl
	}
	return nil
}

func getIronicEndpoint(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningIP != "" {
		ironicEndpoint := fmt.Sprintf("http://%s/%s", net.JoinHostPort(config.ProvisioningIP, baremetalIronicPort), baremetalIronicEndpointSubpath)
		return &ironicEndpoint
	}
	return nil
}

func getIronicInspectorEndpoint(config *metal3iov1alpha1.ProvisioningSpec) *string {
	if config.ProvisioningIP != "" {
		inspectorEndpoint := fmt.Sprintf("http://%s/%s", net.JoinHostPort(config.ProvisioningIP, baremetalIronicInspectorPort), baremetalIronicEndpointSubpath)
		return &inspectorEndpoint
	}
	return nil
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
		return getDeployKernelUrl(baremetalConfig)
	case deployRamdiskUrl:
		return getDeployRamdiskUrl(baremetalConfig)
	case ironicEndpoint:
		return getIronicEndpoint(baremetalConfig)
	case ironicInspectorEndpoint:
		return getIronicInspectorEndpoint(baremetalConfig)
	case httpPort:
		return pointer.StringPtr(baremetalHttpPort)
	case dhcpRange:
		return &baremetalConfig.ProvisioningDHCPRange
	case machineImageUrl:
		return getProvisioningOSDownloadURL(baremetalConfig)
	}
	return nil
}

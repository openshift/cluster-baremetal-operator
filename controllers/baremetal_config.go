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

package controllers

import (
	"fmt"

	"github.com/golang/glog"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	baremetalHttpPort              = "6180"
	baremetalIronicPort            = "6385"
	baremetalIronicInspectorPort   = "5050"
	baremetalKernelUrlSubPath      = "images/ironic-python-agent.kernel"
	baremetalRamdiskUrlSubPath     = "images/ironic-python-agent.initramfs"
	baremetalIronicEndpointSubpath = "v1/"
	provisioningNetworkManaged     = "Managed"
	provisioningNetworkUnmanaged   = "Unmanaged"
	provisioningNetworkDisabled    = "Disabled"
)

func validateBaremetalProvisioningConfig(config *metal3iov1alpha1.Provisioning) error {
	if config.Spec.ProvisioningNetwork == "" {
		config.Spec.ProvisioningNetwork = provisioningNetworkManaged
		if config.Spec.ProvisioningDHCPExternal {
			config.Spec.ProvisioningNetwork = provisioningNetworkUnmanaged
		}
		glog.V(1).Infof("ProvisioningNetwork config not provided. Based on ProvisioningDHCPExternal setting it to: %s", config.Spec.ProvisioningNetwork)
	}
	switch config.Spec.ProvisioningNetwork {
	case provisioningNetworkManaged:
		glog.V(1).Info("Provisioning Network Managed")
		return validateManagedConfig(config)
	case provisioningNetworkUnmanaged:
		glog.V(1).Info("Provisioning Network Unmanaged")
		return validateUnmanagedConfig(config)
	case provisioningNetworkDisabled:
		glog.V(1).Info("Provisioning Network Disabled")
		return validateDisabledConfig(config)
	}
	return nil
}

func validateManagedConfig(config *metal3iov1alpha1.Provisioning) error {
	// All values must be provided in the provisioning CR
	if config.Spec.ProvisioningInterface == "" ||
		config.Spec.ProvisioningIP == "" ||
		config.Spec.ProvisioningNetworkCIDR == "" ||
		config.Spec.ProvisioningDHCPRange == "" ||
		config.Spec.ProvisioningOSDownloadURL == "" {
		return fmt.Errorf("Configuration missing in Managed ProvisioningNetwork mode in config resource %s", baremetalProvisioningCR)
	}
	return nil
}

func validateUnmanagedConfig(config *metal3iov1alpha1.Provisioning) error {
	// All values except ProvisioningDHCPRange must be provided in the provisioning CR
	if config.Spec.ProvisioningInterface == "" ||
		config.Spec.ProvisioningIP == "" ||
		config.Spec.ProvisioningNetworkCIDR == "" ||
		config.Spec.ProvisioningOSDownloadURL == "" {
		return fmt.Errorf("Configuration missing in Unmanaged ProvisioningNetwork mode in config resource %s", baremetalProvisioningCR)
	}
	return nil
}

func validateDisabledConfig(config *metal3iov1alpha1.Provisioning) error {
	// All values except ProvisioningDHCPRange must be provided in the provisioning CR
	if config.Spec.ProvisioningIP == "" ||
		config.Spec.ProvisioningNetworkCIDR == "" ||
		config.Spec.ProvisioningOSDownloadURL == "" {
		return fmt.Errorf("Configuration missing in Disabled ProvisioningNetwork mode in config resource %s", baremetalProvisioningCR)
	}
	return nil
}

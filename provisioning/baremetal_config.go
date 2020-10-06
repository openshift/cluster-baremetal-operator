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

	ctrl "sigs.k8s.io/controller-runtime"

	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

var (
	log = ctrl.Log.WithName("provisioning")
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

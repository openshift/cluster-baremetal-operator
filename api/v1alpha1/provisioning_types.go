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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// ProvisioningNetwork is the boot mode of the system
// +kubebuilder:validation:Enum=Managed;Unmanaged;Disabled
type ProvisioningNetwork string

// ProvisioningNetwork modes
const (
	ProvisioningNetworkManaged   ProvisioningNetwork = "Managed"
	ProvisioningNetworkUnmanaged ProvisioningNetwork = "Unmanaged"
	ProvisioningNetworkDisabled  ProvisioningNetwork = "Disabled"

	// ProvisioningFinalizer is required for proper handling of deletion
	ProvisioningFinalizer = "provisioning.metal3.io"
)

// ProvisioningSpec defines the desired state of Provisioning
type ProvisioningSpec struct {
	// ProvisioningInterface is the name of the network interface
	// on a baremetal server to the provisioning network. It can
	// have values like eth1 or ens3.
	ProvisioningInterface string `json:"provisioningInterface,omitempty"`

	// ProvisioningIP is the IP address assigned to the
	// provisioningInterface of the baremetal server. This IP
	// address should be within the provisioning subnet, and
	// outside of the DHCP range.
	ProvisioningIP string `json:"provisioningIP,omitempty"`

	// ProvisioningNetworkCIDR is the network on which the
	// baremetal nodes are provisioned. The provisioningIP and the
	// IPs in the dhcpRange all come from within this network.
	ProvisioningNetworkCIDR string `json:"provisioningNetworkCIDR,omitempty"`

	// ProvisioningDHCPExternal indicates whether the DHCP server
	// for IP addresses in the provisioning DHCP range is present
	// within the metal3 cluster or external to it. This field is being
	// deprecated in favor of provisioningNetwork.
	ProvisioningDHCPExternal bool `json:"provisioningDHCPExternal,omitempty"`

	// ProvisioningDHCPRange needs to be interpreted along with
	// ProvisioningDHCPExternal. If the value of
	// provisioningDHCPExternal is set to False, then
	// ProvisioningDHCPRange represents the range of IP addresses
	// that the DHCP server running within the metal3 cluster can
	// use while provisioning baremetal servers. If the value of
	// ProvisioningDHCPExternal is set to True, then the value of
	// ProvisioningDHCPRange will be ignored. When the value of
	// ProvisioningDHCPExternal is set to False, indicating an
	// internal DHCP server and the value of ProvisioningDHCPRange
	// is not set, then the DHCP range is taken to be the default
	// range which goes from .10 to .100 of the
	// ProvisioningNetworkCIDR. This is the only value in all of
	// the Provisioning configuration that can be changed after
	// the installer has created the CR. This value needs to be
	// two comma sererated IP addresses within the
	// ProvisioningNetworkCIDR where the 1st address represents
	// the start of the range and the 2nd address represents the
	// last usable address in the  range.
	ProvisioningDHCPRange string `json:"provisioningDHCPRange,omitempty"`

	// ProvisioningOSDownloadURL is the location from which the OS
	// Image used to boot baremetal host machines can be downloaded
	// by the metal3 cluster.
	ProvisioningOSDownloadURL string `json:"provisioningOSDownloadURL,omitempty"`

	// ProvisioningNetwork provides a way to indicate the state of the
	// underlying network configuration for the provisioning network.
	// This field can have one of the following values -
	// `Managed`- when the provisioning network is completely managed by
	// the Baremetal IPI solution.
	// `Unmanaged`- when the provsioning network is present and used but
	// the user is responsible for managing DHCP. Virtual media provisioning
	// is recommended but PXE is still available if required.
	// `Disabled`- when the provisioning network is fully disabled. User can
	// bring up the baremetal cluster using virtual media or assisted
	// installation. If using metal3 for power management, BMCs must be
	// accessible from the machine networks. User should provide two IPs on
	// the external network that would be used for provisioning services.
	ProvisioningNetwork ProvisioningNetwork `json:"provisioningNetwork,omitempty"`

	// WatchAllNamespaces provides a way to explicitly allow use of this
	// Provisioning configuration across all Namespaces. It is an
	// optional configuration which defaults to false and in that state
	// will be used to provision baremetal hosts in only the
	// openshift-machine-api namespace. When set to true, this provisioning
	// configuration would be used for baremetal hosts across all namespaces.
	WatchAllNamespaces bool `json:"watchAllNamespaces,omitempty"`
}

// ProvisioningStatus defines the observed state of Provisioning
type ProvisioningStatus struct {
	operatorv1.OperatorStatus `json:",inline"`
}

// +kubebuilder:resource:path=provisionings,scope=Cluster
// +kubebuilder:subresource:status

// Provisioning contains configuration used by the Provisioning
// service (Ironic) to provision baremetal hosts.
// Provisioning is created by the OpenShift installer using admin or
// user provided information about the provisioning network and the
// NIC on the server that can be used to PXE boot it.
// This CR is a singleton, created by the installer and currently only
// consumed by the cluster-baremetal-operator to bring up and update
// containers in a metal3 cluster.
// +kubebuilder:object:root=true
type Provisioning struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisioningSpec   `json:"spec,omitempty"`
	Status ProvisioningStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProvisioningList contains a list of Provisioning
type ProvisioningList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provisioning `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provisioning{}, &ProvisioningList{})
}

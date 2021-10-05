# cluster-baremetal-operator

## Introduction

OpenShift supports multiple cloud platform types like AWS, GCP, Azure etc. In addition to these public cloud platforms, OpenShift can also be used to
set up a fully functional OpenShift cluster with just Baremetal servers which are either pre-provisioned or are fully provisioned using OpenShift’s
Baremetal IPI solution.
The CBO fits into this specific solution by deploying on an OpenShift control plane, all the components necessary to take an unprovisioned server to
a fully functional worker node ready to run OpenShift compute workloads. Details about the origin of this second level operator and design
alternatives considered during its inception can be found in this [enhancement](https://github.com/openshift/enhancements/blob/master/enhancements/baremetal/an-slo-for-baremetal.md).

## What does it do?

Cluster-baremetal-operator (CBO) is designed to be an OpenShift Operator that is responsible for deploying all components required to provision
Baremetal servers into worker nodes that join an OpenShift cluster.
The components that have knowledge of how to connect and boot a Baremetal Server are encapsulated in an upstream K8s project called [metal3.io](http://metal3.io/). The CBO is responsible for making sure that the metal3 deployment consisting of the baremetal-operator (BMO) and Ironic containers is
always running on one of the master nodes within the OpenShift cluster.
The CBO is also responsible for listening to OpenShift updates to resources that it is watching and take appropriate action. The CBO reports on its
own state via the “baremetal” ClusterOperator resource as is required by every OpenShift operator.
The CBO reads, validates and passes information provided in the [Provisioning Config Resource (CR)](https://github.com/openshift/cluster-baremetal-operator/blob/master/config/crd/bases/metal3.io_provisionings.yaml) and passes this information to the metal3 deployment. It also creates Secrets that
containers within the metal3 deployment use to communicate with each other. Currently, only one copy of the Provisioning CR exists per OpenShift
Cluster so all worker nodes would be provisioned using the same configuration.


## When is CBO active?

CBO runs on all platform types supported by OpenShift but will perform its above mentioned tasks only when the platform type is BareMetal. The
“baremetal” ClusterOperator (CO) displays the current state of the CBO when running on a BareMetal platform and as “Disabled” in all Platform types.
When the CBO is running on the BareMetal platform, it manages the metal3 deployment and will continue communicating its state using the “baremetal”
ClusterOperator.
CBO is considered a second level operator (SLO) in OpenShift parlance. What that means is that another OpenShift operator is responsible for
deploying CBO. In this case, the Cluster Version Operator (CVO) is responsible for deploying its SLOs at a specific run level that is coded into the
manifests of that operator.
The OpenShift Installer is responsible for deploying the control plane but does not wait for CVO to complete deployment of the CBO. The CVO, also
running on the control plane, is completely responsible for running the CBO on one master node at a time. The worker deployment is completely handled
by metal3 which in turn is deployed by CBO as mentioned earlier.

## What are its inputs?

In order to successfully boot a baremetal server, the CBO needs information about the network to which the baremetal servers are connected and where
it can find the image required to boot the server.
This information is provided by the Provisioning CR. CBO watches this resource and passes this information to the metal3 deployment after validating
its contents.

The "Provisioning" Custom Resource Definition (CRD) is the configuration input for cluster-baremetal-operator (CBO).  It contains information used by
the Ironic provisioning service to provision new baremetal hosts.  This is done using either PXE or through a BMC.

There is only one instance of the provisioning resource and in turn, only a single provisioning network is supported.

The configuration of the provisioning resource determines the options used when creating the metal3 pod containing Ironic and supporting containers.

The configurable portions of the Provisioning CRD are:

- ProvisioningInterface is the name of the network interface
on a baremetal server to the provisioning network. It can
have values like eth1 or ens3.

- ProvisioningMacAddresses is a list of mac addresses of network interfaces
on a baremetal server to the provisioning network.
Use this instead of ProvisioningInterface to allow interfaces of different
names. If not provided it will be populated by the BMH.Spec.BootMacAddress
of each master.

- ProvisioningIP is the IP address assigned to the
provisioningInterface of the baremetal server. This IP
address should be within the provisioning subnet, and
outside of the DHCP range.

- ProvisioningNetworkCIDR is the network on which the
baremetal nodes are provisioned. The provisioningIP and the
IPs in the dhcpRange all come from within this network. When using IPv6
and in a network managed by the Baremetal IPI solution this cannot be a
network larger than a /64.

- ProvisioningDHCPExternal indicates whether the DHCP server
for IP addresses in the provisioning DHCP range is present
within the metal3 cluster or external to it. This field is being
deprecated in favor of provisioningNetwork.

- ProvisioningDHCPRange needs to be interpreted along with
ProvisioningDHCPExternal. If the value of
provisioningDHCPExternal is set to False, then
ProvisioningDHCPRange represents the range of IP addresses
that the DHCP server running within the metal3 cluster can
use while provisioning baremetal servers. If the value of
ProvisioningDHCPExternal is set to True, then the value of
ProvisioningDHCPRange will be ignored. When the value of
ProvisioningDHCPExternal is set to False, indicating an
internal DHCP server and the value of ProvisioningDHCPRange
is not set, then the DHCP range is taken to be the default
range which goes from .10 to .100 of the
ProvisioningNetworkCIDR. This is the only value in all of
the Provisioning configuration that can be changed after
the installer has created the CR. This value needs to be
two comma sererated IP addresses within the
ProvisioningNetworkCIDR where the 1st address represents
the start of the range and the 2nd address represents the
last usable address in the  range.

- ProvisioningOSDownloadURL is the location from which the OS
Image used to boot baremetal host machines can be downloaded
by the metal3 cluster.

- ProvisioningNetwork provides a way to indicate the state of the
underlying network configuration for the provisioning network.
This field can have one of the following values -
`Managed`- when the provisioning network is completely managed by
the Baremetal IPI solution.
`Unmanaged`- when the provsioning network is present and used but
the user is responsible for managing DHCP. Virtual media provisioning
is recommended but PXE is still available if required.
`Disabled`- when the provisioning network is fully disabled. User can
bring up the baremetal cluster using virtual media or assisted
installation. If using metal3 for power management, BMCs must be
accessible from the machine networks. User should provide two IPs on
the external network that would be used for provisioning services.

- WatchAllNamespaces provides a way to explicitly allow use of this
Provisioning configuration across all Namespaces. It is an
optional configuration which defaults to false and in that state
will be used to provision baremetal hosts in only the
openshift-machine-api namespace. When set to true, this provisioning
configuration would be used for baremetal hosts across all namespaces.

- BootIsoSource provides a way to set the location where the iso image
to boot the nodes will be served from.
By default the boot iso image is cached locally and served from
the Provisioning service (Ironic) nodes using an auxiliary httpd server.
If the boot iso image is already served by an httpd server, setting
this option to http allows to directly provide the image from there;
in this case, the network (either internal or external) where the
httpd server that hosts the boot iso is needs to be accessible
by the metal3 pod.

- VirtualMediaViaExternalNetwork flag when set to "true" allows for workers
to boot via Virtual Media and contact metal3 over the External Network.
When the flag is set to "false" (which is the default), virtual media
deployments can still happen based on the configuration specified in the
ProvisioningNetwork i.e when in Disabled mode, over the External Network
and over Provisioning Network when in Managed mode.
PXE deployments will always use the Provisioning Network and will not be
affected by this flag.

- PreprovisioningOSDownloadURLs is set of CoreOS Live URLs that would be necessary to provision a worker
either using virtual media or PXE.

- DisableVirtualMediaTLS turns off TLS on the virtual media server,
which may be required for hardware that cannot accept HTTPS links.


## What are its outputs?

If not already created by an external entity, the metal3 deployment and its associated Secrets are created by the CBO. The CBO also creates an
image-cache Daemonset that assists the metal3 deployment by downloading the image provided in the Provisioning CR and making it available locally on
each master node so that the metal3 deployment is able to access a local copy of the image while trying to boot a baremetal server.

CBO reports its own state using the “baremetal” CO as mentioned earlier. It is also designed to provide alerts and metrics regarding its own
deployment. It is also capable of reporting metrics gathered by BMO regarding the baremetal servers being provisioned. These metrics can then be
scraped by Prometheus and can be viewed on the Prometheus dashboard.

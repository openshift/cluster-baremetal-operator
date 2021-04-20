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

- ProvisioningIP: This is the IP address assigned to the provisioningInterface of master node that is running the metal3 pod.
This IP address should be within the provisioning subnet, and outside of the DHCP range.

- ProvisioningInterface: This is the name of the network interface on the baremetal masters which will be used to provision new nodes.
Note that all masters must have the same network interface name.

- ProvisioningNetwork: This specifies the way in which you want to have your provisioning network configured.

    - Managed: DHCP is "managed" by metal3.  This will cause CBO to deploy the metal3 pod with a DHCP server configured.
	All settings for network configuration below will be required for this to work.

    - Unmanaged: DHCP is not managed by metal3.  The administrator of the installation is responsible for configuring DHCP appropriately.

    - Disabled: Provisioning is disabled.  This is generally the result of not using the installer for installation.
	Note you can still use metal3 to do power management.

- ProvisioningNetworkCIDR: This defines the address space used for the provisioning network.

- ProvisioningOSDownloadURL: This is the location from which the OS Image used to boot baremetal host machines can be downloaded by the metal3 pod.

- WatchAllNamespaces: This provides a way to explicitly allow CBO to instruct the Baremetal Operator (BMO)  watch for BareMetal Host (BMH) objects in all namespaces
and not just the openshift-machine-api namespace. The default is 'false' and this must be set to 'true' manually after installation.

## What are its outputs?

If not already created by an external entity, the metal3 deployment and its associated Secrets are created by the CBO. The CBO also creates an
image-cache Daemonset that assists the metal3 deployment by downloading the image provided in the Provisioning CR and making it available locally on
each master node so that the metal3 deployment is able to access a local copy of the image while trying to boot a baremetal server.

CBO reports its own state using the “baremetal” CO as mentioned earlier. It is also designed to provide alerts and metrics regarding its own
deployment. It is also capable of reporting metrics gathered by BMO regarding the baremetal servers being provisioned. These metrics can then be
scraped by Prometheus and can be viewed on the Prometheus dashboard.

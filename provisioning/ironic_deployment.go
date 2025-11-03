package provisioning

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ironicv1alpha1 "github.com/metal3-io/ironic-standalone-operator/api/v1alpha1"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

const (
	ironicDeploymentName     = "metal3-ironic"
	ironicAPICredentialsName = ironicSecretName // Reuse existing secret name
	ironicTrustedCAName      = externalTrustBundleConfigMapName
)

// parseDHCPRange parses a DHCP range string in the format "IP1,IP2" and returns the begin and end IPs.
func parseDHCPRange(dhcpRange string) (begin, end string, err error) {
	if dhcpRange == "" {
		return "", "", nil
	}

	parts := strings.Split(dhcpRange, ",")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid DHCP range format: %q, expected format: \"IP1,IP2\"", dhcpRange)
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// buildIronicSpec constructs an Ironic CR spec from the Provisioning CR.
func buildIronicSpec(info *ProvisioningInfo) (ironicv1alpha1.IronicSpec, error) {
	spec := ironicv1alpha1.IronicSpec{
		APICredentialsName: ironicAPICredentialsName,
		TLS: ironicv1alpha1.TLS{
			CertificateName:        tlsSecretName,
			DisableVirtualMediaTLS: info.ProvConfig.Spec.DisableVirtualMediaTLS,
		},
		DeployRamdisk: ironicv1alpha1.DeployRamdisk{
			SSHKey: info.SSHKey,
		},
	}

	// Configure networking
	networking := ironicv1alpha1.Networking{}

	if UseIronicProxy(info) {
		// When the proxy is using the normal Ironic public port we need to force Ironic to use a private port.
		networking.APIPort = ironicPrivatePort
	}

	// Set IP address based on provisioning network configuration
	if info.ProvConfig.Spec.ProvisioningIP != "" {
		networking.IPAddress = info.ProvConfig.Spec.ProvisioningIP
	}

	// Set network interface
	if info.ProvConfig.Spec.ProvisioningInterface != "" {
		networking.Interface = info.ProvConfig.Spec.ProvisioningInterface
	}

	// Set MAC addresses if provided
	if len(info.ProvConfig.Spec.ProvisioningMacAddresses) > 0 {
		networking.MACAddresses = info.ProvConfig.Spec.ProvisioningMacAddresses
	}

	// Configure DHCP when provisioning network is managed
	if info.ProvConfig.Spec.ProvisioningNetwork == metal3iov1alpha1.ProvisioningNetworkManaged {
		if info.ProvConfig.Spec.ProvisioningDHCPRange != "" && info.ProvConfig.Spec.ProvisioningNetworkCIDR != "" {
			rangeBegin, rangeEnd, err := parseDHCPRange(info.ProvConfig.Spec.ProvisioningDHCPRange)
			if err != nil {
				return spec, errors.Wrap(err, "failed to parse DHCP range")
			}

			dhcp := &ironicv1alpha1.DHCP{
				NetworkCIDR: info.ProvConfig.Spec.ProvisioningNetworkCIDR,
				RangeBegin:  rangeBegin,
				RangeEnd:    rangeEnd,
			}
			networking.DHCP = dhcp
		}
	}

	spec.Networking = networking

	// Add OpenShift-specific static IP management containers via Overrides
	// These are only needed when a provisioning IP is configured and provisioning network is not disabled
	var initContainers []corev1.Container
	var extraContainers []corev1.Container
	if info.ProvConfig.Spec.ProvisioningIP != "" && info.ProvConfig.Spec.ProvisioningNetwork != metal3iov1alpha1.ProvisioningNetworkDisabled {
		initContainers = append(initContainers, createInitContainerStaticIpSet(info.Images, &info.ProvConfig.Spec))
		extraContainers = append(extraContainers, createContainerMetal3StaticIpManager(info.Images, &info.ProvConfig.Spec))
	}

	// Extract the pre-provisioning images from a container in the payload
	initContainers = append(initContainers, createInitContainerMachineOSImages(info, "--all", imageVolumeMount, imageSharedDir))

	// If the ProvisioningOSDownloadURL is set, we download the URL specified in it
	if info.ProvConfig.Spec.ProvisioningOSDownloadURL != "" {
		initContainers = append(initContainers, createInitContainerMachineOsDownloader(info, info.ProvConfig.Spec.ProvisioningOSDownloadURL, false, true))
	}

	initContainers = injectProxyAndCA(initContainers, info.Proxy)
	extraContainers = injectProxyAndCA(extraContainers, info.Proxy)

	spec.Overrides = &ironicv1alpha1.Overrides{
		InitContainers: initContainers,
		Containers:     extraContainers,
	}

	return spec, nil
}

// EnsureIronicDeployment creates or updates the Ironic CR for managing the Ironic deployment.
func EnsureIronicDeployment(info *ProvisioningInfo) (bool, error) {
	klog.Infof("Ensuring Ironic deployment %s/%s", info.Namespace, ironicDeploymentName)

	// Build the Ironic spec from Provisioning config
	ironicSpec, err := buildIronicSpec(info)
	if err != nil {
		return false, errors.Wrap(err, "failed to build Ironic spec")
	}

	// Construct the Ironic CR
	ironicCR := &ironicv1alpha1.Ironic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ironicDeploymentName,
			Namespace: info.Namespace,
		},
	}

	// Use CreateOrPatch to create or update the Ironic CR only when needed
	result, err := controllerutil.CreateOrPatch(context.TODO(), info.CRClient, ironicCR, func() error {
		// Set the spec
		ironicCR.Spec = ironicSpec

		// Set the Provisioning CR as the owner of the Ironic CR
		if err := controllerutil.SetControllerReference(info.ProvConfig, ironicCR, info.Scheme); err != nil {
			return errors.Wrap(err, "failed to set owner reference on Ironic CR")
		}

		return nil
	})

	if err != nil {
		return false, errors.Wrap(err, "failed to create or update Ironic CR")
	}

	switch result {
	case controllerutil.OperationResultCreated:
		klog.Infof("Created Ironic deployment %s/%s", info.Namespace, ironicDeploymentName)
		return true, nil
	case controllerutil.OperationResultUpdated:
		klog.Infof("Updated Ironic deployment %s/%s", info.Namespace, ironicDeploymentName)
		return false, nil
	default:
		klog.V(4).Infof("Ironic deployment %s/%s unchanged", info.Namespace, ironicDeploymentName)
		return false, nil
	}
}

// GetIronicDeploymentState provides the current state of Ironic deployment created by IrSO
func GetIronicDeploymentState(cl client.Client, targetNamespace string) (appsv1.DeploymentConditionType, error) {
	ironicCR := &ironicv1alpha1.Ironic{}
	err := cl.Get(context.Background(), client.ObjectKey{
		Name:      ironicDeploymentName,
		Namespace: targetNamespace,
	}, ironicCR)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Ironic CR not found, deployment hasn't started yet
			return appsv1.DeploymentProgressing, nil
		}
		// There were errors accessing the Ironic CR
		return appsv1.DeploymentReplicaFailure, err
	}

	// Find the Ready condition in the Ironic CR status
	readyCondition := meta.FindStatusCondition(ironicCR.Status.Conditions, string(ironicv1alpha1.IronicStatusReady))
	if readyCondition == nil {
		// Ready condition not set yet, deployment is progressing
		return appsv1.DeploymentProgressing, nil
	}

	// Map Ironic condition to deployment condition type based on status and reason
	switch readyCondition.Status {
	case metav1.ConditionTrue:
		// Ironic is ready
		return appsv1.DeploymentAvailable, nil
	case metav1.ConditionFalse:
		// Check the reason to determine if it's a failure or in progress
		if readyCondition.Reason == ironicv1alpha1.IronicReasonFailed {
			return appsv1.DeploymentReplicaFailure, nil
		}
		// Still progressing
		return appsv1.DeploymentProgressing, nil
	default:
		// Unknown status, treat as progressing
		return appsv1.DeploymentProgressing, nil
	}
}

// DeleteIronicDeployment deletes the Ironic CR.
// In practice, this may not be needed since the Ironic CR has an owner reference
// to the Provisioning CR and will be garbage collected automatically.
func DeleteIronicDeployment(info *ProvisioningInfo) error {
	klog.Infof("Deleting Ironic deployment %s/%s", info.Namespace, ironicDeploymentName)

	ironicCR := &ironicv1alpha1.Ironic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ironicDeploymentName,
			Namespace: info.Namespace,
		},
	}

	err := info.CRClient.Delete(context.TODO(), ironicCR)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete Ironic CR")
	}

	klog.Infof("Ironic deployment %s/%s deleted successfully", info.Namespace, ironicDeploymentName)
	return nil
}

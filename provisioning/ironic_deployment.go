package provisioning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
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
	// When provisioning network is managed, use the provisioning IP
	if info.ProvConfig.Spec.ProvisioningNetwork == metal3iov1alpha1.ProvisioningNetworkManaged {
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
		Spec: ironicSpec,
	}

	// Set the Provisioning CR as the owner of the Ironic CR
	if err := controllerutil.SetControllerReference(info.ProvConfig, ironicCR, info.Scheme); err != nil {
		return false, errors.Wrap(err, "failed to set owner reference on Ironic CR")
	}

	// Try to get existing Ironic CR
	existing := &ironicv1alpha1.Ironic{}
	err = info.CRClient.Get(context.TODO(), client.ObjectKey{
		Name:      ironicDeploymentName,
		Namespace: info.Namespace,
	}, existing)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create the Ironic CR
			klog.Infof("Creating Ironic deployment %s/%s", info.Namespace, ironicDeploymentName)
			if err := info.CRClient.Create(context.TODO(), ironicCR); err != nil {
				return false, errors.Wrap(err, "failed to create Ironic CR")
			}
			return true, nil
		}
		return false, errors.Wrap(err, "failed to get Ironic CR")
	}

	// Update the Ironic CR if it exists
	klog.V(4).Infof("Updating Ironic deployment %s/%s", info.Namespace, ironicDeploymentName)
	existing.Spec = ironicSpec
	if err := info.CRClient.Update(context.TODO(), existing); err != nil {
		return false, errors.Wrap(err, "failed to update Ironic CR")
	}

	klog.Infof("Ironic deployment %s/%s configured successfully", info.Namespace, ironicDeploymentName)
	return false, nil
}

// GetIronicDeploymentState provides the current state of Ironic deployment created by IrSO
func GetIronicDeploymentState(client appsclientv1.DeploymentsGetter, targetNamespace string) (appsv1.DeploymentConditionType, error) {
	existing, err := client.Deployments(targetNamespace).Get(context.Background(), ironicServiceDeploymentName, metav1.GetOptions{})
	if err != nil || existing == nil {
		// There were errors accessing the deployment.
		return appsv1.DeploymentReplicaFailure, err
	}
	deploymentState := getDeploymentCondition(existing)
	if deploymentState == appsv1.DeploymentProgressing && deploymentRolloutTimeout <= time.Since(deploymentRolloutStartTime) {
		return appsv1.DeploymentReplicaFailure, nil
	}
	return deploymentState, nil
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

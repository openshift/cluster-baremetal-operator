package controllers

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
)

// IsEnabled returns true if there is at least one feature enabled.
func IsEnabled(enabledFeatures v1alpha1.EnabledFeatures) bool {
	for _, f := range enabledFeatures.ProvisioningNetwork {
		if f {
			return true
		}
	}
	return false
}

// EnabledFeatures returns the features that are enabled on the current platform.
func EnabledFeatures(ctx context.Context, osClient osclientset.Interface) (v1alpha1.EnabledFeatures, error) {
	features := v1alpha1.EnabledFeatures{
		ProvisioningNetwork: map[v1alpha1.ProvisioningNetwork]bool{
			v1alpha1.ProvisioningNetworkDisabled:  false,
			v1alpha1.ProvisioningNetworkUnmanaged: false,
			v1alpha1.ProvisioningNetworkManaged:   false,
		},
	}

	infra, err := osClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return features, errors.Wrap(err, "unable to determine Platform")
	}

	if infra.Status.ControlPlaneTopology == osconfigv1.ExternalTopologyMode {
		return features, nil
	}

	//NOTE(dtantsur/janders) if the code below is updated, GetServerInternalIP() function in provisioning/utils.go needs to be updated to match.
	switch infra.Status.PlatformStatus.Type {
	case osconfigv1.BareMetalPlatformType:
		features.ProvisioningNetwork[v1alpha1.ProvisioningNetworkDisabled] = true
		features.ProvisioningNetwork[v1alpha1.ProvisioningNetworkUnmanaged] = true
		features.ProvisioningNetwork[v1alpha1.ProvisioningNetworkManaged] = true
	case osconfigv1.OpenStackPlatformType, osconfigv1.NonePlatformType, osconfigv1.VSpherePlatformType, osconfigv1.AWSPlatformType:
		features.ProvisioningNetwork[v1alpha1.ProvisioningNetworkDisabled] = true
	}

	return features, nil
}

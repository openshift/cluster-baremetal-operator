package controllers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	osconfigv1 "github.com/openshift/api/config/v1"
	osclientset "github.com/openshift/client-go/config/clientset/versioned"
)

const featureGateCRName = "cluster"

// FeatureGates provides access to enabled/disabled OpenShift feature gates for the current release.
type FeatureGates struct {
	enabled    map[osconfigv1.FeatureGateName]bool
	disabled   map[osconfigv1.FeatureGateName]bool
	featureSet osconfigv1.FeatureSet
}

// Enabled returns true if the given feature gate is enabled for the current release version.
func (fg FeatureGates) Enabled(name osconfigv1.FeatureGateName) bool {
	return fg.enabled[name]
}

// ActiveFeatureSet returns the active feature set (e.g. Default, TechPreviewNoUpgrade).
func (fg FeatureGates) ActiveFeatureSet() osconfigv1.FeatureSet {
	return fg.featureSet
}

// ReadFeatureGates reads the cluster FeatureGate CR and returns the feature gates
// applicable to the given release version. If releaseVersion is empty or does not
// match any entry in the status, an empty FeatureGates is returned.
func ReadFeatureGates(ctx context.Context, osClient osclientset.Interface, releaseVersion string) (FeatureGates, error) {
	featureGateCR, err := osClient.ConfigV1().FeatureGates().Get(ctx, featureGateCRName, metav1.GetOptions{})
	if err != nil {
		return FeatureGates{}, err
	}
	return featureGatesFromCR(featureGateCR, releaseVersion), nil
}

// featureGatesFromCR builds a FeatureGates from the given FeatureGate CR, selecting
// the status entry that matches releaseVersion.
func featureGatesFromCR(cr *osconfigv1.FeatureGate, releaseVersion string) FeatureGates {
	fg := FeatureGates{
		enabled:    make(map[osconfigv1.FeatureGateName]bool),
		disabled:   make(map[osconfigv1.FeatureGateName]bool),
		featureSet: cr.Spec.FeatureSet,
	}

	if releaseVersion == "" {
		klog.Warning("Release version is not set; feature gates will be unavailable until the operator restarts")
		return fg
	}

	for _, details := range cr.Status.FeatureGates {
		if details.Version != releaseVersion {
			continue
		}
		for _, gate := range details.Enabled {
			fg.enabled[gate.Name] = true
		}
		for _, gate := range details.Disabled {
			fg.disabled[gate.Name] = true
		}
		return fg
	}

	klog.V(4).Infof("No feature gate status found for release version %q; feature gates unavailable until next reconciliation", releaseVersion)
	return fg
}

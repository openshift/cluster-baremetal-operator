package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
)

func makeFeatureGateCR(featureSet osconfigv1.FeatureSet, details []osconfigv1.FeatureGateDetails) *osconfigv1.FeatureGate {
	return &osconfigv1.FeatureGate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FeatureGate",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: featureGateCRName,
		},
		Spec: osconfigv1.FeatureGateSpec{
			FeatureGateSelection: osconfigv1.FeatureGateSelection{
				FeatureSet: featureSet,
			},
		},
		Status: osconfigv1.FeatureGateStatus{
			FeatureGates: details,
		},
	}
}

func TestReadFeatureGates(t *testing.T) {
	const version = "4.18.0"
	const gateA osconfigv1.FeatureGateName = "GateA"
	const gateB osconfigv1.FeatureGateName = "GateB"
	const gateC osconfigv1.FeatureGateName = "GateB"

	details := []osconfigv1.FeatureGateDetails{
		{
			Version: version,
			Enabled: []osconfigv1.FeatureGateAttributes{
				{Name: gateA},
			},
			Disabled: []osconfigv1.FeatureGateAttributes{
				{Name: gateB},
			},
		},
	}

	testCases := []struct {
		name           string
		cr             *osconfigv1.FeatureGate
		releaseVersion string
		expectedSet    osconfigv1.FeatureSet
		expectedError  string
		gateAEnabled   bool
		gateBEnabled   bool
	}{
		{
			name:           "matching version",
			cr:             makeFeatureGateCR(osconfigv1.Default, details),
			releaseVersion: version,
			expectedSet:    osconfigv1.Default,
			gateAEnabled:   true,
			gateBEnabled:   false,
		},
		{
			name:           "different set",
			cr:             makeFeatureGateCR(osconfigv1.TechPreviewNoUpgrade, details),
			releaseVersion: version,
			expectedSet:    osconfigv1.TechPreviewNoUpgrade,
			gateAEnabled:   true,
			gateBEnabled:   false,
		},
		{
			name:           "non-matching version",
			cr:             makeFeatureGateCR(osconfigv1.Default, details),
			releaseVersion: "99.0.0",
			expectedError:  `no feature gate status found for release version "99.0.0"`,
		},
		{
			name:           "empty release version",
			cr:             makeFeatureGateCR(osconfigv1.Default, details),
			releaseVersion: "",
			expectedSet:    osconfigv1.Default,
			gateAEnabled:   false,
			gateBEnabled:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := fakeconfigclientset.NewClientset(tc.cr)
			fg, err := ReadFeatureGates(context.Background(), client, tc.releaseVersion)
			if tc.expectedError == "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedSet, fg.FeatureSet)
				assert.Equal(t, tc.gateAEnabled, fg.Enabled[gateA])
				assert.Equal(t, tc.gateBEnabled, fg.Enabled[gateB])
				// Unknown feature gate is considered disabled
				assert.False(t, fg.Enabled[gateC])
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}

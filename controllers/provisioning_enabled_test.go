package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
)

func TestIsEnabled(t *testing.T) {
	infra := configv1.Infrastructure{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Infrastructure",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Status: configv1.InfrastructureStatus{
			Platform: configv1.NonePlatformType,
		},
	}
	withPlatformType := func(in configv1.Infrastructure, pt configv1.PlatformType) *configv1.Infrastructure {
		out := in.DeepCopy()
		out.Status.Platform = pt
		return out
	}
	withExternalControlPlane := func(in configv1.Infrastructure) *configv1.Infrastructure {
		out := in.DeepCopy()
		out.Status.ControlPlaneTopology = "External"
		return out
	}

	testCases := []struct {
		name          string
		infra         *configv1.Infrastructure
		expectedError bool
		isEnabled     bool
	}{
		{
			name:          "BaremetalPlatform",
			infra:         withPlatformType(infra, configv1.BareMetalPlatformType),
			expectedError: false,
			isEnabled:     true,
		},
		{
			name:          "NonePlatform",
			infra:         &infra,
			expectedError: false,
			isEnabled:     true,
		},
		{
			name:          "aws",
			infra:         withPlatformType(infra, configv1.AWSPlatformType),
			expectedError: false,
			isEnabled:     false,
		},
		{
			name:          "NoPlatform",
			infra:         withPlatformType(infra, ""),
			expectedError: false,
			isEnabled:     false,
		},
		{
			name:          "BadPlatform",
			infra:         &configv1.Infrastructure{},
			expectedError: true,
			isEnabled:     false,
		},
		{
			name:          "external controlplane",
			infra:         withExternalControlPlane(infra),
			expectedError: false,
			isEnabled:     false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)

			ef, err := EnabledFeatures(context.TODO(), fakeconfigclientset.NewSimpleClientset(tc.infra))
			if tc.expectedError && err == nil {
				t.Error("should have produced an error")
				return
			}
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.isEnabled, IsEnabled(ef), "enabled results did not match")
		})
	}
}

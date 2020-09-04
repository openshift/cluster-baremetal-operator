package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1 "github.com/openshift/api/config/v1"
)

//var infrastructureGVK = scheme.GroupVersionResource{Group: "openshift.io", Version: "v1", Kind: "Infrastructure"}

func setUpSchemeForReconciler() *runtime.Scheme {
	scheme := runtime.NewScheme()
	// we need to add the openshift/api to the scheme to be able to read
	// the infrastructure CR
	configv1.Install(scheme)
	return scheme
}

func newFakeProvisioningReconciler(scheme *runtime.Scheme, object runtime.Object) *ProvisioningReconciler {
	return &ProvisioningReconciler{
		Client: fakeclient.NewFakeClientWithScheme(scheme, object),
		Log:    ctrl.Log.WithName("controllers").WithName("Provisioning"),
		Scheme: scheme,
	}
}

func TestReconcile(t *testing.T) {
	request := ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: metav1.NamespaceNone,
			Name:      "Provisioning",
		},
	}
	testCases := []struct {
		name          string
		infra         *configv1.Infrastructure
		expectedError bool
	}{
		{
			name: "BaremetalPlatform",
			infra: &configv1.Infrastructure{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Infrastructure",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: configv1.InfrastructureStatus{
					Platform: configv1.BareMetalPlatformType,
				},
			},
			expectedError: false,
		},
		{
			name:          "BadPlatform",
			infra:         &configv1.Infrastructure{},
			expectedError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)

			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), tc.infra)
			_, err := reconciler.Reconcile(request)
			if tc.expectedError != (err != nil) {
				t.Errorf("ExpectedError: %v, got: %v", tc.expectedError, err)
			}
		})
	}
}

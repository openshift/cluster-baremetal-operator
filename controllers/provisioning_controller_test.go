package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	baremetalv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
)

func setUpSchemeForReconciler() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	// we need to add the openshift/api to the scheme to be able to read
	// the infrastructure CR
	utilruntime.Must(configv1.AddToScheme(scheme))
	utilruntime.Must(metal3iov1alpha1.AddToScheme(scheme))
	utilruntime.Must(baremetalv1alpha1.AddToScheme(scheme))
	return scheme
}

func newFakeProvisioningReconciler(scheme *runtime.Scheme, object runtime.Object) *ProvisioningReconciler {
	return &ProvisioningReconciler{
		Client:   fakeclient.NewFakeClientWithScheme(scheme, object),
		Scheme:   scheme,
		OSClient: fakeconfigclientset.NewSimpleClientset(),
	}
}

func TestProvisioning(t *testing.T) {
	testCases := []struct {
		name           string
		baremetalCR    *metal3iov1alpha1.Provisioning
		expectedError  bool
		expectedConfig bool
	}{
		{
			name: "ValidCR",
			baremetalCR: &metal3iov1alpha1.Provisioning{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Provisioning",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: metal3iov1alpha1.ProvisioningSingletonName,
				},
			},
			expectedError:  false,
			expectedConfig: true,
		},
		{
			name:           "MissingCR",
			baremetalCR:    &metal3iov1alpha1.Provisioning{},
			expectedError:  false,
			expectedConfig: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)

			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), tc.baremetalCR)
			baremetalconfig, err := reconciler.readProvisioningCR(context.TODO())
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedConfig, baremetalconfig != nil, "baremetal config results did not match")
			return
		})
	}
}

func TestNetworkStackFromServiceNetwork(t *testing.T) {
	testCases := []struct {
		name                 string
		networkCR            *configv1.Network
		expectedError        bool
		expectedNetworkStack provisioning.NetworkStackType
	}{
		{
			name: "StatusIPv4",
			networkCR: &configv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: configv1.NetworkStatus{
					ServiceNetwork: []string{"172.30.0.0/16"},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackV4,
		},
		{
			name: "SpecIPv6",
			networkCR: &configv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.NetworkSpec{
					ServiceNetwork: []string{"fd02::/112"},
				},
				Status: configv1.NetworkStatus{
					ServiceNetwork: []string{},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackV6,
		},
		{
			name: "StatusDualStack",
			networkCR: &configv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: configv1.NetworkStatus{
					ServiceNetwork: []string{"172.30.0.0/16", "fd02::/112"},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackDual,
		},
		{
			name: "SpecDualStack",
			networkCR: &configv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.NetworkSpec{
					ServiceNetwork: []string{"172.30.0.0/16", "fd02::/112"},
				},
				Status: configv1.NetworkStatus{
					ServiceNetwork: []string{},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackDual,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing tc : %s", tc.name)

			r := &ProvisioningReconciler{
				Scheme:   setUpSchemeForReconciler(),
				OSClient: fakeconfigclientset.NewSimpleClientset(tc.networkCR),
			}
			ns, err := r.networkStackFromServiceNetwork(context.TODO())
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			assert.Equal(t, tc.expectedNetworkStack, ns, "network stack results did not match")
			return
		})
	}
}

func TestUpdateProvisioningMacAddresses(t *testing.T) {
	sc := setUpSchemeForReconciler()
	objects := []runtime.Object{
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-master-0", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e5",
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane-1", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e6",
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-master-2", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e7",
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-worker-0", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e8",
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-worker-1", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e9",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-0",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "baremetalhost:///openshift-machine-api/test-master-0/something",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-1",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "provider:///openshift-machine-api/test-worker-0/something",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-2",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "baremetalhost:///broken",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-3",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "baremetalhost:///openshift-machine-api/test-controlplane-1/something",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-4",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Spec: corev1.NodeSpec{},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-5",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "baremetalhost:///openshift-machine-api/test-master-2/something",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-6",
				Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "baremetalhost:///openshift-machine-api/test-worker-1/something",
			},
		},
	}
	baremetalCR := metal3iov1alpha1.Provisioning{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Provisioning",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: metal3iov1alpha1.ProvisioningSingletonName,
		},
		Spec: metal3iov1alpha1.ProvisioningSpec{},
	}
	objects = append(objects, &baremetalCR)
	r := &ProvisioningReconciler{
		Scheme: sc,
		Client: fakeclient.NewFakeClientWithScheme(sc, objects...),
	}

	want := []string{"00:3d:25:45:bf:e5", "00:3d:25:45:bf:e6", "00:3d:25:45:bf:e7"}
	err := r.updateProvisioningMacAddresses(context.TODO(), &baremetalCR)
	assert.NoError(t, err, "ProvisioningReconciler.updateProvisioningMacAddresses()")
	assert.ElementsMatch(t, baremetalCR.Spec.ProvisioningMacAddresses, want)
}

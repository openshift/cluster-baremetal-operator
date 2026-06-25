package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	baremetalv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	osconfigv1 "github.com/openshift/api/config/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	metal3iov1alpha1 "github.com/openshift/cluster-baremetal-operator/api/v1alpha1"
	"github.com/openshift/cluster-baremetal-operator/provisioning"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
)

func setUpSchemeForReconciler() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	// we need to add the openshift/api to the scheme to be able to read
	// the infrastructure CR
	utilruntime.Must(osconfigv1.AddToScheme(scheme))
	utilruntime.Must(machinev1beta1.AddToScheme(scheme))
	utilruntime.Must(metal3iov1alpha1.AddToScheme(scheme))
	utilruntime.Must(baremetalv1alpha1.AddToScheme(scheme))
	return scheme
}

func newFakeProvisioningReconciler(scheme *runtime.Scheme, object runtime.Object) *ProvisioningReconciler {
	return &ProvisioningReconciler{
		Client:   fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(object).Build(),
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
		})
	}
}

func TestNetworkStackFromServiceNetwork(t *testing.T) {
	testCases := []struct {
		name                 string
		networkCR            *osconfigv1.Network
		expectedError        bool
		expectedNetworkStack provisioning.NetworkStackType
	}{
		{
			name: "StatusIPv4",
			networkCR: &osconfigv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: osconfigv1.NetworkStatus{
					ServiceNetwork: []string{"172.30.0.0/16"},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackV4,
		},
		{
			name: "SpecIPv6",
			networkCR: &osconfigv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: osconfigv1.NetworkSpec{
					ServiceNetwork: []string{"fd02::/112"},
				},
				Status: osconfigv1.NetworkStatus{
					ServiceNetwork: []string{},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackV6,
		},
		{
			name: "StatusDualStack",
			networkCR: &osconfigv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: osconfigv1.NetworkStatus{
					ServiceNetwork: []string{"172.30.0.0/16", "fd02::/112"},
				},
			},
			expectedError:        false,
			expectedNetworkStack: provisioning.NetworkStackDual,
		},
		{
			name: "SpecDualStack",
			networkCR: &osconfigv1.Network{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Network",
					APIVersion: "config.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: osconfigv1.NetworkSpec{
					ServiceNetwork: []string{"172.30.0.0/16", "fd02::/112"},
				},
				Status: osconfigv1.NetworkStatus{
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
				ConsumerRef: &corev1.ObjectReference{
					APIVersion: "machine.openshift.io/v1beta1",
					Kind:       "Machine",
					Name:       "node-0",
					Namespace:  "openshift-machine-api",
				},
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane-1", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e6",
				// No consumerRef, using the reference from the Machine
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-master-2", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e7",
				ConsumerRef: &corev1.ObjectReference{
					APIVersion: "machine.openshift.io/v1beta1",
					Kind:       "Machine",
					Name:       "node-5",
					Namespace:  "openshift-machine-api",
				},
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-worker-0", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e8",
				ConsumerRef: &corev1.ObjectReference{
					APIVersion: "machine.openshift.io/v1beta1",
					Kind:       "Machine",
					Name:       "node-6",
					Namespace:  "openshift-machine-api",
				},
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "test-worker-1", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:e9",
			},
		},
		&baremetalv1alpha1.BareMetalHost{
			ObjectMeta: metav1.ObjectMeta{Name: "something-else", Namespace: ComponentNamespace},
			Spec: baremetalv1alpha1.BareMetalHostSpec{
				BootMACAddress: "00:3d:25:45:bf:ea",
				ConsumerRef: &corev1.ObjectReference{
					APIVersion: "machine.openshift.io/v1beta1",
					Kind:       "Machine",
					Name:       "not-node",
					Namespace:  "unexpected-namespace",
				},
			},
		},
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node-0",
				Labels:      map[string]string{"machine.openshift.io/cluster-api-machine-role": "master"},
				Annotations: map[string]string{"metal3.io/BareMetalHost": "openshift-machine-api/test-master-0"},
			},
		},
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node-1",
				Labels:      map[string]string{"machine.openshift.io/cluster-api-machine-role": "master"},
				Annotations: map[string]string{"metal3.io/BareMetalHost": "another-ns/host"},
			},
		},
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node-2",
				Labels:      map[string]string{"machine.openshift.io/cluster-api-machine-role": "master"},
				Annotations: map[string]string{"metal3.io/BareMetalHost": "invalid"},
			},
		},
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node-3",
				Labels:      map[string]string{"machine.openshift.io/cluster-api-machine-role": "master"},
				Annotations: map[string]string{"metal3.io/BareMetalHost": "openshift-machine-api/test-controlplane-1"},
			},
		},
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-4",
				Labels: map[string]string{"machine.openshift.io/cluster-api-machine-role": "master"},
			},
		},
		&machinev1beta1.Machine{
			// This machine does not have a direct reference, but there is a back reference from the BMH.
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-5",
				Labels: map[string]string{"machine.openshift.io/cluster-api-machine-role": "master"},
			},
		},
		&machinev1beta1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "node-6",
				Labels:      map[string]string{"machine.openshift.io/cluster-api-machine-role": "worker"},
				Annotations: map[string]string{"metal3.io/BareMetalHost": "openshift-machine-api/test-worker-0"},
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
		Client: fakeclient.NewClientBuilder().WithScheme(sc).WithRuntimeObjects(objects...).Build(),
	}

	want := []string{"00:3d:25:45:bf:e5", "00:3d:25:45:bf:e6", "00:3d:25:45:bf:e7"}
	err := r.updateProvisioningMacAddresses(context.TODO(), &baremetalCR)
	assert.NoError(t, err, "ProvisioningReconciler.updateProvisioningMacAddresses()")
	assert.ElementsMatch(t, baremetalCR.Spec.ProvisioningMacAddresses, want)
}

func TestCheckDaemonSetProgressing(t *testing.T) {
	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	err := reconciler.checkDaemonSet(provisioning.DaemonSetProgressing, nil, "test daemonset", func() error { return nil })
	assert.NoError(t, err, "checkDaemonSet should return nil for DaemonSetProgressing")
}

func TestCheckDaemonSetAvailable(t *testing.T) {
	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	err := reconciler.checkDaemonSet(provisioning.DaemonSetAvailable, nil, "test daemonset", func() error { return nil })
	assert.NoError(t, err, "checkDaemonSet should return nil for DaemonSetAvailable")
}

func getCondition(conditions []osconfigv1.ClusterOperatorStatusCondition, condType osconfigv1.ClusterStatusConditionType) *osconfigv1.ClusterOperatorStatusCondition {
	return v1helpers.FindStatusCondition(conditions, condType)
}

func TestUpdateCOStatusSyncingOnResourceUpdate(t *testing.T) {
	reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
	co, _ := reconciler.createClusterOperator()
	reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

	err := reconciler.updateCOStatus(ReasonSyncing, "", "Applying metal3 resources")
	assert.NoError(t, err)

	gotCO, err := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
	assert.NoError(t, err)

	progressing := getCondition(gotCO.Status.Conditions, osconfigv1.OperatorProgressing)
	assert.NotNil(t, progressing)
	assert.Equal(t, osconfigv1.ConditionTrue, progressing.Status)
	assert.Equal(t, string(ReasonSyncing), progressing.Reason)
	assert.Equal(t, "Applying metal3 resources", progressing.Message)

	available := getCondition(gotCO.Status.Conditions, osconfigv1.OperatorAvailable)
	assert.NotNil(t, available)
	assert.Equal(t, osconfigv1.ConditionTrue, available.Status)
}

func TestUpdateCOStatusDeploymentProgressing(t *testing.T) {
	tCases := []struct {
		name        string
		progressMsg string
	}{
		{
			name:        "Metal3DeploymentRollingOut",
			progressMsg: "metal3 deployment is rolling out",
		},
		{
			name:        "BMODeploymentRollingOut",
			progressMsg: "baremetal-operator deployment is rolling out",
		},
		{
			name:        "ImageCacheDaemonSetRollingOut",
			progressMsg: "metal3 image cache daemonset is rolling out",
		},
		{
			name:        "IronicProxyDaemonSetRollingOut",
			progressMsg: "ironic proxy daemonset is rolling out",
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			reconciler := newFakeProvisioningReconciler(setUpSchemeForReconciler(), &osconfigv1.Infrastructure{})
			co, _ := reconciler.createClusterOperator()
			reconciler.OSClient = fakeconfigclientset.NewSimpleClientset(co)

			err := reconciler.updateCOStatus(ReasonSyncing, "", tc.progressMsg)
			assert.NoError(t, err)

			gotCO, err := reconciler.OSClient.ConfigV1().ClusterOperators().Get(context.Background(), clusterOperatorName, metav1.GetOptions{})
			assert.NoError(t, err)

			progressing := getCondition(gotCO.Status.Conditions, osconfigv1.OperatorProgressing)
			assert.NotNil(t, progressing)
			assert.Equal(t, osconfigv1.ConditionTrue, progressing.Status)
			assert.Equal(t, string(ReasonSyncing), progressing.Reason)
			assert.Equal(t, tc.progressMsg, progressing.Message)

			available := getCondition(gotCO.Status.Conditions, osconfigv1.OperatorAvailable)
			assert.NotNil(t, available)
			assert.Equal(t, osconfigv1.ConditionTrue, available.Status)

			degraded := getCondition(gotCO.Status.Conditions, osconfigv1.OperatorDegraded)
			assert.NotNil(t, degraded)
			assert.Equal(t, osconfigv1.ConditionFalse, degraded.Status)
		})
	}
}

func TestCompleteNotSetDuringRollout(t *testing.T) {
	tCases := []struct {
		name            string
		deploymentState appsv1.DeploymentConditionType
		bmoState        appsv1.DeploymentConditionType
		imageCacheState appsv1.DaemonSetConditionType
		ironicProxyDS   appsv1.DaemonSetConditionType
		expectComplete  bool
	}{
		{
			name:            "AllAvailable",
			deploymentState: appsv1.DeploymentAvailable,
			bmoState:        appsv1.DeploymentAvailable,
			imageCacheState: provisioning.DaemonSetAvailable,
			ironicProxyDS:   provisioning.DaemonSetAvailable,
			expectComplete:  true,
		},
		{
			name:            "Metal3DeploymentProgressing",
			deploymentState: appsv1.DeploymentProgressing,
			bmoState:        appsv1.DeploymentAvailable,
			imageCacheState: provisioning.DaemonSetAvailable,
			ironicProxyDS:   provisioning.DaemonSetAvailable,
			expectComplete:  false,
		},
		{
			name:            "BMODeploymentProgressing",
			deploymentState: appsv1.DeploymentAvailable,
			bmoState:        appsv1.DeploymentProgressing,
			imageCacheState: provisioning.DaemonSetAvailable,
			ironicProxyDS:   provisioning.DaemonSetAvailable,
			expectComplete:  false,
		},
		{
			name:            "ImageCacheDaemonSetProgressing",
			deploymentState: appsv1.DeploymentAvailable,
			bmoState:        appsv1.DeploymentAvailable,
			imageCacheState: provisioning.DaemonSetProgressing,
			ironicProxyDS:   provisioning.DaemonSetAvailable,
			expectComplete:  false,
		},
		{
			name:            "IronicProxyDaemonSetProgressing",
			deploymentState: appsv1.DeploymentAvailable,
			bmoState:        appsv1.DeploymentAvailable,
			imageCacheState: provisioning.DaemonSetAvailable,
			ironicProxyDS:   provisioning.DaemonSetProgressing,
			expectComplete:  false,
		},
		{
			name:            "MultipleProgressing",
			deploymentState: appsv1.DeploymentProgressing,
			bmoState:        appsv1.DeploymentProgressing,
			imageCacheState: provisioning.DaemonSetProgressing,
			ironicProxyDS:   provisioning.DaemonSetProgressing,
			expectComplete:  false,
		},
	}

	for _, tc := range tCases {
		t.Run(tc.name, func(t *testing.T) {
			rolloutInProgress := tc.deploymentState == appsv1.DeploymentProgressing ||
				tc.bmoState == appsv1.DeploymentProgressing ||
				tc.imageCacheState == provisioning.DaemonSetProgressing ||
				tc.ironicProxyDS == provisioning.DaemonSetProgressing

			shouldComplete := !rolloutInProgress &&
				tc.deploymentState == appsv1.DeploymentAvailable &&
				tc.bmoState == appsv1.DeploymentAvailable

			assert.Equal(t, tc.expectComplete, shouldComplete,
				"Complete status should only be set when no rollout is in progress and all deployments are available")
		})
	}
}
